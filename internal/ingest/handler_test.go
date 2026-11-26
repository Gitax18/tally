package ingest

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/Gitax18/tally/internal/clientip"
	"github.com/Gitax18/tally/internal/ratelimit"
	"github.com/Gitax18/tally/internal/sites"
)

type fakeSites map[string]sites.Site

func (f fakeSites) Lookup(id string) (sites.Site, bool) { s, ok := f[id]; return s, ok }

type fakeSalts struct{}

func (fakeSalts) Today(context.Context, time.Time) ([]byte, []byte, error) {
	return bytes.Repeat([]byte{1}, 32), nil, nil
}

type fakeGeo struct{}

func (fakeGeo) Country(netip.Addr) string { return "DE" }

type fakeQueue struct{ events []Event }

func (q *fakeQueue) Push(ev Event) bool { q.events = append(q.events, ev); return true }

func newHandler() (*Handler, *fakeQueue) {
	q := &fakeQueue{}
	return &Handler{
		Sites: fakeSites{
			"site000001": {ID: "site000001", Domain: "example.com"},
			"local00001": {ID: "local00001", Domain: "localhost"},
		},
		Salts:    fakeSalts{},
		Geo:      fakeGeo{},
		Queue:    q,
		IPs:      clientip.Resolver{TrustProxy: true},
		Limiter:  ratelimit.New(30, 10*time.Second),
		Counters: &Counters{},
		Logger:   quiet,
		Now:      func() time.Time { return t0 },
	}, q
}

func post(h *Handler, body, ua, ip string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", "/e", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/plain;charset=UTF-8")
	req.Header.Set("User-Agent", ua)
	req.Header.Set("X-Forwarded-For", ip)
	rec := httptest.NewRecorder()
	h.Event(rec, req)
	return rec
}

func TestEventAccepted(t *testing.T) {
	h, q := newHandler()
	body := `{"s":"site000001","n":"pageview","u":"https://www.example.com/blog/post?utm_source=newsletter&utm_campaign=launch","r":"https://github.com/x/y","w":390}`
	rec := post(h, body, uaIPhone, "198.51.100.7")
	if rec.Code != http.StatusAccepted || len(q.events) != 1 {
		t.Fatalf("status %d, %d events", rec.Code, len(q.events))
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("the fetch fallback needs CORS")
	}
	ev := q.events[0]
	want := Dims{Hostname: "example.com", Source: "newsletter", Referrer: "github.com/x/y", UTMSource: "newsletter",
		UTMCampaign: "launch", Country: "DE", Browser: "Safari", OS: "iOS", Device: "Mobile"}
	if ev.Dims != want {
		t.Errorf("dims\n got %+v\nwant %+v", ev.Dims, want)
	}
	if ev.Path != "/blog/post" || ev.Hostname != "example.com" || ev.VisitorID == 0 || ev.Kind != KindPageview {
		t.Errorf("event %+v", ev)
	}

	// The same person on another request gets the same ID; another address does not.
	post(h, body, uaIPhone, "198.51.100.7")
	post(h, body, uaIPhone, "198.51.100.8")
	if q.events[1].VisitorID != ev.VisitorID || q.events[2].VisitorID == ev.VisitorID {
		t.Error("visitor IDs should depend on address and user agent")
	}
}

