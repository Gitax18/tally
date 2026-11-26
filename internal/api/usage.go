package api

import (
	"math"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Gitax18/tally/internal/httpx"
	"github.com/Gitax18/tally/internal/ingest"
)

// WarnPercent is when the storage meter turns into a warning. Heroku Essential plans stop
// accepting writes once the database is full, so the warning has to come well before that.
const WarnPercent = 80

// Usage reports database size against the plan limit, plus the ingest counters.
type Usage struct {
	Pool       *pgxpool.Pool
	LimitBytes int64
	Counters   *ingest.Counters
	Queued     func() int
}

func (u *Usage) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var size, events int64
	err := u.Pool.QueryRow(r.Context(), `
		SELECT pg_database_size(current_database()),
		       greatest((SELECT reltuples FROM pg_class WHERE oid = 'events'::regclass), 0)::bigint`).Scan(&size, &events)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	percent := math.Round(1000*float64(size)/float64(u.LimitBytes)) / 10
	c := u.Counters
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"database_bytes":  size,
		"limit_bytes":     u.LimitBytes,
		"percent":         percent,
		"warn":            percent >= WarnPercent,
		"events_estimate": events,
		"ingest": map[string]int64{
			"accepted":     c.Accepted.Load(),
			"stored":       c.Stored.Load(),
			"dropped":      c.Dropped.Load(),
			"failed":       c.Failed.Load(),
			"bots":         c.Bots.Load(),
			"rejected":     c.Rejected.Load(),
			"rate_limited": c.RateLimited.Load(),
			"orphaned":     c.Orphaned.Load(),
			"queued":       int64(u.Queued()),
		},
	})
}
