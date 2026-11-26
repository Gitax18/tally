// Package visitor turns an address and user agent into an anonymous, daily-rotating visitor ID.
//
// ID = first 8 bytes of SHA-256(daily_salt ‖ site_id ‖ ip ‖ user_agent). The salt is random,
// lives only in Postgres, and is deleted once it is older than yesterday, after which an old ID
// can no longer be linked to an address. Neither the address nor the user agent is ever stored.
package visitor

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"net/netip"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const saltBytes = 32

// Hash returns the visitor ID for one site, address and user agent under salt.
func Hash(salt []byte, siteID string, ip netip.Addr, userAgent string) int64 {
	h := sha256.New()
	h.Write(salt)
	h.Write([]byte(siteID))
	h.Write([]byte{0})
	ipBytes, _ := ip.MarshalBinary()
	h.Write(ipBytes)
	h.Write([]byte{0})
	h.Write([]byte(userAgent))
	sum := h.Sum(nil)
	return int64(binary.BigEndian.Uint64(sum[:8]))
}

// Day is the UTC calendar day a salt belongs to.
func Day(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// Salts reads and creates the daily salts, caching them in memory.
type Salts struct {
	pool  *pgxpool.Pool
	mu    sync.Mutex
	byDay map[time.Time][]byte
}

// NewSalts returns a salt store backed by the salts table.
func NewSalts(pool *pgxpool.Pool) *Salts {
	return &Salts{pool: pool, byDay: map[time.Time][]byte{}}
}

// Today returns today's salt, creating it on first use, and yesterday's salt if it still
// exists (nil otherwise). Yesterday's is needed so a visit that crosses midnight keeps its
// session instead of starting a new one.
func (s *Salts) Today(ctx context.Context, now time.Time) (today, yesterday []byte, err error) {
	day := Day(now)
	today, err = s.get(ctx, day, true)
	if err != nil {
		return nil, nil, err
	}
	yesterday, err = s.get(ctx, day.AddDate(0, 0, -1), false)
	if err != nil {
		return nil, nil, err
	}
	return today, yesterday, nil
}

func (s *Salts) get(ctx context.Context, day time.Time, create bool) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if salt, ok := s.byDay[day]; ok {
		return salt, nil
	}
	if create {
		fresh := make([]byte, saltBytes)
		_, _ = rand.Read(fresh)
		// Concurrent creators (two dynos during a deploy) agree on whichever row won.
		if _, err := s.pool.Exec(ctx, `INSERT INTO salts (day, salt) VALUES ($1, $2) ON CONFLICT (day) DO NOTHING`, day, fresh); err != nil {
			return nil, fmt.Errorf("creating the salt: %w", err)
		}
	}
	var salt []byte
	err := s.pool.QueryRow(ctx, `SELECT salt FROM salts WHERE day = $1`, day).Scan(&salt)
	if errors.Is(err, pgx.ErrNoRows) {
		// Remember that it is gone, so every request does not look it up again.
		s.byDay[day] = nil
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading the salt: %w", err)
	}
	s.byDay[day] = salt
	return salt, nil
}

// DeleteOld removes salts older than yesterday, in the database and in memory.
func (s *Salts) DeleteOld(ctx context.Context, now time.Time) (int64, error) {
	cutoff := Day(now).AddDate(0, 0, -1)
	tag, err := s.pool.Exec(ctx, `DELETE FROM salts WHERE day < $1`, cutoff)
	if err != nil {
		return 0, err
	}
	s.mu.Lock()
	for day := range s.byDay {
		if day.Before(cutoff) {
			delete(s.byDay, day)
		}
	}
	s.mu.Unlock()
	return tag.RowsAffected(), nil
}
