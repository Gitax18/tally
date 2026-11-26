package stats

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Gitax18/tally/internal/filter"
	"github.com/Gitax18/tally/internal/testdb"
)

// The fixture. The site is in India (UTC+5:30); "now" is 27 Nov 2026, 12:00 UTC = 17:30 IST.
//
//	visit  visitor  starts (UTC)       IST date  pages (engaged ms)                      source       country device
//	A      1        27 Nov 03:00       27 Nov    / (30000), /docs (10000)                GitHub       IN      Desktop
//	B      2        27 Nov 05:00       27 Nov    / (5000)                                Google       US      Mobile
//	C      3        26 Nov 20:00       27 Nov    /pricing (20000), / (0), /docs (15000)  Direct       DE      Desktop
//	D      4        26 Nov 10:00       26 Nov    / (0)                                   Direct       IN      Desktop
//	E      5        26 Nov 18:00       26 Nov    /blog (10000), / (2000)                 Hacker News  US      Desktop
//
// C is on 26 Nov in UTC but 27 Nov in India, so it tests the time-zone bucketing; E is the
// last visit of 26 Nov in India (23:30 IST).
var (
	now  = time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)
	site = Site{ID: "site000001", Timezone: "Asia/Kolkata"}
)

type pv struct {
	path    string
	at      time.Time
	engaged int64
}

type dims struct{ source, referrer, country, browser, os, device string }

func utc(day, hour, minute int) time.Time {
	return time.Date(2026, 11, day, hour, minute, 0, 0, time.UTC)
}

func seed(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool := testdb.New(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `INSERT INTO sites (id, name, domain, timezone) VALUES ($1, 'Fixture', 'example.com', $2)`, site.ID, site.Timezone); err != nil {
		t.Fatal(err)
	}
	visit := func(visitor int64, d dims, pages ...pv) {
		var engaged int64
		for _, p := range pages {
			engaged += p.engaged
		}
		var id int64
		err := pool.QueryRow(ctx, `
			INSERT INTO sessions (site_id, visitor_id, started_at, last_seen_at, pageviews, engaged_ms, entry_path, exit_path,
			                      hostname, source, referrer, country, browser, os, device)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'example.com', $9, $10, $11, $12, $13, $14) RETURNING id`,
			site.ID, visitor, pages[0].at, pages[len(pages)-1].at, len(pages), engaged, pages[0].path, pages[len(pages)-1].path,
			d.source, d.referrer, d.country, d.browser, d.os, d.device).Scan(&id)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range pages {
			if _, err := pool.Exec(ctx, `
				INSERT INTO events (site_id, session_id, visitor_id, ts, name, path, engaged_ms, hostname, source, referrer, country, browser, os, device)
				VALUES ($1, $2, $3, $4, 'pageview', $5, $6, 'example.com', $7, $8, $9, $10, $11, $12)`,
				site.ID, id, visitor, p.at, p.path, p.engaged, d.source, d.referrer, d.country, d.browser, d.os, d.device); err != nil {
				t.Fatal(err)
			}
		}
	}
	visit(1, dims{"GitHub", "github.com", "IN", "Chrome", "macOS", "Desktop"},
		pv{"/", utc(27, 3, 0), 30000}, pv{"/docs", utc(27, 3, 2), 10000})
	visit(2, dims{"Google", "google.com", "US", "Safari", "iOS", "Mobile"},
		pv{"/", utc(27, 5, 0), 5000})
	visit(3, dims{"Direct", "", "DE", "Firefox", "Linux", "Desktop"},
		pv{"/pricing", utc(26, 20, 0), 20000}, pv{"/", utc(26, 20, 1), 0}, pv{"/docs", utc(26, 20, 2), 15000})
	visit(4, dims{"Direct", "", "IN", "Chrome", "macOS", "Desktop"},
		pv{"/", utc(26, 10, 0), 0})
	visit(5, dims{"Hacker News", "news.ycombinator.com/item", "US", "Chrome", "Windows", "Desktop"},
		pv{"/blog", utc(26, 18, 0), 10000}, pv{"/", utc(26, 18, 5), 2000})
	return pool
}

