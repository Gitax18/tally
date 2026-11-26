// Package sites stores the tracked websites and keeps an in-memory copy for the event path,
// refreshed every minute and after every write.
package sites

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound means no site has that ID.
var ErrNotFound = errors.New("site not found")

// ValidationError is a problem with user input. Its message is safe to show.
type ValidationError struct{ Field, Msg string }

func (e *ValidationError) Error() string { return e.Field + ": " + e.Msg }

// Site is one tracked website.
type Site struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Domain      string     `json:"domain"`
	Timezone    string     `json:"timezone"`
	CreatedAt   time.Time  `json:"created_at"`
	LastEventAt *time.Time `json:"last_event_at"`
}

// Location returns the site's time zone (UTC if it cannot be loaded).
func (s Site) Location() *time.Location {
	if loc, err := time.LoadLocation(s.Timezone); err == nil {
		return loc
	}
	return time.UTC
}

// IsLocal reports whether the site is a local test site, whose tracker runs on localhost.
func (s Site) IsLocal() bool { return s.Domain == "localhost" }

// HostAllowed reports whether a page on host may send events for the site: the site's domain or
// one of its subdomains. A localhost site also accepts the loopback addresses.
func (s Site) HostAllowed(host string) bool {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if s.IsLocal() {
		return host == "localhost" || host == "127.0.0.1" || host == "[::1]" || host == "::1"
	}
	return host == s.Domain || strings.HasSuffix(host, "."+s.Domain)
}

// NewID returns 10 random characters from [a-z0-9].
func NewID() string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	out := make([]byte, 0, 10)
	buf := make([]byte, 16)
	for len(out) < 10 {
		_, _ = rand.Read(buf)
		for _, b := range buf {
			// 252 = 7*36: rejecting the rest keeps every character equally likely.
			if b < 252 && len(out) < 10 {
				out = append(out, alphabet[b%36])
			}
		}
	}
	return string(out)
}

// ValidID reports whether id has the shape NewID produces.
func ValidID(id string) bool {
	if len(id) != 10 {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') {
			return false
		}
	}
	return true
}

// NormalizeDomain accepts what people paste ("https://www.Example.com/blog") and returns the bare
// host ("example.com"). Subdomains are then tracked automatically.
func NormalizeDomain(raw string) (string, error) {
	d := strings.ToLower(strings.TrimSpace(raw))
	for _, scheme := range []string{"https://", "http://"} {
		d = strings.TrimPrefix(d, scheme)
	}
	if i := strings.IndexAny(d, "/?#"); i >= 0 {
		d = d[:i]
	}
	if i := strings.LastIndexByte(d, ':'); i >= 0 {
		d = d[:i]
	}
	d = strings.TrimSuffix(d, ".")
	d = strings.TrimPrefix(d, "www.")
	if d == "localhost" {
		return d, nil
	}
	if len(d) == 0 || len(d) > 253 {
		return "", &ValidationError{"domain", "enter a domain such as example.com"}
	}
	labels := strings.Split(d, ".")
	if len(labels) < 2 {
		return "", &ValidationError{"domain", "enter a full domain such as example.com (or localhost for testing)"}
	}
	for _, l := range labels {
		if len(l) == 0 || len(l) > 63 || l[0] == '-' || l[len(l)-1] == '-' {
			return "", &ValidationError{"domain", "enter a domain such as example.com"}
		}
		for i := 0; i < len(l); i++ {
			c := l[i]
			if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
				return "", &ValidationError{"domain", "use the ASCII (punycode) form of the domain"}
			}
		}
	}
	return d, nil
}

func validateName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 100 {
		return "", &ValidationError{"name", "enter a name of 1 to 100 characters"}
	}
	return name, nil
}

func validateTimezone(tz string) (string, error) {
	tz = strings.TrimSpace(tz)
	if tz == "" {
		return "UTC", nil
	}
	if tz == "Local" {
		return "", &ValidationError{"timezone", "choose a time zone such as Europe/Berlin"}
	}
	if _, err := time.LoadLocation(tz); err != nil {
		return "", &ValidationError{"timezone", "unknown time zone; use an IANA name such as Europe/Berlin"}
	}
	return tz, nil
}

// Store reads and writes sites.
type Store struct {
	pool   *pgxpool.Pool
	logger *slog.Logger

	mu    sync.RWMutex
	cache map[string]Site

	touchMu sync.Mutex
	touched map[string]time.Time
}

// NewStore returns a store. Call Refresh before serving events.
func NewStore(pool *pgxpool.Pool, logger *slog.Logger) *Store {
	return &Store{pool: pool, logger: logger, cache: map[string]Site{}, touched: map[string]time.Time{}}
}

const columns = `id, name, domain, timezone, created_at, last_event_at`

func scan(row pgx.Row) (Site, error) {
	var s Site
	err := row.Scan(&s.ID, &s.Name, &s.Domain, &s.Timezone, &s.CreatedAt, &s.LastEventAt)
	return s, err
}

