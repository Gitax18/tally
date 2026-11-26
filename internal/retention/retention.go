// Package retention deletes data older than RETENTION_MONTHS and salts older than yesterday.
//
// It runs in-process once a day. A Postgres advisory lock makes sure only one process does the
// work when two briefly overlap (during a deploy).
package retention

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// lockKey identifies the retention job's advisory lock ("tally" in ASCII).
const lockKey = 0x74616c6c79

// batchSize keeps each DELETE short, so it never holds locks the event writer waits on for long.
const batchSize = 5000

// SaltCleaner deletes old salts.
type SaltCleaner interface {
	DeleteOld(ctx context.Context, now time.Time) (int64, error)
}

// Job is the daily cleanup.
type Job struct {
	Pool   *pgxpool.Pool
	Salts  SaltCleaner
	Months int
	Logger *slog.Logger
}

// Result says what one run removed.
type Result struct {
	Ran      bool // false when another process held the lock
	Events   int64
	Sessions int64
	Salts    int64
}

// Run waits a minute after boot, then runs once a day until ctx ends.
func (j *Job) Run(ctx context.Context) {
	timer := time.NewTimer(time.Minute)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		res, err := j.Once(ctx, time.Now())
		switch {
		case err != nil && ctx.Err() == nil:
			j.Logger.Error("retention: failed", "error", err)
		case res.Ran:
			j.Logger.Info("retention: done", "events", res.Events, "sessions", res.Sessions, "salts", res.Salts)
		}
		timer.Reset(24 * time.Hour)
	}
}

// Once deletes everything older than the retention period, site by site in batches.
func (j *Job) Once(ctx context.Context, now time.Time) (Result, error) {
	var res Result
	conn, err := j.Pool.Acquire(ctx)
	if err != nil {
		return res, err
	}
	defer conn.Release()

	// Advisory locks belong to a connection, so lock and unlock on this one.
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, int64(lockKey)).Scan(&res.Ran); err != nil {
		return res, err
	}
	if !res.Ran {
		return res, nil
	}
	defer func() {
		_, _ = conn.Exec(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock($1)`, int64(lockKey))
	}()

	res.Salts, err = j.Salts.DeleteOld(ctx, now)
	if err != nil {
		return res, fmt.Errorf("salts: %w", err)
	}

	cutoff := now.AddDate(0, -j.Months, 0)
	rows, err := conn.Query(ctx, `SELECT id FROM sites`)
	if err != nil {
		return res, err
	}
	var siteIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return res, err
		}
		siteIDs = append(siteIDs, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return res, err
	}

	for _, site := range siteIDs {
		n, err := deleteBatches(ctx, j.Pool, `
			DELETE FROM events WHERE id IN (
				SELECT id FROM events WHERE site_id = $1 AND ts < $2 LIMIT $3)`, site, cutoff)
		res.Events += n
		if err != nil {
			return res, fmt.Errorf("events: %w", err)
		}
		n, err = deleteBatches(ctx, j.Pool, `
			DELETE FROM sessions WHERE id IN (
				SELECT id FROM sessions WHERE site_id = $1 AND started_at < $2 LIMIT $3)`, site, cutoff)
		res.Sessions += n
		if err != nil {
			return res, fmt.Errorf("sessions: %w", err)
		}
	}
	return res, nil
}

func deleteBatches(ctx context.Context, pool *pgxpool.Pool, sql, site string, cutoff time.Time) (int64, error) {
	var total int64
	for {
		tag, err := pool.Exec(ctx, sql, site, cutoff, batchSize)
		if err != nil {
			return total, err
		}
		total += tag.RowsAffected()
		if tag.RowsAffected() < batchSize {
			return total, nil
		}
	}
}
