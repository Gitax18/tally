package ratelimit

import (
	"testing"
	"time"
)

func TestThirtyPerTenSeconds(t *testing.T) {
	l := New(30, 10*time.Second)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range 30 {
		if !l.AllowAt("a", now) {
			t.Fatalf("request %d of the burst was refused", i+1)
		}
	}
	if l.AllowAt("a", now) {
		t.Fatal("request 31 in the same instant should be refused")
	}
	if !l.AllowAt("b", now) {
		t.Fatal("another key has its own budget")
	}
	// One token comes back every 10s/30.
	if !l.AllowAt("a", now.Add(10*time.Second/30)) {
		t.Fatal("a token should refill")
	}
	// After a full period the whole burst is back.
	later := now.Add(11 * time.Second)
	for i := range 30 {
		if !l.AllowAt("a", later) {
			t.Fatalf("after refilling, request %d was refused", i+1)
		}
	}
}

func TestIdleKeysAreForgotten(t *testing.T) {
	l := New(5, time.Minute)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l.AllowAt("a", now)
	l.AllowAt("b", now.Add(idleAfter+2*time.Minute))
	if _, ok := l.keys["a"]; ok {
		t.Fatal("an idle key should be swept")
	}
}
