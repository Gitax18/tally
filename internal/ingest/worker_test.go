package ingest

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Gitax18/tally/internal/testdb"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

func setup(t *testing.T) (*pgxpool.Pool, *Worker, *Counters) {
	t.Helper()
	pool := testdb.New(t)
	if _, err := pool.Exec(context.Background(), `INSERT INTO sites (id, name, domain) VALUES ('site000001', 'Test', 'example.com')`); err != nil {
		t.Fatal(err)
	}
	c := &Counters{}
	return pool, NewWorker(pool, quiet, c, 100, nil), c
}

var t0 = time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)

func pv(visitor int64, path string, at time.Time) Event {
	return Event{
		SiteID: "site000001", Kind: KindPageview, TS: at, VisitorID: visitor, Path: path, Hostname: "example.com",
		Dims: Dims{Hostname: "example.com", Source: "GitHub", Referrer: "github.com", Country: "IN", Browser: "Firefox", OS: "Linux", Device: "Desktop"},
	}
}

func eng(visitor int64, path string, ms int64, at time.Time) Event {
	return Event{SiteID: "site000001", Kind: KindEngagement, TS: at, VisitorID: visitor, Path: path, EngagedMS: ms}
}

func run(t *testing.T, w *Worker, events ...Event) {
	t.Helper()
	ctx := context.Background()
	for _, ev := range events {
		w.process(ctx, ev)
	}
}

type sessionRow struct {
	visitor             int64
	pageviews           int
	engaged             int64
	entry, exit, source string
}

func sessionsOf(t *testing.T, pool *pgxpool.Pool) []sessionRow {
	t.Helper()
	rows, err := pool.Query(context.Background(), `SELECT visitor_id, pageviews, engaged_ms, entry_path, exit_path, source FROM sessions ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []sessionRow
	for rows.Next() {
		var s sessionRow
		if err := rows.Scan(&s.visitor, &s.pageviews, &s.engaged, &s.entry, &s.exit, &s.source); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	return out
}

func TestSessionBoundaryIsThirtyMinutes(t *testing.T) {
	pool, w, c := setup(t)
	run(t, w,
		pv(1, "/", t0),
		pv(1, "/docs", t0.Add(10*time.Minute)),
		pv(1, "/pricing", t0.Add(40*time.Minute)),      // exactly 30 min later: same visit
		pv(1, "/", t0.Add(70*time.Minute+time.Second)), // 30 min and 1 s later: new visit
	)
	got := sessionsOf(t, pool)
	if len(got) != 2 {
		t.Fatalf("want 2 visits, got %d: %+v", len(got), got)
	}
	if got[0].pageviews != 3 || got[0].entry != "/" || got[0].exit != "/pricing" {
		t.Errorf("first visit: %+v", got[0])
	}
	if got[1].pageviews != 1 || got[1].entry != "/" {
		t.Errorf("second visit: %+v", got[1])
	}
	if c.Stored.Load() != 4 || c.Failed.Load() != 0 {
		t.Errorf("stored %d failed %d", c.Stored.Load(), c.Failed.Load())
	}

	// Every event carries the visit's attributes.
	var n int
	_ = pool.QueryRow(context.Background(), `SELECT count(*) FROM events WHERE source = 'GitHub' AND country = 'IN' AND browser = 'Firefox' AND name = 'pageview'`).Scan(&n)
	if n != 4 {
		t.Errorf("want 4 events with the visit's dimensions, got %d", n)
	}
}

func TestEngagementAddsToVisitAndPageview(t *testing.T) {
	pool, w, _ := setup(t)
	run(t, w,
		pv(1, "/", t0),
		eng(1, "/", 4000, t0.Add(5*time.Second)),
		pv(1, "/docs", t0.Add(6*time.Second)),
		eng(1, "/docs", 2500, t0.Add(9*time.Second)),
		eng(1, "/docs", 500, t0.Add(20*time.Second)),
	)
	got := sessionsOf(t, pool)
	if len(got) != 1 || got[0].engaged != 7000 || got[0].pageviews != 2 {
		t.Fatalf("visit: %+v", got)
	}
	rows, _ := pool.Query(context.Background(), `SELECT path, engaged_ms FROM events ORDER BY id`)
	defer rows.Close()
	want := map[string]int64{"/": 4000, "/docs": 3000}
	for rows.Next() {
		var path string
		var ms int64
		_ = rows.Scan(&path, &ms)
		if want[path] != ms {
			t.Errorf("%s: engaged %d, want %d", path, ms, want[path])
		}
	}
	var total int
	_ = pool.QueryRow(context.Background(), `SELECT count(*) FROM events`).Scan(&total)
	if total != 2 {
		t.Errorf("engagement pings must not write rows: %d events", total)
	}
}

func TestEngagementWithoutAVisitIsDropped(t *testing.T) {
	pool, w, c := setup(t)
	run(t, w, eng(9, "/", 1000, t0))
	if c.Orphaned.Load() != 1 || len(sessionsOf(t, pool)) != 0 {
		t.Fatal("an engagement with no open visit should be counted and dropped")
	}
}

func TestVisitContinuesAcrossTheSaltChange(t *testing.T) {
	pool, w, _ := setup(t)
	before := time.Date(2026, 11, 27, 23, 55, 0, 0, time.UTC)
	after := before.Add(10 * time.Minute)
	next := pv(2, "/b", after) // the new day's ID for the same person...
	next.PrevVisitorID = 1     // ...and their ID under yesterday's salt
	run(t, w, pv(1, "/a", before), next, pv(2, "/c", after.Add(time.Minute)))
	got := sessionsOf(t, pool)
	if len(got) != 1 || got[0].pageviews != 3 || got[0].visitor != 1 {
		t.Fatalf("the visit should continue under its original ID: %+v", got)
	}
}

func TestVisitIsFoundInTheDatabaseAfterARestart(t *testing.T) {
	pool, w, _ := setup(t)
	now := time.Now().UTC()
	run(t, w, pv(1, "/", now.Add(-5*time.Minute)))

	restarted := NewWorker(pool, quiet, &Counters{}, 100, nil)
	restarted.started = time.Now()
	run(t, restarted, pv(1, "/docs", now), eng(1, "/", 1500, now))

	got := sessionsOf(t, pool)
	if len(got) != 1 || got[0].pageviews != 2 || got[0].engaged != 1500 {
		t.Fatalf("the restarted worker should continue the visit: %+v", got)
	}
	var ms int64
	_ = pool.QueryRow(context.Background(), `SELECT engaged_ms FROM events WHERE path = '/'`).Scan(&ms)
	if ms != 1500 {
		t.Fatalf("engagement after a restart should find the pageview in the database: %d", ms)
	}
}

func TestQueueDrainsOnClose(t *testing.T) {
	pool, w, _ := setup(t)
	for i := range 20 {
		if !w.Push(pv(int64(i+1), "/", t0)) {
			t.Fatal("push refused")
		}
	}
	go w.Run(context.Background())
	w.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := w.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	if len(sessionsOf(t, pool)) != 20 {
		t.Fatal("every queued event should be written before Run returns")
	}
	if w.Push(pv(99, "/", t0)) {
		t.Fatal("a closed queue must refuse events")
	}
}

func TestFullQueueRefuses(t *testing.T) {
	_, _, c := setup(t)
	w := NewWorker(nil, quiet, c, 1, nil)
	if !w.Push(pv(1, "/", t0)) || w.Push(pv(2, "/", t0)) {
		t.Fatal("a full queue should refuse instead of blocking")
	}
}
