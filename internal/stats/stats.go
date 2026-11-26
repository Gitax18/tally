// Package stats answers the dashboard's questions with plain SQL on the events and sessions
// tables. Every grouping by time uses the site's time zone.
//
// Metric definitions (see docs/metrics.md):
//   - visitors:       distinct visitor IDs. IDs change daily, so a person who comes back on
//     another day counts again, the same trade-off Plausible makes.
//   - visits:         sessions that started in the range.
//   - pageviews:      pageview events in the range.
//   - views_per_visit, bounce_rate (visits with exactly one pageview, in %), visit_duration
//     (average engaged seconds): over those visits.
package stats

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Gitax18/tally/internal/filter"
)

// Service runs the stats queries.
type Service struct {
	pool *pgxpool.Pool
}

// New returns a Service.
func New(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

// Site is what the queries need to know about a site.
type Site struct {
	ID       string
	Timezone string
}

// Metrics are the six headline numbers.
type Metrics struct {
	Visitors      int64   `json:"visitors"`
	Visits        int64   `json:"visits"`
	Pageviews     int64   `json:"pageviews"`
	ViewsPerVisit float64 `json:"views_per_visit"`
	BounceRate    float64 `json:"bounce_rate"`
	VisitDuration float64 `json:"visit_duration"`
}

// Change is each metric's change against the previous period: percent for counts and averages,
// percentage points for the bounce rate. Nil when there is nothing to compare with.
type Change struct {
	Visitors      *float64 `json:"visitors"`
	Visits        *float64 `json:"visits"`
	Pageviews     *float64 `json:"pageviews"`
	ViewsPerVisit *float64 `json:"views_per_visit"`
	BounceRate    *float64 `json:"bounce_rate"`
	VisitDuration *float64 `json:"visit_duration"`
}

// SummaryResult is the summary of a period and its comparison.
type SummaryResult struct {
	Current  Metrics `json:"current"`
	Previous Metrics `json:"previous"`
	Change   Change  `json:"change"`
}

// Summary returns the headline metrics for the period and the previous period.
func (s *Service) Summary(ctx context.Context, site Site, p Period, fs filter.Filters, now time.Time) (SummaryResult, error) {
	cur, err := s.metrics(ctx, site, p.Current, fs)
	if err != nil {
		return SummaryResult{}, err
	}
	prev, err := s.metrics(ctx, site, p.SummaryPrevious(now), fs)
	if err != nil {
		return SummaryResult{}, err
	}
	return SummaryResult{
		Current:  cur,
		Previous: prev,
		Change: Change{
			Visitors:      pct(float64(cur.Visitors), float64(prev.Visitors)),
			Visits:        pct(float64(cur.Visits), float64(prev.Visits)),
			Pageviews:     pct(float64(cur.Pageviews), float64(prev.Pageviews)),
			ViewsPerVisit: pct(cur.ViewsPerVisit, prev.ViewsPerVisit),
			BounceRate:    points(cur.BounceRate, prev.BounceRate, cur.Visits, prev.Visits),
			VisitDuration: pct(cur.VisitDuration, prev.VisitDuration),
		},
	}, nil
}

func (s *Service) metrics(ctx context.Context, site Site, r Range, fs filter.Filters) (Metrics, error) {
	var m Metrics

	a := &filter.Args{}
	sc := filter.Scope{Site: a.Add(site.ID), From: a.Add(r.From), To: a.Add(r.To)}
	sql := `SELECT count(DISTINCT visitor_id), count(*) FROM events
		WHERE site_id = ` + sc.Site + ` AND name = 'pageview' AND ts >= ` + sc.From + ` AND ts < ` + sc.To + fs.Events(a, sc)
	if err := s.pool.QueryRow(ctx, sql, a.Values()...).Scan(&m.Visitors, &m.Pageviews); err != nil {
		return m, fmt.Errorf("summary events: %w", err)
	}

	a = &filter.Args{}
	sc = filter.Scope{Site: a.Add(site.ID), From: a.Add(r.From), To: a.Add(r.To)}
	var bounces int64
	var avgPages, avgEngaged float64
	sql = `SELECT count(*), count(*) FILTER (WHERE pageviews = 1),
			coalesce(avg(pageviews), 0)::float8, coalesce(avg(engaged_ms), 0)::float8
		FROM sessions
		WHERE site_id = ` + sc.Site + ` AND started_at >= ` + sc.From + ` AND started_at < ` + sc.To + fs.Sessions(a, sc)
	if err := s.pool.QueryRow(ctx, sql, a.Values()...).Scan(&m.Visits, &bounces, &avgPages, &avgEngaged); err != nil {
		return m, fmt.Errorf("summary sessions: %w", err)
	}
	m.ViewsPerVisit = round(avgPages, 2)
	m.VisitDuration = round(avgEngaged/1000, 1)
	if m.Visits > 0 {
		m.BounceRate = round(100*float64(bounces)/float64(m.Visits), 1)
	}
	return m, nil
}

// Point is one timeseries bucket.
type Point struct {
	Bucket string `json:"bucket"`
	Metrics
}

// TimeseriesResult is the chart: the current period and the previous one, bucket by bucket.
type TimeseriesResult struct {
	Interval string  `json:"interval"`
	Current  []Point `json:"current"`
	Previous []Point `json:"previous"`
}

// Timeseries returns every metric per bucket, with empty buckets filled with zeros.
func (s *Service) Timeseries(ctx context.Context, site Site, p Period, interval string, fs filter.Filters) (TimeseriesResult, error) {
	cur, err := s.series(ctx, site, p.Current, interval, p.Loc, fs)
	if err != nil {
		return TimeseriesResult{}, err
	}
	prev, err := s.series(ctx, site, p.Previous, interval, p.Loc, fs)
	if err != nil {
		return TimeseriesResult{}, err
	}
	return TimeseriesResult{Interval: interval, Current: cur, Previous: prev}, nil
}

func (s *Service) series(ctx context.Context, site Site, r Range, interval string, loc *time.Location, fs filter.Filters) ([]Point, error) {
	keys := Buckets(r, interval, loc)
	byKey := make(map[string]*Point, len(keys))
	points := make([]Point, len(keys))
	for i, k := range keys {
		points[i].Bucket = k
		byKey[k] = &points[i]
	}
	layout := bucketLayout(interval)

	a := &filter.Args{}
	sc := filter.Scope{Site: a.Add(site.ID), From: a.Add(r.From), To: a.Add(r.To)}
	unit, tz := a.Add(interval), a.Add(site.Timezone)
	sql := `SELECT date_trunc(` + unit + `::text, ts AT TIME ZONE ` + tz + `::text) AS b,
			count(DISTINCT visitor_id), count(*)
		FROM events
		WHERE site_id = ` + sc.Site + ` AND name = 'pageview' AND ts >= ` + sc.From + ` AND ts < ` + sc.To + fs.Events(a, sc) + `
		GROUP BY 1`
	rows, err := s.pool.Query(ctx, sql, a.Values()...)
	if err != nil {
		return nil, fmt.Errorf("timeseries events: %w", err)
	}
	for rows.Next() {
		var b time.Time
		var visitors, pageviews int64
		if err := rows.Scan(&b, &visitors, &pageviews); err != nil {
			rows.Close()
			return nil, err
		}
		if p := byKey[b.Format(layout)]; p != nil {
			p.Visitors, p.Pageviews = visitors, pageviews
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	a = &filter.Args{}
	sc = filter.Scope{Site: a.Add(site.ID), From: a.Add(r.From), To: a.Add(r.To)}
	unit, tz = a.Add(interval), a.Add(site.Timezone)
	sql = `SELECT date_trunc(` + unit + `::text, started_at AT TIME ZONE ` + tz + `::text) AS b,
			count(*), count(*) FILTER (WHERE pageviews = 1),
			avg(pageviews)::float8, avg(engaged_ms)::float8
		FROM sessions
		WHERE site_id = ` + sc.Site + ` AND started_at >= ` + sc.From + ` AND started_at < ` + sc.To + fs.Sessions(a, sc) + `
		GROUP BY 1`
	rows, err = s.pool.Query(ctx, sql, a.Values()...)
	if err != nil {
		return nil, fmt.Errorf("timeseries sessions: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var b time.Time
		var visits, bounces int64
		var avgPages, avgEngaged float64
		if err := rows.Scan(&b, &visits, &bounces, &avgPages, &avgEngaged); err != nil {
			return nil, err
		}
		if p := byKey[b.Format(layout)]; p != nil {
			p.Visits = visits
			p.ViewsPerVisit = round(avgPages, 2)
			p.VisitDuration = round(avgEngaged/1000, 1)
			p.BounceRate = round(100*float64(bounces)/float64(visits), 1)
		}
	}
	return points, rows.Err()
}

// Row is one breakdown row. Which counts are filled depends on the dimension: pages carry
// pageviews and time on page, entry and exit pages carry visits.
type Row struct {
	Value        string  `json:"value"`
	Visitors     int64   `json:"visitors"`
	Pageviews    int64   `json:"pageviews,omitempty"`
	Visits       int64   `json:"visits,omitempty"`
	TimeOnPageMS float64 `json:"time_on_page_ms,omitempty"`
}

// BreakdownResult is a ranked list of a dimension's values.
type BreakdownResult struct {
	Dimension string `json:"dimension"`
	Rows      []Row  `json:"rows"`
	More      bool   `json:"more"`
}

// MaxBreakdownLimit bounds one breakdown request.
const MaxBreakdownLimit = 1000

// Breakdown ranks a dimension's values by visitors. search, when set, keeps values containing it
// (case-insensitive).
func (s *Service) Breakdown(ctx context.Context, site Site, p Period, dim filter.Dimension, fs filter.Filters, limit int, search string) (BreakdownResult, error) {
	limit = max(1, min(limit, MaxBreakdownLimit))
	r := p.Current
	a := &filter.Args{}
	sc := filter.Scope{Site: a.Add(site.ID), From: a.Add(r.From), To: a.Add(r.To)}
	col := dim.Column // from the whitelist, never user input

	var sql string
	if dim.Level == filter.VisitOnly {
		sql = `SELECT ` + col + `, count(DISTINCT visitor_id), 0::bigint, count(*), 0::float8
			FROM sessions
			WHERE site_id = ` + sc.Site + ` AND started_at >= ` + sc.From + ` AND started_at < ` + sc.To + fs.Sessions(a, sc)
	} else {
		timeOnPage := `0::float8`
		if dim.Level == filter.Page {
			// Pageviews whose engagement ping never arrived would read as 0 s and drag the
			// average down, so only pageviews with engaged time count.
			timeOnPage = `coalesce(avg(engaged_ms) FILTER (WHERE engaged_ms > 0), 0)::float8`
		}
		sql = `SELECT ` + col + `, count(DISTINCT visitor_id), count(*), 0::bigint, ` + timeOnPage + `
			FROM events
			WHERE site_id = ` + sc.Site + ` AND name = 'pageview' AND ts >= ` + sc.From + ` AND ts < ` + sc.To + fs.Events(a, sc)
	}
	if search = strings.TrimSpace(search); search != "" {
		sql += ` AND ` + col + ` ILIKE ` + a.Add("%"+escapeLike(search)+"%")
	}
	sql += ` GROUP BY 1 ORDER BY 2 DESC, 3 DESC, 4 DESC, 1 LIMIT ` + a.Add(limit+1)

	rows, err := s.pool.Query(ctx, sql, a.Values()...)
	if err != nil {
		return BreakdownResult{}, fmt.Errorf("breakdown: %w", err)
	}
	defer rows.Close()
	out := BreakdownResult{Dimension: dim.Name, Rows: []Row{}}
	for rows.Next() {
		var row Row
		if err := rows.Scan(&row.Value, &row.Visitors, &row.Pageviews, &row.Visits, &row.TimeOnPageMS); err != nil {
			return BreakdownResult{}, err
		}
		row.TimeOnPageMS = math.Round(row.TimeOnPageMS)
		out.Rows = append(out.Rows, row)
	}
	if len(out.Rows) > limit {
		out.Rows, out.More = out.Rows[:limit], true
	}
	return out, rows.Err()
}

// RealtimeWindow is how recently a visitor must have been seen to count as current.
const RealtimeWindow = 5 * time.Minute

// Realtime counts visitors seen in the last five minutes.
func (s *Service) Realtime(ctx context.Context, site Site, now time.Time) (int64, error) {
	var n int64
	// started_at bounds the scan to recent visits through the (site_id, started_at) index.
	err := s.pool.QueryRow(ctx, `
		SELECT count(DISTINCT visitor_id) FROM sessions
		WHERE site_id = $1 AND started_at >= $2 AND last_seen_at >= $3`,
		site.ID, now.Add(-24*time.Hour), now.Add(-RealtimeWindow)).Scan(&n)
	return n, err
}

// Overview is a site's row in the sites list: visitors in the last 24 hours and daily visitors
// for the last 7 days (oldest first, today last).
func (s *Service) Overview(ctx context.Context, site Site, now time.Time) (int64, []int64, error) {
	var visitors24h int64
	if err := s.pool.QueryRow(ctx, `
		SELECT count(DISTINCT visitor_id) FROM events
		WHERE site_id = $1 AND name = 'pageview' AND ts >= $2`,
		site.ID, now.Add(-24*time.Hour)).Scan(&visitors24h); err != nil {
		return 0, nil, err
	}
	loc, err := time.LoadLocation(site.Timezone)
	if err != nil {
		loc = time.UTC
	}
	local := now.In(loc)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	r := Range{today.AddDate(0, 0, -6), today.AddDate(0, 0, 1)}
	points, err := s.seriesVisitors(ctx, site, r, loc)
	if err != nil {
		return 0, nil, err
	}
	return visitors24h, points, nil
}

func (s *Service) seriesVisitors(ctx context.Context, site Site, r Range, loc *time.Location) ([]int64, error) {
	keys := Buckets(r, Day, loc)
	index := make(map[string]int, len(keys))
	for i, k := range keys {
		index[k] = i
	}
	out := make([]int64, len(keys))
	rows, err := s.pool.Query(ctx, `
		SELECT date_trunc('day', ts AT TIME ZONE $2::text), count(DISTINCT visitor_id)
		FROM events
		WHERE site_id = $1 AND name = 'pageview' AND ts >= $3 AND ts < $4
		GROUP BY 1`, site.ID, site.Timezone, r.From, r.To)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var b time.Time
		var n int64
		if err := rows.Scan(&b, &n); err != nil {
			return nil, err
		}
		if i, ok := index[b.Format(dateLayout)]; ok {
			out[i] = n
		}
	}
	return out, rows.Err()
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func pct(cur, prev float64) *float64 {
	if prev == 0 {
		return nil
	}
	v := round(100*(cur-prev)/prev, 1)
	return &v
}

func points(cur, prev float64, curVisits, prevVisits int64) *float64 {
	if curVisits == 0 || prevVisits == 0 {
		return nil
	}
	v := round(cur-prev, 1)
	return &v
}

func round(v float64, places int) float64 {
	p := math.Pow(10, float64(places))
	return math.Round(v*p) / p
}
