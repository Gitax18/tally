// Command server is the tally HTTP service: the tracker script, event collection, and the
// stats API the dashboard reads.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Gitax18/tally/internal/config"
	"github.com/Gitax18/tally/internal/db"
	"github.com/Gitax18/tally/internal/httpx"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.Getenv, os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

// run is main without the process-level pieces. Exit codes: 0 clean stop, 1 runtime failure,
// 2 bad configuration.
func run(ctx context.Context, _ []string, getenv func(string) string, stdout, stderr io.Writer) int {
	cfg, err := config.Load(getenv)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "configuration error:\n%v\n", err)
		return 2
	}
	logger := slog.New(slog.NewJSONHandler(stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))

	pool, err := db.Open(ctx, cfg.DatabaseURL, 10)
	if err != nil {
		logger.Error(err.Error())
		return 2
	}
	defer pool.Close()

	r := chi.NewRouter()
	r.Use(httpx.Recover(logger))
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "ok\n") })

	srv := &http.Server{Addr: cfg.Addr, Handler: r, ReadHeaderTimeout: 5 * time.Second}
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	logger.Info("listening", "addr", cfg.Addr)

	select {
	case err := <-errc:
		logger.Error("server failed", "error", err)
		return 1
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("shutdown", "error", err)
		return 1
	}
	return 0
}
