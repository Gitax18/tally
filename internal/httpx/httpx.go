// Package httpx holds the HTTP pieces every handler shares: JSON responses, errors, logging and
// panic recovery.
package httpx

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	chimw "github.com/go-chi/chi/v5/middleware"
)

// Error is the body of every error response. Code is stable and machine-readable; nothing about
// the underlying failure is ever sent.
type Error struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// WriteJSON writes v as a JSON response with the given status.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// WriteError writes {"error": code}.
func WriteError(w http.ResponseWriter, status int, code string) {
	WriteJSON(w, status, Error{Error: code})
}

// WriteErrorMessage writes {"error": code, "message": msg}. msg must be safe to show a user.
func WriteErrorMessage(w http.ResponseWriter, status int, code, msg string) {
	WriteJSON(w, status, Error{Error: code, Message: msg})
}

// DecodeJSON reads a JSON body into v, refusing unknown fields and bodies over limit bytes.
func DecodeJSON(w http.ResponseWriter, r *http.Request, limit int64, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, limit))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("invalid JSON body: %w", err)
	}
	return nil
}

// Recover turns a handler panic into a generic 500 and logs the details.
func Recover(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				rec := recover()
				if rec == nil {
					return
				}
				if rec == http.ErrAbortHandler { //nolint:errorlint // sentinel panic value, not an error chain
					panic(rec)
				}
				logger.ErrorContext(r.Context(), "panic recovered",
					"path", r.URL.Path, "panic", fmt.Sprint(rec), "stack", string(debug.Stack()))
				WriteError(w, http.StatusInternalServerError, "internal_error")
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// AccessLog logs one line per request, except successful event and script requests, which are
// most of the traffic and are counted instead of logged. The query string is never logged.
func AccessLog(logger *slog.Logger, quiet func(r *http.Request, status int) bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			status := ww.Status()
			if status == 0 {
				status = http.StatusOK
			}
			level := slog.LevelInfo
			if quiet != nil && quiet(r, status) {
				level = slog.LevelDebug
			}
			logger.LogAttrs(r.Context(), level, "request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", status),
				slog.Int("bytes", ww.BytesWritten()),
				slog.Duration("duration", time.Since(start)))
		})
	}
}

// NoStore marks API responses as private and uncacheable.
func NoStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Cache-Control", "no-store")
		h.Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}
