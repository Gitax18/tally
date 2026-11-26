package ingest

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SessionTimeout ends a visit after this long without an event.
const SessionTimeout = 30 * time.Minute

// Event kinds the tracker sends.
const (
	KindPageview   = "pageview"
	KindEngagement = "engagement"
)

// Dims are a visit's attributes. They come from its first pageview and are copied onto every
// event of the visit.
type Dims struct {
	Hostname    string
	Source      string
	Referrer    string
	UTMSource   string
	UTMMedium   string
	UTMCampaign string
	Country     string
	Browser     string
	OS          string
	Device      string
}

// Event is one validated tracker request, ready for the worker.
type Event struct {
	SiteID    string
	Kind      string
	TS        time.Time
	VisitorID int64
	// PrevVisitorID is the same visitor under yesterday's salt (0 if there is none), so a visit
	// that crosses midnight UTC continues instead of starting over.
	PrevVisitorID int64
	Path          string
	Hostname      string
	EngagedMS     int64
	Dims          Dims // used only when the event starts a new visit
}

// Counters are what the ingest path has done since boot.
type Counters struct {
	Accepted    atomic.Int64 // queued for the worker
	Dropped     atomic.Int64 // the queue was full
	Stored      atomic.Int64 // written by the worker
	Failed      atomic.Int64 // the database refused the write
	Orphaned    atomic.Int64 // engagement with no open visit
	Bots        atomic.Int64
	Rejected    atomic.Int64 // invalid, unknown site, or wrong hostname
	RateLimited atomic.Int64
}

type sessionKey struct {
	site    string
	visitor int64
}

type session struct {
	id       int64
	visitor  int64 // the ID stored on the visit's rows
	lastSeen time.Time
	dims     Dims
	pages    map[string]int64 // path -> latest pageview event ID, for engagement pings
}

// maxPagesPerSession bounds the per-visit path map.
const maxPagesPerSession = 64

// Worker owns the queue and is the only writer of sessions and events, so visits never race.
type Worker struct {
	pool     *pgxpool.Pool
	logger   *slog.Logger
	counters *Counters
	touch    func(ctx context.Context, siteID string, at time.Time)

	mu     sync.RWMutex // guards closed and sending on ch
	closed bool
	ch     chan Event

	sessions  map[sessionKey]*session
	started   time.Time
	lastError time.Time
	done      chan struct{}
}

// NewWorker returns a worker with a queue of the given size. touch records a site's last event.
func NewWorker(pool *pgxpool.Pool, logger *slog.Logger, counters *Counters, queueSize int, touch func(context.Context, string, time.Time)) *Worker {
	return &Worker{
		pool:     pool,
		logger:   logger,
		counters: counters,
		touch:    touch,
		ch:       make(chan Event, queueSize),
		sessions: map[sessionKey]*session{},
		done:     make(chan struct{}),
	}
}

// Push queues an event without blocking. It returns false when the queue is full or closed.
func (w *Worker) Push(ev Event) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.closed {
		return false
	}
	select {
	case w.ch <- ev:
		return true
	default:
		return false
	}
}

// Len is the number of queued events.
func (w *Worker) Len() int { return len(w.ch) }

// Close stops accepting events. Run returns once the queue is drained; Wait blocks until then.
func (w *Worker) Close() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.closed {
		w.closed = true
		close(w.ch)
	}
}