func period(t *testing.T, name string) Period {
	t.Helper()
	loc, _ := time.LoadLocation(site.Timezone)
	p, err := ParsePeriod(name, "", "", loc, now)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func filters(t *testing.T, raw string) filter.Filters {
	t.Helper()
	fs, err := filter.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return fs
}

func f(v float64) *float64 { return &v }

func eqPtr(a, b *float64) bool { return (a == nil && b == nil) || (a != nil && b != nil && *a == *b) }

func show(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

func TestSummary(t *testing.T) {
	svc := New(seed(t))
	ctx := context.Background()

	got, err := svc.Summary(ctx, site, period(t, "today"), nil, now)
	if err != nil {
		t.Fatal(err)
	}
	want := Metrics{Visitors: 3, Visits: 3, Pageviews: 6, ViewsPerVisit: 2, BounceRate: 33.3, VisitDuration: 26.7}
	if got.Current != want {
		t.Errorf("today\n got %+v\nwant %+v", got.Current, want)
	}
	// Compared with yesterday until 17:30 IST: only visit D (15:30 IST); E (23:30) is later.
	wantPrev := Metrics{Visitors: 1, Visits: 1, Pageviews: 1, ViewsPerVisit: 1, BounceRate: 100, VisitDuration: 0}
	if got.Previous != wantPrev {
		t.Errorf("today's comparison\n got %+v\nwant %+v", got.Previous, wantPrev)
	}
	c := got.Change
	for name, pair := range map[string][2]*float64{
		"visitors": {c.Visitors, f(200)}, "visits": {c.Visits, f(200)}, "pageviews": {c.Pageviews, f(500)},
		"views_per_visit": {c.ViewsPerVisit, f(100)}, "bounce_rate": {c.BounceRate, f(-66.7)}, "visit_duration": {c.VisitDuration, nil},
	} {
		if !eqPtr(pair[0], pair[1]) {
			t.Errorf("change in %s: got %v, want %v", name, show(pair[0]), show(pair[1]))
		}
	}

	got, err = svc.Summary(ctx, site, period(t, "yesterday"), nil, now)
	if err != nil {
		t.Fatal(err)
	}
	want = Metrics{Visitors: 2, Visits: 2, Pageviews: 3, ViewsPerVisit: 1.5, BounceRate: 50, VisitDuration: 6}
	if got.Current != want {
		t.Errorf("yesterday\n got %+v\nwant %+v", got.Current, want)
	}
	if got.Change.Visitors != nil || got.Change.BounceRate != nil {
		t.Error("with nothing the day before, there is no change to show")
	}
}

func TestSummaryWithFilters(t *testing.T) {
	svc := New(seed(t))
	ctx := context.Background()
	tests := []struct {
		filters string
		want    Metrics
	}{
		{"source:GitHub", Metrics{Visitors: 1, Visits: 1, Pageviews: 2, ViewsPerVisit: 2, BounceRate: 0, VisitDuration: 40}},
		// Pageviews of /docs; visits that include /docs (A and C).
		{"page:%2Fdocs", Metrics{Visitors: 2, Visits: 2, Pageviews: 2, ViewsPerVisit: 2.5, BounceRate: 0, VisitDuration: 37.5}},
		// Visits that entered on / (A and B), and all of their pageviews.
		{"entry_page:%2F", Metrics{Visitors: 2, Visits: 2, Pageviews: 3, ViewsPerVisit: 1.5, BounceRate: 50, VisitDuration: 22.5}},
		{"country:IN,device:Desktop", Metrics{Visitors: 1, Visits: 1, Pageviews: 2, ViewsPerVisit: 2, BounceRate: 0, VisitDuration: 40}},
		// An empty value matches visits without a referrer.
		{"referrer:", Metrics{Visitors: 1, Visits: 1, Pageviews: 3, ViewsPerVisit: 3, BounceRate: 0, VisitDuration: 35}},
		{"country:FR", Metrics{}},
	}
	for _, tt := range tests {
		got, err := svc.Summary(ctx, site, period(t, "today"), filters(t, tt.filters), now)
		if err != nil {
			t.Fatalf("%s: %v", tt.filters, err)
		}
		if got.Current != tt.want {
			t.Errorf("%s\n got %+v\nwant %+v", tt.filters, got.Current, tt.want)
		}
	}
}

func TestBreakdown(t *testing.T) {
	svc := New(seed(t))
	ctx := context.Background()
	today := period(t, "today")
	tests := []struct {
		dim, filters, search string
		limit                int
		want                 []Row
		more                 bool
	}{
		{"source", "", "", 10, []Row{
			{Value: "Direct", Visitors: 1, Pageviews: 3},
			{Value: "GitHub", Visitors: 1, Pageviews: 2},
			{Value: "Google", Visitors: 1, Pageviews: 1},
		}, false},
		{"page", "", "", 10, []Row{
			{Value: "/", Visitors: 3, Pageviews: 3, TimeOnPageMS: 17500},
			{Value: "/docs", Visitors: 2, Pageviews: 2, TimeOnPageMS: 12500},
			{Value: "/pricing", Visitors: 1, Pageviews: 1, TimeOnPageMS: 20000},
		}, false},
		{"entry_page", "", "", 10, []Row{{Value: "/", Visitors: 2, Visits: 2}, {Value: "/pricing", Visitors: 1, Visits: 1}}, false},
		{"exit_page", "", "", 10, []Row{{Value: "/docs", Visitors: 2, Visits: 2}, {Value: "/", Visitors: 1, Visits: 1}}, false},
		{"country", "", "", 10, []Row{
			{Value: "DE", Visitors: 1, Pageviews: 3}, {Value: "IN", Visitors: 1, Pageviews: 2}, {Value: "US", Visitors: 1, Pageviews: 1},
		}, false},
		{"device", "", "", 10, []Row{{Value: "Desktop", Visitors: 2, Pageviews: 5}, {Value: "Mobile", Visitors: 1, Pageviews: 1}}, false},
		{"page", "", "DOC", 10, []Row{{Value: "/docs", Visitors: 2, Pageviews: 2, TimeOnPageMS: 12500}}, false},
		{"page", "", "%", 10, []Row{}, false},
		{"page", "", "", 1, []Row{{Value: "/", Visitors: 3, Pageviews: 3, TimeOnPageMS: 17500}}, true},
		{"page", "source:GitHub", "", 10, []Row{
			{Value: "/", Visitors: 1, Pageviews: 1, TimeOnPageMS: 30000},
			{Value: "/docs", Visitors: 1, Pageviews: 1, TimeOnPageMS: 10000},
		}, false},
	}
	for _, tt := range tests {
		dim, _ := filter.Lookup(tt.dim)
		got, err := svc.Breakdown(ctx, site, today, dim, filters(t, tt.filters), tt.limit, tt.search)
		if err != nil {
			t.Fatalf("%s: %v", tt.dim, err)
		}
		if len(got.Rows) != len(tt.want) || got.More != tt.more {
			t.Errorf("%s %s %q: got %+v (more %v)", tt.dim, tt.filters, tt.search, got.Rows, got.More)
			continue
		}
		for i := range tt.want {
			if got.Rows[i] != tt.want[i] {
				t.Errorf("%s %s %q row %d\n got %+v\nwant %+v", tt.dim, tt.filters, tt.search, i, got.Rows[i], tt.want[i])
			}
		}
	}
}

func TestTimeseries(t *testing.T) {
	svc := New(seed(t))
	ctx := context.Background()

	today := period(t, "today")
	ts, err := svc.Timeseries(ctx, site, today, Hour, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(ts.Current) != 24 || len(ts.Previous) != 24 {
		t.Fatalf("want 24 hourly buckets each, got %d and %d", len(ts.Current), len(ts.Previous))
	}
	byBucket := map[string]Point{}
	for _, p := range append(ts.Current, ts.Previous...) {
		byBucket[p.Bucket] = p
	}
	checks := map[string]Metrics{
		"2026-11-27T01:00": {Visitors: 1, Visits: 1, Pageviews: 3, ViewsPerVisit: 3, VisitDuration: 35},                 // C, 01:30 IST
		"2026-11-27T08:00": {Visitors: 1, Visits: 1, Pageviews: 2, ViewsPerVisit: 2, VisitDuration: 40},                 // A
		"2026-11-27T10:00": {Visitors: 1, Visits: 1, Pageviews: 1, ViewsPerVisit: 1, BounceRate: 100, VisitDuration: 5}, // B
		"2026-11-27T00:00": {},
		"2026-11-26T15:00": {Visitors: 1, Visits: 1, Pageviews: 1, ViewsPerVisit: 1, BounceRate: 100},   // D
		"2026-11-26T23:00": {Visitors: 1, Visits: 1, Pageviews: 2, ViewsPerVisit: 2, VisitDuration: 12}, // E
	}
	for bucket, want := range checks {
		if got := byBucket[bucket]; got.Metrics != want {
			t.Errorf("%s\n got %+v\nwant %+v", bucket, got.Metrics, want)
		}
	}
	var sum int64
	for _, p := range ts.Current {
		sum += p.Pageviews
	}
	if sum != 6 {
		t.Errorf("today's hourly pageviews add up to %d, want 6", sum)
	}

	week, err := svc.Timeseries(ctx, site, period(t, "7d"), Day, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(week.Current) != 7 || week.Current[0].Bucket != "2026-11-21" || week.Current[6].Bucket != "2026-11-27" {
		t.Fatalf("7d buckets: %+v", week.Current)
	}
	if d := week.Current[5]; d.Bucket != "2026-11-26" || d.Visitors != 2 || d.Pageviews != 3 || d.Visits != 2 {
		t.Errorf("26 Nov (IST): %+v", d)
	}
	if d := week.Current[6]; d.Visitors != 3 || d.Pageviews != 6 || d.Visits != 3 {
		t.Errorf("27 Nov (IST) must include visit C, which is 26 Nov in UTC: %+v", d)
	}

	filtered, err := svc.Timeseries(ctx, site, period(t, "7d"), Day, filters(t, "country:US"))
	if err != nil {
		t.Fatal(err)
	}
	if filtered.Current[5].Visitors != 1 || filtered.Current[6].Visitors != 1 || filtered.Current[6].Pageviews != 1 {
		t.Errorf("filtered: %+v %+v", filtered.Current[5], filtered.Current[6])
	}
}

func TestRealtimeAndOverview(t *testing.T) {
	pool := seed(t)
	svc := New(pool)
	ctx := context.Background()

	n, err := svc.Realtime(ctx, site, now)
	if err != nil || n != 0 {
		t.Fatalf("nobody was seen in the last five minutes: %d %v", n, err)
	}
	for _, seen := range []time.Duration{2 * time.Minute, 6 * time.Minute} {
		if _, err := pool.Exec(ctx, `INSERT INTO sessions (site_id, visitor_id, started_at, last_seen_at, pageviews, entry_path, exit_path)
			VALUES ($1, $2, $3, $4, 1, '/', '/')`, site.ID, int64(seen), now.Add(-10*time.Minute), now.Add(-seen)); err != nil {
			t.Fatal(err)
		}
	}
	if n, _ := svc.Realtime(ctx, site, now); n != 1 {
		t.Fatalf("want 1 current visitor (seen 2 minutes ago, not 6), got %d", n)
	}

	visitors, spark, err := svc.Overview(ctx, site, now)
	if err != nil {
		t.Fatal(err)
	}
	// Since 26 Nov 12:00 UTC: visits E, C, A, B.
	if visitors != 4 {
		t.Errorf("24h visitors: %d, want 4", visitors)
	}
	want := []int64{0, 0, 0, 0, 0, 2, 3}
	for i := range want {
		if len(spark) != 7 || spark[i] != want[i] {
			t.Fatalf("sparkline %v, want %v", spark, want)
		}
	}
}
