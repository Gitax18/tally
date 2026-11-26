// Command migrate applies the database schema. Heroku runs it in the release phase.
//
//	migrate            apply pending migrations (same as "up")
//	migrate up         apply pending migrations
//	migrate status     list migrations and whether they are applied
//	migrate down       revert the latest migration (needs ALLOW_MIGRATE_DOWN=1)
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/Gitax18/tally/internal/db"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.Getenv, os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

func run(ctx context.Context, args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	cmd := "up"
	if len(args) > 0 {
		cmd = args[0]
	}
	url := strings.TrimSpace(getenv("DATABASE_URL"))
	if url == "" {
		_, _ = fmt.Fprintln(stderr, "migrate: DATABASE_URL is required")
		return 2
	}

	var err error
	switch cmd {
	case "up":
		var n int
		n, err = db.Migrate(ctx, url)
		if err == nil {
			_, _ = fmt.Fprintf(stdout, "migrate: %d migration(s) applied\n", n)
		}
	case "status":
		err = db.Status(ctx, url, stdout)
	case "down":
		if getenv("ALLOW_MIGRATE_DOWN") != "1" {
			_, _ = fmt.Fprintln(stderr, "migrate: down deletes data; set ALLOW_MIGRATE_DOWN=1 to run it")
			return 2
		}
		err = db.MigrateDown(ctx, url)
		if err == nil {
			_, _ = fmt.Fprintln(stdout, "migrate: reverted the latest migration")
		}
	default:
		_, _ = fmt.Fprintf(stderr, "migrate: unknown command %q (use up, status or down)\n", cmd)
		return 2
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "migrate: %v\n", err)
		return 1
	}
	return 0
}
