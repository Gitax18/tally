package retention

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/Gitax18/tally/internal/testdb"
	"github.com/Gitax18/tally/internal/visitor"
)

func TestOnceDeletesOnlyExpiredData(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)
	old := now.AddDate(0, -25, 0)
	recent := now.AddDate(0, -23, 0)

	mustExec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	mustExec(`INSERT INTO sites (id, name, domain) VALUES ('site000001', 'A', 'a.com')`)
	for _, at := range []time.Time{old, recent} {
		mustExec(`INSERT INTO sessions (site_id, visitor_id, started_at, last_seen_at, pageviews, entry_path, exit_path)
		          VALUES ('site000001', 1, $1, $1, 1, '/', '/')`, at)
		mustExec(`INSERT INTO events (site_id, session_id, visitor_id, ts, name, path) VALUES ('site000001', 1, 1, $1, 'pageview', '/')`, at)
	}
	mustExec(`INSERT INTO salts (day, salt) VALUES ($1, $2), ($3, $2), ($4, $2)`,
		now.AddDate(0, 0, -3), make([]byte, 32), now.AddDate(0, 0, -1), now)

	job := &Job{Pool: pool, Salts: visitor.NewSalts(pool), Months: 24, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	res, err := job.Once(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Ran || res.Events != 1 || res.Sessions != 1 || res.Salts != 1 {
		t.Fatalf("result %+v", res)
	}
	var events, sessions, salts int
	_ = pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM events), (SELECT count(*) FROM sessions), (SELECT count(*) FROM salts)`).Scan(&events, &sessions, &salts)
	if events != 1 || sessions != 1 || salts != 2 {
		t.Fatalf("left %d events, %d sessions, %d salts", events, sessions, salts)
	}
}

func TestOnceSkipsWhenAnotherProcessHoldsTheLock(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	other, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Release()
	if _, err := other.Exec(ctx, `SELECT pg_advisory_lock($1)`, int64(lockKey)); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = other.Exec(ctx, `SELECT pg_advisory_unlock($1)`, int64(lockKey)) }()

	job := &Job{Pool: pool, Salts: visitor.NewSalts(pool), Months: 24, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	res, err := job.Once(ctx, time.Now())
	if err != nil || res.Ran {
		t.Fatalf("want a skipped run, got %+v %v", res, err)
	}
}
