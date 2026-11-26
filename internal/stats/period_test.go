package stats

import (
	"testing"
	"time"
)

func mustLoc(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func TestParsePeriod(t *testing.T) {
	ist := mustLoc(t, "Asia/Kolkata")
	// 17:30 on 27 Nov in India, 12:00 UTC.
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)
	d := func(y int, m time.Month, day int) time.Time { return time.Date(y, m, day, 0, 0, 0, 0, ist) }

	tests := []struct {
		name, from, to string
		cur, prev      Range
		interval       string
	}{
		{"today", "", "", Range{d(2026, 11, 27), d(2026, 11, 28)}, Range{d(2026, 11, 26), d(2026, 11, 27)}, Hour},
		{"yesterday", "", "", Range{d(2026, 11, 26), d(2026, 11, 27)}, Range{d(2026, 11, 25), d(2026, 11, 26)}, Hour},
		{"7d", "", "", Range{d(2026, 11, 21), d(2026, 11, 28)}, Range{d(2026, 11, 14), d(2026, 11, 21)}, Day},
		{"30d", "", "", Range{d(2026, 10, 29), d(2026, 11, 28)}, Range{d(2026, 9, 29), d(2026, 10, 29)}, Day},
		{"month", "", "", Range{d(2026, 11, 1), d(2026, 12, 1)}, Range{d(2026, 10, 1), d(2026, 11, 1)}, Day},
		{"last_month", "", "", Range{d(2026, 10, 1), d(2026, 11, 1)}, Range{d(2026, 9, 1), d(2026, 10, 1)}, Day},
		{"12mo", "", "", Range{d(2025, 12, 1), d(2026, 12, 1)}, Range{d(2024, 12, 1), d(2025, 12, 1)}, Month},
		{"custom", "2026-11-01", "2026-11-10", Range{d(2026, 11, 1), d(2026, 11, 11)}, Range{d(2026, 10, 22), d(2026, 11, 1)}, Day},
		{"custom", "2026-11-05", "2026-11-05", Range{d(2026, 11, 5), d(2026, 11, 6)}, Range{d(2026, 11, 4), d(2026, 11, 5)}, Hour},
	}
	for _, tt := range tests {
		p, err := ParsePeriod(tt.name, tt.from, tt.to, ist, now)
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		if !p.Current.From.Equal(tt.cur.From) || !p.Current.To.Equal(tt.cur.To) {
			t.Errorf("%s current: %v – %v", tt.name, p.Current.From, p.Current.To)
		}
		if !p.Previous.From.Equal(tt.prev.From) || !p.Previous.To.Equal(tt.prev.To) {
			t.Errorf("%s previous: %v – %v", tt.name, p.Previous.From, p.Previous.To)
		}
		if p.Interval != tt.interval {
			t.Errorf("%s interval %s, want %s", tt.name, p.Interval, tt.interval)
		}
	}

	for _, bad := range [][3]string{{"week", "", ""}, {"custom", "2026-11-10", "2026-11-01"}, {"custom", "nope", "2026-11-01"}, {"custom", "2020-01-01", "2026-01-01"}} {
		if _, err := ParsePeriod(bad[0], bad[1], bad[2], ist, now); err == nil {
			t.Errorf("ParsePeriod%v should fail", bad)
		}
	}
}

func TestSummaryPreviousIsCutToElapsedTime(t *testing.T) {
	ist := mustLoc(t, "Asia/Kolkata")
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC) // 17:30 IST
	p, _ := ParsePeriod("today", "", "", ist, now)
	prev := p.SummaryPrevious(now)
	want := time.Date(2026, 11, 26, 17, 30, 0, 0, ist)
	if !prev.To.Equal(want) {
		t.Fatalf("previous should end at %v, got %v", want, prev.To)
	}
	y, _ := ParsePeriod("yesterday", "", "", ist, now)
	if got := y.SummaryPrevious(now); !got.To.Equal(y.Previous.To) {
		t.Fatal("a finished period compares with the whole previous one")
	}
}

func TestBucketsAcrossDST(t *testing.T) {
	ny := mustLoc(t, "America/New_York")
	// DST ends on 1 Nov 2026 at 02:00 local: that day has 25 hours.
	day := Range{time.Date(2026, 11, 1, 0, 0, 0, 0, ny), time.Date(2026, 11, 2, 0, 0, 0, 0, ny)}
	hours := Buckets(day, Hour, ny)
	if len(hours) != 24 || hours[0] != "2026-11-01T00:00" || hours[23] != "2026-11-01T23:00" {
		t.Fatalf("hour buckets: %d %v", len(hours), hours)
	}
	week := Range{time.Date(2026, 10, 29, 0, 0, 0, 0, ny), time.Date(2026, 11, 5, 0, 0, 0, 0, ny)}
	days := Buckets(week, Day, ny)
	want := []string{"2026-10-29", "2026-10-30", "2026-10-31", "2026-11-01", "2026-11-02", "2026-11-03", "2026-11-04"}
	if len(days) != len(want) {
		t.Fatalf("day buckets: %v", days)
	}
	for i := range want {
		if days[i] != want[i] {
			t.Fatalf("day buckets: %v", days)
		}
	}
	year := Range{time.Date(2025, 12, 1, 0, 0, 0, 0, ny), time.Date(2026, 12, 1, 0, 0, 0, 0, ny)}
	if m := Buckets(year, Month, ny); len(m) != 12 || m[0] != "2025-12" || m[11] != "2026-11" {
		t.Fatalf("month buckets: %v", m)
	}
}

func TestCheckInterval(t *testing.T) {
	ist := mustLoc(t, "Asia/Kolkata")
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)
	p, _ := ParsePeriod("30d", "", "", ist, now)
	if CheckInterval(Hour, p.Current, ist) == nil {
		t.Error("30 days of hours should be refused")
	}
	if CheckInterval(Day, p.Current, ist) != nil || CheckInterval(Month, p.Current, ist) != nil {
		t.Error("day and month should be allowed")
	}
	if CheckInterval("week", p.Current, ist) == nil {
		t.Error("unknown interval")
	}
}
