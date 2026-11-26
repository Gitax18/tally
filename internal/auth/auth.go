// Package auth checks the dashboard's single admin password.
//
// The password is never stored: ADMIN_PASSWORD_HASH holds an scrypt hash, made by
// `go run ./cmd/server hash-password`. The dashboard's server sends the typed password to
// POST /v1/login and, on success, sets its own signed session cookie.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/netip"
	"strconv"
	"strings"

	"golang.org/x/crypto/scrypt"

	"github.com/Gitax18/tally/internal/clientip"
	"github.com/Gitax18/tally/internal/httpx"
	"github.com/Gitax18/tally/internal/ratelimit"
)

// MinPasswordLength is enforced when a hash is created.
const MinPasswordLength = 12

// Cost parameters for new hashes (about 50-100 ms and 32 MiB per check).
const (
	scryptN      = 1 << 15
	scryptR      = 8
	scryptP      = 1
	scryptKeyLen = 32
	saltLen      = 16
)

// VisitorIPHeader carries the browser's address from the dashboard's server. It is believed
// only because the request also carries API_SECRET.
const VisitorIPHeader = "X-Tally-Visitor-IP"

var b64 = base64.RawStdEncoding

// HashPassword returns "scrypt$N$r$p$salt$key".
func HashPassword(password string) (string, error) {
	if len([]rune(password)) < MinPasswordLength {
		return "", fmt.Errorf("the password must be at least %d characters", MinPasswordLength)
	}
	salt := make([]byte, saltLen)
	_, _ = rand.Read(salt)
	key, err := scrypt.Key([]byte(password), salt, scryptN, scryptR, scryptP, scryptKeyLen)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("scrypt$%d$%d$%d$%s$%s", scryptN, scryptR, scryptP, b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

// ErrMalformedHash means ADMIN_PASSWORD_HASH is not something HashPassword produced.
var ErrMalformedHash = errors.New("ADMIN_PASSWORD_HASH is malformed")

type parsed struct {
	n, r, p   int
	salt, key []byte
}

func parse(encoded string) (parsed, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "scrypt" {
		return parsed{}, ErrMalformedHash
	}
	var h parsed
	var err error
	if h.n, err = strconv.Atoi(parts[1]); err != nil || h.n < 2 || h.n > 1<<20 || h.n&(h.n-1) != 0 {
		return parsed{}, ErrMalformedHash
	}
	if h.r, err = strconv.Atoi(parts[2]); err != nil || h.r < 1 || h.r > 32 {
		return parsed{}, ErrMalformedHash
	}
	if h.p, err = strconv.Atoi(parts[3]); err != nil || h.p < 1 || h.p > 16 {
		return parsed{}, ErrMalformedHash
	}
	if h.salt, err = b64.DecodeString(parts[4]); err != nil || len(h.salt) < 8 {
		return parsed{}, ErrMalformedHash
	}
	if h.key, err = b64.DecodeString(parts[5]); err != nil || len(h.key) < 16 {
		return parsed{}, ErrMalformedHash
	}
	return h, nil
}

// ValidateHash reports whether encoded can be used to check passwords. The server checks it at
// startup, so a pasted-wrong hash fails the deploy instead of every login.
func ValidateHash(encoded string) error {
	_, err := parse(encoded)
	return err
}

// Verify reports whether password matches encoded.
func Verify(encoded, password string) bool {
	h, err := parse(encoded)
	if err != nil {
		return false
	}
	key, err := scrypt.Key([]byte(password), h.salt, h.n, h.r, h.p, len(h.key))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(key, h.key) == 1
}

// Login handles POST /v1/login: {"password": "..."} -> 204, 401 or 429.
type Login struct {
	Hash    string
	Limiter *ratelimit.Limiter // 5 attempts per minute per visitor address
	IPs     clientip.Resolver
	Logger  *slog.Logger
}

func (l *Login) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key := clientip.RateKey(l.IPs.Addr(r))
	if a, err := netip.ParseAddr(strings.TrimSpace(r.Header.Get(VisitorIPHeader))); err == nil {
		key = clientip.RateKey(a.Unmap())
	}
	if !l.Limiter.Allow(key) {
		w.Header().Set("Retry-After", "60")
		httpx.WriteErrorMessage(w, http.StatusTooManyRequests, "rate_limited", "Too many attempts. Wait a minute and try again.")
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if err := httpx.DecodeJSON(w, r, 4<<10, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	if req.Password == "" || !Verify(l.Hash, req.Password) {
		l.Logger.Info("auth: failed login")
		httpx.WriteErrorMessage(w, http.StatusUnauthorized, "invalid_password", "That password is not correct.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
