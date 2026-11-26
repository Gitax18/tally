// Command server is the tally HTTP service: the tracker script, event collection, and the
// private stats API the dashboard reads.
//
//	server                 run the service
//	server hash-password   read a password from stdin and print ADMIN_PASSWORD_HASH
package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata" // site time zones work even where the OS has no zoneinfo

	"golang.org/x/term"

	"github.com/Gitax18/tally/internal/api"
	"github.com/Gitax18/tally/internal/auth"
	"github.com/Gitax18/tally/internal/clientip"
	"github.com/Gitax18/tally/internal/config"
	"github.com/Gitax18/tally/internal/db"
	"github.com/Gitax18/tally/internal/geo"
	"github.com/Gitax18/tally/internal/ingest"
	"github.com/Gitax18/tally/internal/ratelimit"
	"github.com/Gitax18/tally/internal/retention"
	"github.com/Gitax18/tally/internal/sites"
	"github.com/Gitax18/tally/internal/visitor"
)

const (
	queueSize        = 10_000
	sitesRefresh     = time.Minute
	eventsPerWindow  = 30
	eventsWindow     = 10 * time.Second
	loginsPerMinute  = 5
	countersInterval = 5 * time.Minute
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.Getenv, os.Stdin, os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

// run is main without the process-level pieces. Exit codes: 0 clean stop, 1 runtime failure,
// 2 bad configuration.
func run(ctx context.Context, args []string, getenv func(string) string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "hash-password":
			return hashPassword(stdin, stdout, stderr)
		default:
			_, _ = fmt.Fprintf(stderr, "unknown command %q (did you mean hash-password?)\n", args[0])
			return 2
		}
	}

	cfg, err := config.Load(getenv)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "configuration error:\n%v\n", err)
		return 2
	}
	if err := auth.ValidateHash(cfg.AdminPasswordHash); err != nil {
		_, _ = fmt.Fprintf(stderr, "configuration error:\nADMIN_PASSWORD_HASH: %v\n", err)
		return 2
	}
	logger := slog.New(slog.NewJSONHandler(stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))

	pool, err := db.Open(ctx, cfg.DatabaseURL, 10)
	if err != nil {
		logger.Error(err.Error())
		return 2
	}
	defer pool.Close()

	siteStore := sites.NewStore(pool, logger)
	if err := siteStore.Refresh(ctx); err != nil {
		// Keep going: the refresh loop retries, and /healthz still answers for the router.
		logger.Error("sites: initial load failed; retrying every minute", "error", err)
	}

	countries := &geo.DB{}
	switch {
	case cfg.GeoIPPath != "":
		if countries, err = geo.Open(cfg.GeoIPPath); err != nil {
			logger.Error("geo: GEOIP_PATH could not be opened", "error", err)
			return 2
		}
	case cfg.GeoIPDownload:
		countries.DownloadInBackground(ctx, logger, &http.Client{Timeout: 2 * time.Minute})
	}

	counters := &ingest.Counters{}
	salts := visitor.NewSalts(pool)
	worker := ingest.NewWorker(pool, logger, counters, queueSize, siteStore.TouchLastEvent)
	ips := clientip.Resolver{TrustProxy: cfg.TrustProxy}

	handler := api.Router(api.Deps{
		Logger:    logger,
		APISecret: cfg.APISecret,
		Sites:     siteStore,
		Ingest: &ingest.Handler{
			Sites:    siteStore,
			Salts:    salts,
			Geo:      countries,
			Queue:    worker,
			IPs:      ips,
			Limiter:  ratelimit.New(eventsPerWindow, eventsWindow),
			Counters: counters,
			Logger:   logger,
			Now:      time.Now,
		},
		Login: (&auth.Login{
			Hash:    cfg.AdminPasswordHash,
			Limiter: ratelimit.New(loginsPerMinute, time.Minute),
			IPs:     ips,
			Logger:  logger,
		}).ServeHTTP,
	})

	// Background work stops with bgCtx; the worker's writes use a context that survives the
	// signal, so the queue can drain after the HTTP server has stopped.
	bgCtx, cancelBG := context.WithCancel(ctx)
	defer cancelBG()
	go siteStore.RunRefresh(bgCtx, sitesRefresh)
	go (&retention.Job{Pool: pool, Salts: salts, Months: cfg.RetentionMonths, Logger: logger}).Run(bgCtx)
	go logCounters(bgCtx, logger, counters, worker)
	go worker.Run(context.WithoutCancel(ctx))

	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", cfg.Addr)
	if err != nil {
		logger.Error("cannot listen", "addr", cfg.Addr, "error", err)
		return 1
	}
	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       90 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()
	logger.Info("listening", "addr", ln.Addr().String(), "trust_proxy", cfg.TrustProxy, "retention_months", cfg.RetentionMonths)

	code := 0
	select {
	case err := <-serveErr:
		logger.Error("server failed", "error", err)
		code = 1
	case <-ctx.Done():
		logger.Info("shutting down")
	}

	// Heroku allows 30 seconds after SIGTERM: stop taking requests, then write what is queued.
	deadline, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(deadline); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("http shutdown", "error", err)
	}
	queued := worker.Len()
	worker.Close()
	if err := worker.Wait(deadline); err != nil {
		logger.Error("queue did not drain in time", "left", worker.Len())
		code = 1
	} else {
		logger.Info("queue drained", "events", queued)
	}
	cancelBG()
	return code
}

// logCounters writes the ingest counters every few minutes, so a quiet `heroku logs` still shows
// what is being collected, dropped or rejected.
func logCounters(ctx context.Context, logger *slog.Logger, c *ingest.Counters, w *ingest.Worker) {
	t := time.NewTicker(countersInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			logger.Info("ingest: counters",
				"accepted", c.Accepted.Load(), "stored", c.Stored.Load(), "dropped", c.Dropped.Load(),
				"failed", c.Failed.Load(), "bots", c.Bots.Load(), "rejected", c.Rejected.Load(),
				"rate_limited", c.RateLimited.Load(), "orphaned", c.Orphaned.Load(), "queued", w.Len())
		}
	}
}

// hashPassword prints an ADMIN_PASSWORD_HASH for a password typed twice at a terminal, or read
// as the first line of piped input.
func hashPassword(stdin io.Reader, stdout, stderr io.Writer) int {
	var password string
	if f, ok := stdin.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		_, _ = fmt.Fprint(stderr, "Admin password: ")
		first, err := term.ReadPassword(int(f.Fd()))
		_, _ = fmt.Fprintln(stderr)
		if err != nil {
			_, _ = fmt.Fprintln(stderr, "could not read the password:", err)
			return 1
		}
		_, _ = fmt.Fprint(stderr, "Again: ")
		second, err := term.ReadPassword(int(f.Fd()))
		_, _ = fmt.Fprintln(stderr)
		if err != nil || string(first) != string(second) {
			_, _ = fmt.Fprintln(stderr, "the passwords do not match")
			return 1
		}
		password = string(first)
	} else {
		line, err := bufio.NewReader(stdin).ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			_, _ = fmt.Fprintln(stderr, "could not read the password:", err)
			return 1
		}
		password = strings.TrimRight(line, "\r\n")
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	_, _ = fmt.Fprintln(stdout, hash)
	return 0
}
