// Command seed fills a "Demo" site (domain demo.example) with made-up visits, for working on the
// dashboard locally and for screenshots. It only ever touches that one site, and only with -yes:
//
//	DATABASE_URL=... go run ./tools/seed -yes [-days 90] [-visits 140] [-timezone Europe/Berlin]
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"time"
	_ "time/tzdata"

	"github.com/jackc/pgx/v5"

	"github.com/Gitax18/tally/internal/db"
	"github.com/Gitax18/tally/internal/ingest"
	"github.com/Gitax18/tally/internal/sites"
)

const demoDomain = "demo.example"

type weighted[T any] struct {
	v T
	w float64
}

func pick[T any](r *rand.Rand, items []weighted[T]) T {
	var total float64
	for _, it := range items {
		total += it.w
	}
	x := r.Float64() * total
	for _, it := range items {
		if x -= it.w; x < 0 {
			return it.v
		}
	}
	return items[len(items)-1].v
}

// source is what a visitor's browser sends; ingest.Source turns it into the stored source name.
type source struct{ referrer, utmSource, utmMedium, utmCampaign string }

var hackerNews = source{referrer: "https://news.ycombinator.com/item?id=1"}

var (
	sources = []weighted[source]{
		{source{}, 34},
		{source{referrer: "https://www.google.com/"}, 24},
		{source{referrer: "https://github.com/someone/awesome-selfhosted"}, 11},
		{hackerNews, 7},
		{source{referrer: "https://www.reddit.com/r/selfhosted/"}, 5},
		{source{referrer: "https://duckduckgo.com/"}, 4},
		{source{referrer: "https://t.co/abc"}, 3},
		{source{referrer: "https://www.bing.com/"}, 2},
		{source{utmSource: "newsletter", utmMedium: "email", utmCampaign: "november"}, 5},
		{source{utmSource: "chatgpt.com"}, 3},
		{source{referrer: "https://lobste.rs/s/abc"}, 2},
	}
	entryPages = []weighted[string]{
		{"/", 40}, {"/blog/self-hosting-analytics", 14}, {"/blog/postgres-is-enough", 10},
		{"/docs", 9}, {"/blog/one-dyno", 7}, {"/pricing", 5}, {"/about", 4}, {"/docs/install", 6}, {"/changelog", 5},
	}
	nextPages = []weighted[string]{
		{"/docs", 20}, {"/pricing", 14}, {"/docs/install", 14}, {"/about", 8}, {"/", 18}, {"/blog", 12}, {"/changelog", 6}, {"/docs/api", 8},
	}
	countries = []weighted[string]{
		{"US", 30}, {"IN", 14}, {"DE", 10}, {"GB", 8}, {"CA", 5}, {"FR", 5}, {"NL", 4}, {"BR", 4}, {"JP", 3}, {"AU", 3}, {"SE", 2}, {"PL", 2}, {"", 2},
	}
	clients = []weighted[[3]string]{
		{[3]string{"Chrome", "Windows", "Desktop"}, 24}, {[3]string{"Chrome", "macOS", "Desktop"}, 16},
		{[3]string{"Safari", "macOS", "Desktop"}, 10}, {[3]string{"Firefox", "Linux", "Desktop"}, 7},
		{[3]string{"Firefox", "Windows", "Desktop"}, 4}, {[3]string{"Edge", "Windows", "Desktop"}, 6},
		{[3]string{"Safari", "iOS", "Mobile"}, 16}, {[3]string{"Chrome", "Android", "Mobile"}, 12},
		{[3]string{"Samsung Browser", "Android", "Mobile"}, 2}, {[3]string{"Safari", "iOS", "Tablet"}, 3},
	}
)