// Wait blocks until Run has drained the queue and returned, or ctx ends.
func (w *Worker) Wait(ctx context.Context) error {
	select {
	case <-w.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Run processes events in order until Close is called and the queue is empty. Database writes
// use ctx; pass a context that outlives the HTTP server so the drain can finish.
func (w *Worker) Run(ctx context.Context) {
	defer close(w.done)
	w.started = time.Now()
	sweep := time.NewTicker(time.Minute)
	defer sweep.Stop()
	for {
		select {
		case ev, ok := <-w.ch:
			if !ok {
				return
			}
			w.process(ctx, ev)
		case now := <-sweep.C:
			w.sweep(now)
		}
	}
}

func (w *Worker) process(ctx context.Context, ev Event) {
	var err error
	switch ev.Kind {
	case KindPageview:
		err = w.pageview(ctx, ev)
	case KindEngagement:
		err = w.engagement(ctx, ev)
	}
	if err != nil {
		w.counters.Failed.Add(1)
		// One line per 10 seconds is enough to notice a database outage without flooding logs.
		if time.Since(w.lastError) > 10*time.Second {
			w.lastError = time.Now()
			w.logger.Error("ingest: write failed", "site", ev.SiteID, "kind", ev.Kind, "error", err,
				"failed_total", w.counters.Failed.Load())
		}
		return
	}
	w.counters.Stored.Add(1)
	if w.touch != nil && ev.Kind == KindPageview {
		w.touch(ctx, ev.SiteID, ev.TS)
	}
}

// sweep forgets visits that have timed out.
func (w *Worker) sweep(now time.Time) {
	for k, s := range w.sessions {
		if now.Sub(s.lastSeen) > SessionTimeout {
			delete(w.sessions, k)
		}
	}
}

// resolve finds the open visit for an event: in memory under today's or yesterday's visitor ID,
// or, in the first half hour after a restart, in the database.
func (w *Worker) resolve(ctx context.Context, ev Event) (*session, error) {
	for _, id := range []int64{ev.VisitorID, ev.PrevVisitorID} {
		if id == 0 {
			continue
		}
		if s, ok := w.sessions[sessionKey{ev.SiteID, id}]; ok && ev.TS.Sub(s.lastSeen) <= SessionTimeout {
			w.sessions[sessionKey{ev.SiteID, ev.VisitorID}] = s
			return s, nil
		}
	}
	// After the first SessionTimeout every open visit was seen by this process, so memory is
	// complete and the database is not asked again.
	if time.Since(w.started) > SessionTimeout {
		return nil, nil
	}
	ids := []int64{ev.VisitorID}
	if ev.PrevVisitorID != 0 {
		ids = append(ids, ev.PrevVisitorID)
	}
	s := &session{pages: map[string]int64{}}
	d := &s.dims
	err := w.pool.QueryRow(ctx, `
		SELECT id, visitor_id, last_seen_at, hostname, source, referrer, utm_source, utm_medium,
		       utm_campaign, country, browser, os, device
		FROM sessions
		WHERE site_id = $1 AND visitor_id = ANY($2) AND last_seen_at >= $3
		ORDER BY last_seen_at DESC LIMIT 1`,
		ev.SiteID, ids, ev.TS.Add(-SessionTimeout),
	).Scan(&s.id, &s.visitor, &s.lastSeen, &d.Hostname, &d.Source, &d.Referrer, &d.UTMSource,
		&d.UTMMedium, &d.UTMCampaign, &d.Country, &d.Browser, &d.OS, &d.Device)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	w.sessions[sessionKey{ev.SiteID, ev.VisitorID}] = s
	return s, nil
}

func (w *Worker) pageview(ctx context.Context, ev Event) error {
	s, err := w.resolve(ctx, ev)
	if err != nil {
		return err
	}
	if s == nil {
		return w.newSession(ctx, ev)
	}
	d := s.dims
	var eventID int64
	err = w.pool.QueryRow(ctx, `
		WITH s AS (
			UPDATE sessions
			SET pageviews = pageviews + 1, last_seen_at = greatest(last_seen_at, $4), exit_path = $5
			WHERE id = $2
		)
		INSERT INTO events (site_id, session_id, visitor_id, ts, name, path, hostname, source,
		                    referrer, utm_source, utm_medium, utm_campaign, country, browser, os, device)
		VALUES ($1, $2, $3, $4, 'pageview', $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		RETURNING id`,
		ev.SiteID, s.id, s.visitor, ev.TS, ev.Path, ev.Hostname, d.Source, d.Referrer,
		d.UTMSource, d.UTMMedium, d.UTMCampaign, d.Country, d.Browser, d.OS, d.Device,
	).Scan(&eventID)
	if err != nil {
		return err
	}
	if ev.TS.After(s.lastSeen) {
		s.lastSeen = ev.TS
	}
	s.rememberPage(ev.Path, eventID)
	return nil
}

func (w *Worker) newSession(ctx context.Context, ev Event) error {
	d := ev.Dims
	var sessionID, eventID int64
	err := w.pool.QueryRow(ctx, `
		WITH s AS (
			INSERT INTO sessions (site_id, visitor_id, started_at, last_seen_at, pageviews, entry_path,
			                      exit_path, hostname, source, referrer, utm_source, utm_medium,
			                      utm_campaign, country, browser, os, device)
			VALUES ($1, $2, $3, $3, 1, $4, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
			RETURNING id
		)
		INSERT INTO events (site_id, session_id, visitor_id, ts, name, path, hostname, source,
		                    referrer, utm_source, utm_medium, utm_campaign, country, browser, os, device)
		SELECT $1::text, s.id, $2::bigint, $3::timestamptz, 'pageview', $4::text, $15::text, $6::text,
		       $7::text, $8::text, $9::text, $10::text, $11::text, $12::text, $13::text, $14::text
		FROM s
		RETURNING session_id, id`,
		ev.SiteID, ev.VisitorID, ev.TS, ev.Path, d.Hostname, d.Source, d.Referrer, d.UTMSource,
		d.UTMMedium, d.UTMCampaign, d.Country, d.Browser, d.OS, d.Device, ev.Hostname,
	).Scan(&sessionID, &eventID)
	if err != nil {
		return err
	}
	s := &session{id: sessionID, visitor: ev.VisitorID, lastSeen: ev.TS, dims: d, pages: map[string]int64{}}
	s.rememberPage(ev.Path, eventID)
	w.sessions[sessionKey{ev.SiteID, ev.VisitorID}] = s
	return nil
}

// engagement adds visible time to the visit and to the latest pageview of that path. It never
// writes a row of its own.
func (w *Worker) engagement(ctx context.Context, ev Event) error {
	s, err := w.resolve(ctx, ev)
	if err != nil {
		return err
	}
	if s == nil {
		w.counters.Orphaned.Add(1)
		return nil
	}
	if eventID, ok := s.pages[ev.Path]; ok {
		_, err = w.pool.Exec(ctx, `
			WITH s AS (
				UPDATE sessions SET engaged_ms = engaged_ms + $2, last_seen_at = greatest(last_seen_at, $3)
				WHERE id = $1
			)
			UPDATE events SET engaged_ms = engaged_ms + $2 WHERE id = $4`,
			s.id, ev.EngagedMS, ev.TS, eventID)
	} else {
		// After a restart the path map is empty: find the pageview in the database.
		_, err = w.pool.Exec(ctx, `
			WITH s AS (
				UPDATE sessions SET engaged_ms = engaged_ms + $2, last_seen_at = greatest(last_seen_at, $3)
				WHERE id = $1
			)
			UPDATE events SET engaged_ms = engaged_ms + $2
			WHERE id = (
				SELECT id FROM events
				WHERE site_id = $4 AND session_id = $1 AND path = $5 AND name = 'pageview'
				  AND ts >= $3::timestamptz - interval '1 day'
				ORDER BY ts DESC LIMIT 1
			)`,
			s.id, ev.EngagedMS, ev.TS, ev.SiteID, ev.Path)
	}
	if err != nil {
		return err
	}
	if ev.TS.After(s.lastSeen) {
		s.lastSeen = ev.TS
	}
	return nil
}

func (s *session) rememberPage(path string, eventID int64) {
	if _, ok := s.pages[path]; !ok && len(s.pages) >= maxPagesPerSession {
		return
	}
	s.pages[path] = eventID
}
