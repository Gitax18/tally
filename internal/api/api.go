// Package api assembles the HTTP routes: the public tracker endpoints and the private /v1 API
// that only the dashboard's server calls, with API_SECRET as a bearer token.
package api

import (
	"crypto/subtle"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Gitax18/tally/internal/httpx"
	"github.com/Gitax18/tally/internal/ingest"
	"github.com/Gitax18/tally/internal/sites"
	"github.com/Gitax18/tally/internal/stats"
)

// maxJSONBytes bounds request bodies of the private API.
const maxJSONBytes = 16 << 10

// Deps is everything the routes need.
type Deps struct {
	Logger    *slog.Logger
	APISecret string
	Ingest    *ingest.Handler
	Sites     *sites.Store
	Stats     *stats.Service
	Login     http.HandlerFunc
	Usage     http.Handler
	Now       func() time.Time // the clock for periods; time.Now when nil
}

// Router returns the service's handler.
func Router(d Deps) http.Handler {
	r := chi.NewRouter()
	r.Use(httpx.Recover(d.Logger))
	r.Use(httpx.AccessLog(d.Logger, func(req *http.Request, status int) bool {
		// The event and script endpoints are most of the traffic; they are counted, not logged.
		return status < 400 && (req.URL.Path == "/e" || strings.HasPrefix(req.URL.Path, "/s/") || req.URL.Path == "/healthz")
	}))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		_, _ = io.WriteString(w, "ok\n")
	})
	r.Get("/s/{file}", func(w http.ResponseWriter, req *http.Request) {
		id, ok := strings.CutSuffix(chi.URLParam(req, "file"), ".js")
		if !ok || !sites.ValidID(id) {
			http.NotFound(w, req)
			return
		}
		d.Ingest.Script(w, req, id)
	})
	r.Head("/s/{file}", func(w http.ResponseWriter, req *http.Request) {
		id, _ := strings.CutSuffix(chi.URLParam(req, "file"), ".js")
		d.Ingest.Script(w, req, id)
	})
	r.Post("/e", d.Ingest.Event)
	r.Options("/e", d.Ingest.Event)

	r.Route("/v1", func(v1 chi.Router) {
		v1.Use(httpx.NoStore, requireSecret(d.APISecret))
		h := &handlers{d: d}
		if d.Login != nil {
			v1.Post("/login", d.Login)
		}
		if d.Usage != nil {
			v1.Method(http.MethodGet, "/usage", d.Usage)
		}
		v1.Get("/sites", h.listSites)
		v1.Post("/sites", h.createSite)
		v1.Get("/sites/{id}", h.getSite)
		v1.Patch("/sites/{id}", h.updateSite)
		v1.Delete("/sites/{id}", h.deleteSite)
		v1.Get("/sites/{id}/summary", h.summary)
		v1.Get("/sites/{id}/timeseries", h.timeseries)
		v1.Get("/sites/{id}/breakdown", h.breakdown)
		v1.Get("/sites/{id}/realtime", h.realtime)
	})

	r.NotFound(func(w http.ResponseWriter, _ *http.Request) { httpx.WriteError(w, http.StatusNotFound, "not_found") })
	r.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed")
	})
	return r
}

// requireSecret lets a request through only with "Authorization: Bearer <API_SECRET>".
func requireSecret(secret string) func(http.Handler) http.Handler {
	want := []byte("Bearer " + secret)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got := []byte(r.Header.Get("Authorization"))
			if subtle.ConstantTimeCompare(got, want) != 1 {
				httpx.WriteError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

type handlers struct{ d Deps }

// writeStoreError maps store errors to responses without leaking internals.
func (h *handlers) writeStoreError(w http.ResponseWriter, r *http.Request, err error) {
	var verr *sites.ValidationError
	switch {
	case errors.Is(err, sites.ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, "site_not_found")
	case errors.As(err, &verr):
		httpx.WriteJSON(w, http.StatusUnprocessableEntity, map[string]string{
			"error": "invalid", "field": verr.Field, "message": verr.Msg,
		})
	default:
		h.d.Logger.ErrorContext(r.Context(), "api: store error", "path", r.URL.Path, "error", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error")
	}
}