// List returns every site, oldest first.
func (s *Store) List(ctx context.Context) ([]Site, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+columns+` FROM sites ORDER BY created_at, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Site{}
	for rows.Next() {
		site, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, site)
	}
	return out, rows.Err()
}

// Get returns one site or ErrNotFound.
func (s *Store) Get(ctx context.Context, id string) (Site, error) {
	if !ValidID(id) {
		return Site{}, ErrNotFound
	}
	site, err := scan(s.pool.QueryRow(ctx, `SELECT `+columns+` FROM sites WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Site{}, ErrNotFound
	}
	return site, err
}

// Create adds a site with a new random ID.
func (s *Store) Create(ctx context.Context, name, domain, timezone string) (Site, error) {
	name, err := validateName(name)
	if err != nil {
		return Site{}, err
	}
	domain, err = NormalizeDomain(domain)
	if err != nil {
		return Site{}, err
	}
	timezone, err = validateTimezone(timezone)
	if err != nil {
		return Site{}, err
	}
	site, err := scan(s.pool.QueryRow(ctx,
		`INSERT INTO sites (id, name, domain, timezone) VALUES ($1, $2, $3, $4) RETURNING `+columns,
		NewID(), name, domain, timezone))
	if err != nil {
		return Site{}, fmt.Errorf("creating the site: %w", err)
	}
	s.put(site)
	return site, nil
}

// Patch is a partial update; nil fields are left unchanged.
type Patch struct {
	Name     *string `json:"name"`
	Domain   *string `json:"domain"`
	Timezone *string `json:"timezone"`
}

// Update applies a patch and returns the updated site.
func (s *Store) Update(ctx context.Context, id string, p Patch) (Site, error) {
	site, err := s.Get(ctx, id)
	if err != nil {
		return Site{}, err
	}
	if p.Name != nil {
		if site.Name, err = validateName(*p.Name); err != nil {
			return Site{}, err
		}
	}
	if p.Domain != nil {
		if site.Domain, err = NormalizeDomain(*p.Domain); err != nil {
			return Site{}, err
		}
	}
	if p.Timezone != nil {
		if site.Timezone, err = validateTimezone(*p.Timezone); err != nil {
			return Site{}, err
		}
	}
	site, err = scan(s.pool.QueryRow(ctx,
		`UPDATE sites SET name = $2, domain = $3, timezone = $4 WHERE id = $1 RETURNING `+columns,
		id, site.Name, site.Domain, site.Timezone))
	if errors.Is(err, pgx.ErrNoRows) {
		return Site{}, ErrNotFound
	}
	if err != nil {
		return Site{}, fmt.Errorf("updating the site: %w", err)
	}
	s.put(site)
	return site, nil
}

// Delete removes a site and, through the foreign keys, all of its data.
func (s *Store) Delete(ctx context.Context, id string) error {
	if !ValidID(id) {
		return ErrNotFound
	}
	tag, err := s.pool.Exec(ctx, `DELETE FROM sites WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting the site: %w", err)
	}
	s.mu.Lock()
	delete(s.cache, id)
	s.mu.Unlock()
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) put(site Site) {
	s.mu.Lock()
	s.cache[site.ID] = site
	s.mu.Unlock()
}

// Lookup returns a site from memory. It is what the event and script endpoints use, so they
// never touch the database to validate a site.
func (s *Store) Lookup(id string) (Site, bool) {
	s.mu.RLock()
	site, ok := s.cache[id]
	s.mu.RUnlock()
	return site, ok
}

// Refresh reloads the in-memory copy from the database.
func (s *Store) Refresh(ctx context.Context) error {
	list, err := s.List(ctx)
	if err != nil {
		return err
	}
	m := make(map[string]Site, len(list))
	for _, site := range list {
		m[site.ID] = site
	}
	s.mu.Lock()
	s.cache = m
	s.mu.Unlock()
	return nil
}

// RunRefresh refreshes the copy every interval until ctx ends.
func (s *Store) RunRefresh(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := s.Refresh(ctx); err != nil && ctx.Err() == nil {
				s.logger.Warn("sites: refresh failed", "error", err)
			}
		}
	}
}

// TouchLastEvent records that a site received an event. It writes at most once a minute per
// site; the "waiting for first event" check only needs to know that one arrived.
func (s *Store) TouchLastEvent(ctx context.Context, id string, at time.Time) {
	s.touchMu.Lock()
	last, ok := s.touched[id]
	if ok && at.Sub(last) < time.Minute {
		s.touchMu.Unlock()
		return
	}
	s.touched[id] = at
	s.touchMu.Unlock()
	if _, err := s.pool.Exec(ctx, `UPDATE sites SET last_event_at = $2 WHERE id = $1 AND (last_event_at IS NULL OR last_event_at < $2)`, id, at); err != nil {
		s.logger.Warn("sites: recording the last event failed", "error", err)
	}
}
