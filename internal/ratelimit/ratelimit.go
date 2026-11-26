// Package ratelimit is an in-memory, per-key token bucket. The service runs on one dyno, so
// memory is the whole truth; a restart forgets the counts, which only errs toward allowing.
package ratelimit

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// idleAfter is how long an unused key is kept before it is forgotten.
const idleAfter = 10 * time.Minute

// Limiter allows `burst` requests per key at once, refilled evenly over `per`.
type Limiter struct {
	limit rate.Limit
	burst int

	mu        sync.Mutex
	keys      map[string]*entry
	lastSweep time.Time
}

type entry struct {
	lim  *rate.Limiter
	seen time.Time
}

// New returns a limiter allowing n requests per period for each key.
func New(n int, per time.Duration) *Limiter {
	return &Limiter{limit: rate.Every(per / time.Duration(n)), burst: n, keys: map[string]*entry{}}
}

// Allow reports whether one more request for key is allowed now.
func (l *Limiter) Allow(key string) bool { return l.AllowAt(key, time.Now()) }

// AllowAt is Allow at a given time, for tests.
func (l *Limiter) AllowAt(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if now.Sub(l.lastSweep) > time.Minute {
		for k, e := range l.keys {
			if now.Sub(e.seen) > idleAfter {
				delete(l.keys, k)
			}
		}
		l.lastSweep = now
	}
	e, ok := l.keys[key]
	if !ok {
		e = &entry{lim: rate.NewLimiter(l.limit, l.burst)}
		l.keys[key] = e
	}
	e.seen = now
	return e.lim.AllowN(now, 1)
}
