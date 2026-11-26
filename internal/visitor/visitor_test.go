package visitor

import (
	"bytes"
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/Gitax18/tally/internal/testdb"
)

var (
	ip = netip.MustParseAddr("198.51.100.7")
	ua = "Mozilla/5.0 (Macintosh) Safari/605.1.15"
)

func TestHashIsStableForTheSameInputs(t *testing.T) {
	first := Hash(bytes.Repeat([]byte{1}, 32), "site000001", ip, ua)
	second := Hash(bytes.Repeat([]byte{1}, 32), "site000001", ip, ua)
	if first != second {
		t.Fatal("same inputs gave different IDs")
	}
}

func TestHashChangesWithEachInput(t *testing.T) {
	salt := bytes.Repeat([]byte{1}, 32)
	base := Hash(salt, "site000001", ip, ua)
	for name, other := range map[string]int64{
		"salt": Hash(bytes.Repeat([]byte{2}, 32), "site000001", ip, ua),
		"site": Hash(salt, "site000002", ip, ua),
		"ip":   Hash(salt, "site000001", netip.MustParseAddr("198.51.100.8"), ua),
		"ua":   Hash(salt, "site000001", ip, ua+"x"),
	} {
		if other == base {
			t.Errorf("changing the %s did not change the ID", name)
		}
	}
}

func TestDailySaltStableWithinADayAndChangesAcrossDays(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	s := NewSalts(pool)

	morning := time.Date(2026, 11, 27, 0, 5, 0, 0, time.UTC)
	evening := time.Date(2026, 11, 27, 23, 55, 0, 0, time.UTC)
	nextDay := time.Date(2026, 11, 28, 0, 5, 0, 0, time.UTC)

	a, prevA, err := s.Today(ctx, morning)
	if err != nil {
		t.Fatal(err)
	}
	if prevA != nil {
		t.Fatal("no salt existed for yesterday, so none should be returned")
	}
	b, _, _ := s.Today(ctx, evening)
	// A second process sees the same salt (read from the table, not its own memory).
	c, _, _ := NewSalts(pool).Today(ctx, evening)
	if Hash(a, "s", ip, ua) != Hash(b, "s", ip, ua) || Hash(a, "s", ip, ua) != Hash(c, "s", ip, ua) {
		t.Fatal("the visitor ID changed within one day")
	}

	d, prevD, _ := s.Today(ctx, nextDay)
	if Hash(a, "s", ip, ua) == Hash(d, "s", ip, ua) {
		t.Fatal("the visitor ID did not change across days")
	}
	if !bytes.Equal(prevD, a) {
		t.Fatal("yesterday's salt should be returned alongside today's")
	}

	// Two days later, the first salt is deleted and can no longer be used.
	n, err := s.DeleteOld(ctx, nextDay.AddDate(0, 0, 1))
	if err != nil || n != 1 {
		t.Fatalf("deleted %d, err %v; want 1", n, err)
	}
	var count int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM salts WHERE day = $1`, Day(morning)).Scan(&count)
	if count != 0 {
		t.Fatal("the old salt is still stored")
	}
}
