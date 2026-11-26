package stats

import (
	"errors"
	"fmt"
	"time"
)

// Intervals for timeseries buckets.
const (
	Hour  = "hour"
	Day   = "day"
	Month = "month"
)

// Range is [From, To) in absolute time.
type Range struct {
	From time.Time
	To   time.Time
}

// Period is a dashboard period resolved in the site's time zone, with the period it is compared
// against.
type Period struct {
	Name     string
	Current  Range
	Previous Range
	Interval string // the default chart interval
	Loc      *time.Location
}

const (
	maxCustomDays = 3 * 366
	dateLayout    = "2006-01-02"
)

// Periods the dashboard offers.
var Periods = []string{"today", "yesterday", "7d", "30d", "month", "last_month", "12mo", "custom"}

// ParsePeriod resolves a period name (and, for "custom", inclusive from/to dates as YYYY-MM-DD)
// at time now in loc.
func ParsePeriod(name, from, to string, loc *time.Location, now time.Time) (Period, error) {
	now = now.In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	thisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
	days := func(start time.Time, n int) Range { return Range{start, start.AddDate(0, 0, n)} }
	months := func(start time.Time, n int) Range { return Range{start, start.AddDate(0, n, 0)} }

	p := Period{Name: name, Loc: loc}
	switch name {
	case "", "30d":
		p.Name = "30d"
		p.Current = days(today.AddDate(0, 0, -29), 30)
		p.Previous = days(today.AddDate(0, 0, -59), 30)
	case "today":
		p.Current = days(today, 1)
		p.Previous = days(today.AddDate(0, 0, -1), 1)
	case "yesterday":
		p.Current = days(today.AddDate(0, 0, -1), 1)
		p.Previous = days(today.AddDate(0, 0, -2), 1)
	case "7d":
		p.Current = days(today.AddDate(0, 0, -6), 7)
		p.Previous = days(today.AddDate(0, 0, -13), 7)
	case "month":
		p.Current = months(thisMonth, 1)
		p.Previous = months(thisMonth.AddDate(0, -1, 0), 1)
	case "last_month":
		p.Current = months(thisMonth.AddDate(0, -1, 0), 1)
		p.Previous = months(thisMonth.AddDate(0, -2, 0), 1)
	case "12mo":
		p.Current = months(thisMonth.AddDate(0, -11, 0), 12)
		p.Previous = months(thisMonth.AddDate(0, -23, 0), 12)
	case "custom":
		start, err1 := time.ParseInLocation(dateLayout, from, loc)
		end, err2 := time.ParseInLocation(dateLayout, to, loc)
		if err1 != nil || err2 != nil {
			return Period{}, errors.New("a custom period needs from and to as YYYY-MM-DD")
		}
		if end.Before(start) {
			return Period{}, errors.New("from must not be after to")
		}
		n := int(end.Sub(start).Hours()/24+0.5) + 1 // inclusive; rounding absorbs DST hours
		if n > maxCustomDays {
			return Period{}, fmt.Errorf("a custom period can be at most %d days", maxCustomDays)
		}
		p.Current = days(start, n)
		p.Previous = days(start.AddDate(0, 0, -n), n)
	default:
		return Period{}, fmt.Errorf("unknown period %q", name)
	}
	p.Interval = DefaultInterval(p.Current, loc)
	return p, nil
}

// DefaultInterval is hourly for up to two days, daily up to about three months, monthly beyond.
func DefaultInterval(r Range, loc *time.Location) string {
	n := dayCount(r, loc)
	switch {
	case n <= 2:
		return Hour
	case n <= 93:
		return Day
	}
	return Month
}

// CheckInterval refuses intervals that would make an absurd number of buckets.
func CheckInterval(interval string, r Range, loc *time.Location) error {
	n := dayCount(r, loc)
	switch interval {
	case Hour:
		if n > 7 {
			return errors.New("hourly charts are limited to 7 days")
		}
	case Day:
		if n > 400 {
			return errors.New("daily charts are limited to 400 days")
		}
	case Month:
	default:
		return fmt.Errorf("unknown interval %q (use hour, day or month)", interval)
	}
	return nil
}

func dayCount(r Range, loc *time.Location) int {
	from, to := r.From.In(loc), r.To.In(loc)
	return int(to.Sub(from).Hours()/24 + 0.5)
}

// SummaryPrevious is the range a summary compares against. While the current period is still
// running, the previous one is cut to the same elapsed time, so "today until 2pm" is compared
// with "yesterday until 2pm", not with all of yesterday.
func (p Period) SummaryPrevious(now time.Time) Range {
	prev := p.Previous
	if now.After(p.Current.From) && now.Before(p.Current.To) {
		if cut := prev.From.Add(now.Sub(p.Current.From)); cut.Before(prev.To) {
			prev.To = cut
		}
	}
	return prev
}

// Buckets returns the bucket keys of r at interval in loc, in order. Keys are local wall-clock
// times formatted like the SQL side (see bucketLayout), so the two can be matched.
func Buckets(r Range, interval string, loc *time.Location) []string {
	var keys []string
	seen := map[string]bool{}
	add := func(t time.Time) {
		k := t.Format(bucketLayout(interval))
		if !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}
	from := r.From.In(loc)
	switch interval {
	case Hour:
		// Step in absolute time: across a DST change a local hour can repeat (deduplicated
		// above) or be skipped, exactly as in the data.
		// Start on the local hour (Truncate would use absolute time, which is :30 in India).
		start := time.Date(from.Year(), from.Month(), from.Day(), from.Hour(), 0, 0, 0, loc)
		for t := start; t.Before(r.To); t = t.Add(time.Hour) {
			add(t.In(loc))
		}
	case Day:
		for t := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, loc); t.Before(r.To); t = t.AddDate(0, 0, 1) {
			add(t)
		}
	case Month:
		for t := time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, loc); t.Before(r.To); t = t.AddDate(0, 1, 0) {
			add(t)
		}
	}
	return keys
}

func bucketLayout(interval string) string {
	switch interval {
	case Hour:
		return "2006-01-02T15:00"
	case Month:
		return "2006-01"
	}
	return dateLayout
}