func TestEventRejections(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		ua     string
		status int
	}{
		{"unknown site", `{"s":"nosuchsite","n":"pageview","u":"https://example.com/"}`, uaMacSafari, 404},
		{"wrong hostname", `{"s":"site000001","n":"pageview","u":"https://evil.io/"}`, uaMacSafari, 403},
		{"lookalike hostname", `{"s":"site000001","n":"pageview","u":"https://notexample.com/"}`, uaMacSafari, 403},
		{"bad json", `{"s":`, uaMacSafari, 400},
		{"unknown kind", `{"s":"site000001","n":"purchase","u":"https://example.com/"}`, uaMacSafari, 400},
		{"not http", `{"s":"site000001","n":"pageview","u":"javascript:alert(1)"}`, uaMacSafari, 400},
		{"too large", `{"s":"site000001","n":"pageview","u":"https://example.com/` + strings.Repeat("a", 3000) + `"}`, uaMacSafari, 413},
		{"bot is accepted but not queued", `{"s":"site000001","n":"pageview","u":"https://example.com/"}`, uaGooglebot, 202},
		{"localhost on a normal site", `{"s":"site000001","n":"pageview","u":"http://localhost:3000/"}`, uaMacSafari, 403},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, q := newHandler()
			rec := post(h, tt.body, tt.ua, "198.51.100.7")
			if rec.Code != tt.status {
				t.Fatalf("status %d, want %d (%s)", rec.Code, tt.status, rec.Body.String())
			}
			if len(q.events) != 0 {
				t.Fatal("a rejected event was queued")
			}
		})
	}
}

func TestLocalSiteAcceptsLocalhost(t *testing.T) {
	h, q := newHandler()
	rec := post(h, `{"s":"local00001","n":"pageview","u":"http://localhost:8099/demo.html","r":"http://localhost:8099/"}`, uaMacSafari, "127.0.0.1")
	if rec.Code != 202 || len(q.events) != 1 || q.events[0].Dims.Source != Direct {
		t.Fatalf("status %d events %+v", rec.Code, q.events)
	}
}

func TestEventRateLimit(t *testing.T) {
	h, q := newHandler()
	body := `{"s":"site000001","n":"pageview","u":"https://example.com/"}`
	for range 30 {
		post(h, body, uaMacSafari, "198.51.100.7")
	}
	if rec := post(h, body, uaMacSafari, "198.51.100.7"); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("request 31 got %d", rec.Code)
	}
	if rec := post(h, body, uaMacSafari, "198.51.100.9"); rec.Code != http.StatusAccepted {
		t.Fatal("another address has its own budget")
	}
	if len(q.events) != 31 || h.Counters.RateLimited.Load() != 1 {
		t.Fatalf("%d events, %d limited", len(q.events), h.Counters.RateLimited.Load())
	}
}

func TestEngagementPayload(t *testing.T) {
	h, q := newHandler()
	post(h, `{"s":"site000001","n":"engagement","u":"https://example.com/docs","e":12345}`, uaMacSafari, "198.51.100.7")
	post(h, `{"s":"site000001","n":"engagement","u":"https://example.com/docs","e":-5}`, uaMacSafari, "198.51.100.7")
	post(h, `{"s":"site000001","n":"engagement","u":"https://example.com/docs","e":999999999999}`, uaMacSafari, "198.51.100.7")
	if len(q.events) != 2 || q.events[0].EngagedMS != 12345 || q.events[1].EngagedMS != maxEngagedMS {
		t.Fatalf("events %+v", q.events)
	}
}

func TestScript(t *testing.T) {
	h, _ := newHandler()
	req := httptest.NewRequest("GET", "/s/site000001.js", nil)
	req.Header.Set("Accept-Encoding", "gzip, br")
	rec := httptest.NewRecorder()
	h.Script(rec, req, "site000001")
	if rec.Code != 200 || rec.Header().Get("Cache-Control") != "public, max-age=3600" || rec.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("status %d headers %v", rec.Code, rec.Header())
	}
	zr, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	js, _ := io.ReadAll(zr)
	if !strings.Contains(string(js), `"site000001","0")`) {
		t.Fatal("the site ID is not in the script")
	}

	rec = httptest.NewRecorder()
	h.Script(rec, httptest.NewRequest("GET", "/s/local00001.js", nil), "local00001")
	if !strings.Contains(rec.Body.String(), `"local00001","1")`) {
		t.Fatal("a localhost site's script should allow localhost")
	}

	rec = httptest.NewRecorder()
	h.Script(rec, httptest.NewRequest("GET", "/s/nosuchsite.js", nil), "nosuchsite")
	if rec.Code != 404 {
		t.Fatalf("unknown site: %d", rec.Code)
	}
}