func main() {
	yes := flag.Bool("yes", false, "really write demo data")
	days := flag.Int("days", 90, "days of history")
	perDay := flag.Float64("visits", 140, "average visits per day")
	tz := flag.String("timezone", "Europe/Berlin", "the demo site's time zone")
	flag.Parse()
	url := os.Getenv("DATABASE_URL")
	if !*yes || url == "" {
		fmt.Fprintln(os.Stderr, "seed: writes fake visits for the site demo.example; run with DATABASE_URL=... and -yes")
		os.Exit(2)
	}
	ctx := context.Background()
	pool, err := db.Open(ctx, url, 4)
	if err != nil {
		fmt.Fprintln(os.Stderr, "seed:", err)
		os.Exit(1)
	}
	defer pool.Close()
	loc, err := time.LoadLocation(*tz)
	if err != nil {
		fmt.Fprintln(os.Stderr, "seed: unknown time zone")
		os.Exit(2)
	}

	var siteID string
	err = pool.QueryRow(ctx, `SELECT id FROM sites WHERE domain = $1`, demoDomain).Scan(&siteID)
	if errors.Is(err, pgx.ErrNoRows) {
		siteID = sites.NewID()
		_, err = pool.Exec(ctx, `INSERT INTO sites (id, name, domain, timezone) VALUES ($1, 'Demo', $2, $3)`, siteID, demoDomain, *tz)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "seed:", err)
		os.Exit(1)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM events WHERE site_id = $1`, siteID); err != nil {
		fmt.Fprintln(os.Stderr, "seed:", err)
		os.Exit(1)
	}
	_, _ = pool.Exec(ctx, `DELETE FROM sessions WHERE site_id = $1`, siteID)
	_, _ = pool.Exec(ctx, `UPDATE sites SET timezone = $2, last_event_at = now() WHERE id = $1`, siteID, *tz)

	r := rand.New(rand.NewPCG(42, uint64(time.Now().Unix()/86400))) //nolint:gosec // fake data, not secrets
	now := time.Now()
	today := time.Date(now.In(loc).Year(), now.In(loc).Month(), now.In(loc).Day(), 0, 0, 0, 0, loc)
	visits, pageviews := 0, 0
	tx, err := pool.Begin(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "seed:", err)
		os.Exit(1)
	}
	var rows [][]any
	type person struct {
		id      int64
		client  [3]string
		country string
	}
	for d := *days - 1; d >= 0; d-- {
		day := today.AddDate(0, 0, -d)
		var seen []person // visitors are counted per day; some come back the same day
		// Growth over time, quieter weekends, and a launch spike three weeks ago.
		n := *perDay * (0.55 + 0.45*float64(*days-d)/float64(*days))
		if wd := day.Weekday(); wd == time.Saturday || wd == time.Sunday {
			n *= 0.65
		}
		switch d {
		case 21:
			n *= 4.2
		case 20:
			n *= 2.1
		}
		n *= 0.85 + 0.3*r.Float64()
		for range int(n) {
			// Busier in the local afternoon.
			hour := math.Mod(14+r.NormFloat64()*4.5+24, 24)
			start := day.Add(time.Duration(hour * float64(time.Hour)))
			if start.After(now) {
				continue
			}
			s := pick(r, sources)
			if d == 21 || d == 20 {
				if r.Float64() < 0.6 {
					s = hackerNews
				}
			}
			name, referrer := ingest.Source(s.utmSource, "", s.referrer, demoDomain)
			who := person{r.Int64(), pick(r, clients), pick(r, countries)}
			if len(seen) > 0 && r.Float64() < 0.15 {
				who = seen[r.IntN(len(seen))]
			} else {
				seen = append(seen, who)
			}
			visitor, c, country := who.id, who.client, who.country
			path := pick(r, entryPages)
			count := 1
			for r.Float64() < 0.52 && count < 9 {
				count++
			}
			ts := start
			type pv struct {
				path    string
				ts      time.Time
				engaged int64
			}
			var pvs []pv
			for i := range count {
				engaged := int64(math.Exp(r.NormFloat64()*0.9+3.2) * 1000) // ~25s median
				if count == 1 && r.Float64() < 0.4 {
					engaged = int64(r.IntN(6000))
				}
				pvs = append(pvs, pv{path, ts, engaged})
				ts = ts.Add(time.Duration(engaged)*time.Millisecond + time.Duration(r.IntN(4000))*time.Millisecond)
				if i < count-1 {
					path = pick(r, nextPages)
				}
			}
			var engagedTotal int64
			for _, p := range pvs {
				engagedTotal += p.engaged
			}
			last := pvs[len(pvs)-1]
			var sessionID int64
			err := tx.QueryRow(ctx, `
				INSERT INTO sessions (site_id, visitor_id, started_at, last_seen_at, pageviews, engaged_ms, entry_path, exit_path,
				                      hostname, source, referrer, utm_source, utm_medium, utm_campaign, country, browser, os, device)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18) RETURNING id`,
				siteID, visitor, start, last.ts, count, engagedTotal, pvs[0].path, last.path, demoDomain,
				name, referrer, s.utmSource, s.utmMedium, s.utmCampaign, country, c[0], c[1], c[2]).Scan(&sessionID)
			if err != nil {
				fmt.Fprintln(os.Stderr, "seed:", err)
				os.Exit(1)
			}
			for _, p := range pvs {
				rows = append(rows, []any{siteID, sessionID, visitor, p.ts, "pageview", p.path, p.engaged, demoDomain,
					name, referrer, s.utmSource, s.utmMedium, s.utmCampaign, country, c[0], c[1], c[2]})
			}
			visits++
			pageviews += count
		}
	}
	// A few people on the site right now, for the realtime counter.
	for i := range 3 {
		at := now.Add(-time.Duration(i+1) * time.Minute)
		var sessionID int64
		visitor := r.Int64()
		if err := tx.QueryRow(ctx, `
			INSERT INTO sessions (site_id, visitor_id, started_at, last_seen_at, pageviews, engaged_ms, entry_path, exit_path, hostname, source, country, browser, os, device)
			VALUES ($1, $2, $3, $3, 1, 0, '/', '/', $4, 'Direct', 'US', 'Chrome', 'macOS', 'Desktop') RETURNING id`,
			siteID, visitor, at, demoDomain).Scan(&sessionID); err != nil {
			fmt.Fprintln(os.Stderr, "seed:", err)
			os.Exit(1)
		}
		rows = append(rows, []any{siteID, sessionID, visitor, at, "pageview", "/", int64(0), demoDomain, "Direct", "", "", "", "", "US", "Chrome", "macOS", "Desktop"})
		visits++
		pageviews++
	}
	if _, err := tx.CopyFrom(ctx, pgx.Identifier{"events"},
		[]string{"site_id", "session_id", "visitor_id", "ts", "name", "path", "engaged_ms", "hostname", "source", "referrer",
			"utm_source", "utm_medium", "utm_campaign", "country", "browser", "os", "device"},
		pgx.CopyFromRows(rows)); err != nil {
		fmt.Fprintln(os.Stderr, "seed:", err)
		os.Exit(1)
	}
	if err := tx.Commit(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "seed:", err)
		os.Exit(1)
	}
	fmt.Printf("seed: site %s (%s): %d visits, %d pageviews over %d days\n", siteID, demoDomain, visits, pageviews, *days)
}
