package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Gitax18/tally/internal/clientip"
	"github.com/Gitax18/tally/internal/geo"
	"github.com/Gitax18/tally/internal/ingest"
	"github.com/Gitax18/tally/internal/ratelimit"
	"github.com/Gitax18/tally/internal/sites"
	"github.com/Gitax18/tally/internal/stats"
	"github.com/Gitax18/tally/internal/testdb"
	"github.com/Gitax18/tally/internal/visitor"
)

const secret = "test-api-secret-0123456789abcdef0123456789"

var now = time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)

type harness struct {
	t      *testing.T
	router http.Handler
}

func newHarness(t *testing.T) *harness {
	pool := testdb.New(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	store := sites.NewStore(pool, logger)
	counters := &ingest.Counters{}
	worker := ingest.NewWorker(pool, logger, counters, 10, nil)
	router := Router(Deps{
		Logger:    logger,
		APISecret: secret,
		Sites:     store,
		Stats:     stats.New(pool),
		Usage:     &Usage{Pool: pool, LimitBytes: 1 << 30, Counters: counters, Queued: worker.Len},
		Now:       func() time.Time { return now },
		Ingest: &ingest.Handler{
			Sites: store, Salts: visitor.NewSalts(pool), Geo: &geo.DB{}, Queue: worker,
			IPs: clientip.Resolver{}, Limiter: ratelimit.New(30, 10*time.Second), Counters: counters,
			Logger: logger, Now: time.Now,
		},
	})
	return &harness{t: t, router: router}
}

// do sends a request with the API secret unless auth is set to something else.
func (h *harness) do(method, path, body string, auth ...string) (int, map[string]any) {
	h.t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if len(auth) > 0 {
		if auth[0] != "" {
			req.Header.Set("Authorization", auth[0])
		}
	} else {
		req.Header.Set("Authorization", "Bearer "+secret)
	}
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func TestPrivateAPINeedsTheSecret(t *testing.T) {
	h := newHarness(t)
	for _, auth := range []string{"", "Bearer wrong", "Bearer " + secret + "x", secret} {
		if code, _ := h.do("GET", "/v1/sites", "", auth); code != http.StatusUnauthorized {
			t.Errorf("auth %q: got %d", auth, code)
		}
	}
	if code, _ := h.do("GET", "/v1/sites", ""); code != http.StatusOK {
		t.Fatalf("with the secret: %d", code)
	}
}

func TestSitesAndStatsEndpoints(t *testing.T) {
	h := newHarness(t)

	code, body := h.do("POST", "/v1/sites", `{"name":"Blog","domain":"https://www.example.com/","timezone":"Asia/Kolkata"}`)
	if code != http.StatusCreated || body["domain"] != "example.com" {
		t.Fatalf("create: %d %v", code, body)
	}
	id := body["id"].(string)

	code, body = h.do("POST", "/v1/sites", `{"name":"X","domain":"nope","timezone":"UTC"}`)
	if code != http.StatusUnprocessableEntity || body["field"] != "domain" {
		t.Fatalf("invalid create: %d %v", code, body)
	}
	if code, _ := h.do("POST", "/v1/sites", `{"name":"X","domain":"a.com","extra":1}`); code != http.StatusBadRequest {
		t.Fatalf("unknown fields should be refused: %d", code)
	}

	code, body = h.do("GET", "/v1/sites", "")
	list := body["sites"].([]any)
	if code != 200 || len(list) != 1 || len(list[0].(map[string]any)["sparkline"].([]any)) != 7 {
		t.Fatalf("list: %d %v", code, body)
	}

	base := "/v1/sites/" + id
	for _, path := range []string{
		base + "/summary?period=today",
		base + "/summary?period=custom&from=2026-11-01&to=2026-11-27&filters=country:IN",
		base + "/timeseries?period=7d",
		base + "/timeseries?period=12mo&interval=month",
		base + "/breakdown?period=30d&dimension=page&limit=50&search=blog",
		base + "/breakdown?period=30d&dimension=entry_page",
		base + "/realtime",
		"/v1/usage",
	} {
		if code, body := h.do("GET", path, ""); code != http.StatusOK {
			t.Errorf("GET %s: %d %v", path, code, body)
		}
	}

	_, body = h.do("GET", base+"/timeseries?period=today", "")
	if body["interval"] != "hour" || len(body["current"].([]any)) != 24 || len(body["previous"].([]any)) != 24 {
		t.Errorf("today's chart should be 24 hourly points against 24: %v", body["interval"])
	}
	period := body["period"].(map[string]any)
	if period["from"] != "2026-11-27T00:00:00+05:30" || period["timezone"] != "Asia/Kolkata" {
		t.Errorf("period is not in the site's time zone: %v", period)
	}

	for path, wantCode := range map[string]string{
		base + "/summary?period=week":                   "invalid_period",
		base + "/summary?period=custom&from=2026-11-05": "invalid_period",
		base + "/summary?filters=visitor_id:1":          "invalid_filters",
		base + "/breakdown?dimension=visitor_id":        "invalid_dimension",
		base + "/breakdown?dimension=page&limit=0":      "invalid_limit",
		base + "/timeseries?period=30d&interval=hour":   "invalid_interval",
		base + "/timeseries?period=7d&interval=minute":  "invalid_interval",
		"/v1/sites/nosuchsite/summary":                  "site_not_found",
		"/v1/sites/abcdefghij/realtime":                 "site_not_found",
	} {
		code, body := h.do("GET", path, "")
		if body["error"] != wantCode || (code != 400 && code != 404) {
			t.Errorf("GET %s: %d %v, want %s", path, code, body, wantCode)
		}
	}

	code, body = h.do("PATCH", base, `{"timezone":"Europe/Berlin"}`)
	if code != 200 || body["timezone"] != "Europe/Berlin" || body["name"] != "Blog" {
		t.Fatalf("patch: %d %v", code, body)
	}
	if code, _ := h.do("DELETE", base, ""); code != http.StatusNoContent {
		t.Fatalf("delete: %d", code)
	}
	if code, _ := h.do("GET", base, ""); code != http.StatusNotFound {
		t.Fatalf("after delete: %d", code)
	}
}

func TestPublicEndpoints(t *testing.T) {
	h := newHarness(t)
	_, body := h.do("POST", "/v1/sites", `{"name":"Blog","domain":"example.com"}`)
	id := body["id"].(string)

	get := func(method, path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.router.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
		return rec
	}
	if rec := get("GET", "/s/"+id+".js"); rec.Code != 200 || !strings.Contains(rec.Body.String(), id) {
		t.Fatalf("script: %d", rec.Code)
	}
	if rec := get("HEAD", "/s/"+id+".js"); rec.Code != 200 || rec.Body.Len() != 0 {
		t.Fatalf("HEAD script: %d, %d bytes", rec.Code, rec.Body.Len())
	}
	for _, path := range []string{"/s/" + id + ".css", "/s/NOTVALID!.js", "/s/abcdefghij.js"} {
		if rec := get("GET", path); rec.Code != 404 {
			t.Errorf("%s: %d", path, rec.Code)
		}
	}
	if rec := get("OPTIONS", "/e"); rec.Code != 204 || rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("preflight: %d %v", rec.Code, rec.Header())
	}
	if rec := get("GET", "/healthz"); rec.Code != 200 {
		t.Fatalf("healthz: %d", rec.Code)
	}
	_ = context.Background()
}
