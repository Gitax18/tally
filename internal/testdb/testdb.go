// Package testdb gives integration tests a migrated, empty database.
//
// Tests that use it are skipped unless TEST_DATABASE_URL is set (make test-integration and CI
// set it). Every call truncates all tables, so packages that use it must not run in parallel
// against the same database: `go test -p 1`.
package testdb

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Gitax18/tally/internal/db"
)

var migrateOnce sync.Once
var migrateErr error

// URL returns TEST_DATABASE_URL or skips the test.
func URL(t testing.TB) string {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	return url
}

// New returns a pool on a freshly truncated test database.
func New(t testing.TB) *pgxpool.Pool {
	t.Helper()
	url := URL(t)
	ctx := context.Background()
	migrateOnce.Do(func() { _, migrateErr = db.Migrate(ctx, url) })
	if migrateErr != nil {
		t.Fatalf("migrate: %v", migrateErr)
	}
	pool, err := db.Open(ctx, url, 8)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, `TRUNCATE events, sessions, salts, sites RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return pool
}
