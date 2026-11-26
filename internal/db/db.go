// Package db opens the Postgres pool and applies the embedded migrations.
//
// Errors never include the connection string: it carries the password.
package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"

	// Registers the "pgx" database/sql driver that goose uses.
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/Gitax18/tally/migrations"
)

// Open returns a connection pool. It does not connect eagerly, so the service starts while the
// database is briefly unavailable and reports that through its logs instead of crashing.
func Open(ctx context.Context, url string, maxConns int32) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, errors.New("DATABASE_URL could not be parsed")
	}
	if maxConns > 0 {
		cfg.MaxConns = maxConns
	}
	cfg.MaxConnIdleTime = 5 * time.Minute
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, errors.New("DATABASE_URL could not be used to create a connection pool")
	}
	return pool, nil
}

func provider(url string) (*goose.Provider, *sql.DB, error) {
	sqlDB, err := sql.Open("pgx", url)
	if err != nil {
		return nil, nil, errors.New("DATABASE_URL could not be used")
	}
	p, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS)
	if err != nil {
		_ = sqlDB.Close()
		return nil, nil, err
	}
	return p, sqlDB, nil
}

// Migrate applies every pending migration and returns how many ran.
func Migrate(ctx context.Context, url string) (int, error) {
	p, sqlDB, err := provider(url)
	if err != nil {
		return 0, err
	}
	defer func() { _ = sqlDB.Close() }()
	results, err := p.Up(ctx)
	return len(results), err
}

// MigrateDown reverts the most recent migration.
func MigrateDown(ctx context.Context, url string) error {
	p, sqlDB, err := provider(url)
	if err != nil {
		return err
	}
	defer func() { _ = sqlDB.Close() }()
	_, err = p.Down(ctx)
	return err
}

// Status writes one line per migration: applied or pending.
func Status(ctx context.Context, url string, w io.Writer) error {
	p, sqlDB, err := provider(url)
	if err != nil {
		return err
	}
	defer func() { _ = sqlDB.Close() }()
	statuses, err := p.Status(ctx)
	if err != nil {
		return err
	}
	for _, s := range statuses {
		state := "pending"
		if s.State == goose.StateApplied {
			state = "applied"
		}
		name := s.Source.Path
		if i := strings.LastIndex(name, "/"); i >= 0 {
			name = name[i+1:]
		}
		if _, err := fmt.Fprintf(w, "%-8s %s\n", state, name); err != nil {
			return err
		}
	}
	return nil
}
