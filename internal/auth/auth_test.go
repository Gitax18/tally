package auth

import (
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Gitax18/tally/internal/clientip"
	"github.com/Gitax18/tally/internal/ratelimit"
)

func TestHashAndVerify(t *testing.T) {
	h, err := HashPassword("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateHash(h); err != nil {
		t.Fatal(err)
	}
	if !Verify(h, "correct horse battery") || Verify(h, "correct horse batterz") || Verify(h, "") {
		t.Fatal("verify gave the wrong answer")
	}
	h2, _ := HashPassword("correct horse battery")
	if h == h2 {
		t.Fatal("two hashes of one password should differ (random salt)")
	}
	if _, err := HashPassword("short"); err == nil {
		t.Fatal("a short password should be refused")
	}
}

func TestMalformedHashes(t *testing.T) {
	for _, h := range []string{"", "scrypt$", "bcrypt$1$2$3$4$5", "scrypt$1000$8$1$c2FsdHNhbHQ$a2V5a2V5a2V5a2V5a2V5", "scrypt$99999999$8$1$c2FsdHNhbHQ$a2V5a2V5a2V5a2V5a2V5"} {
		if ValidateHash(h) == nil || Verify(h, "anything") {
			t.Errorf("%q should be rejected", h)
		}
	}
}

func TestLoginHandler(t *testing.T) {
	hash, _ := HashPassword("correct horse battery")
	l := &Login{Hash: hash, Limiter: ratelimit.New(5, time.Minute), IPs: clientip.Resolver{}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	try := func(password, visitorIP string) int {
		req := httptest.NewRequest("POST", "/v1/login", strings.NewReader(`{"password":"`+password+`"}`))
		req.RemoteAddr = "10.0.0.1:1234" // the dashboard's server
		if visitorIP != "" {
			req.Header.Set(VisitorIPHeader, visitorIP)
		}
		rec := httptest.NewRecorder()
		l.ServeHTTP(rec, req)
		return rec.Code
	}
	if code := try("correct horse battery", "198.51.100.1"); code != 204 {
		t.Fatalf("right password: %d", code)
	}
	if code := try("wrong", "198.51.100.1"); code != 401 {
		t.Fatalf("wrong password: %d", code)
	}
	for range 3 {
		try("wrong", "198.51.100.1")
	}
	if code := try("correct horse battery", "198.51.100.1"); code != 429 {
		t.Fatalf("the 6th attempt in a minute should be limited, got %d", code)
	}
	// The limit is per visitor, not per dashboard server.
	if code := try("correct horse battery", "198.51.100.2"); code != 204 {
		t.Fatalf("another visitor should not be limited, got %d", code)
	}
}
