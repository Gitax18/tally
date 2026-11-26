// Package ingest receives tracker events: it validates them, works out who and where they come
// from without storing the address, and hands them to a single worker that writes visits and
// pageviews in order.
package ingest

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"github.com/Gitax18/tally/internal/clientip"
	"github.com/Gitax18/tally/internal/ratelimit"
	"github.com/Gitax18/tally/internal/sites"
	"github.com/Gitax18/tally/internal/visitor"
	"github.com/Gitax18/tally/tracker"
)

const (
	// MaxBodyBytes is the largest event the endpoint accepts.
	MaxBodyBytes = 2 << 10
	maxPathBytes = 1024
	// maxEngagedMS caps one engagement ping (a tab left visible all day is not six hours read).
	maxEngagedMS = 6 * 60 * 60 * 1000
)

// Sites is what the handler needs from the site store.
type Sites interface {
	Lookup(id string) (sites.Site, bool)
}

// Salts is what the handler needs from the salt store.
type Salts interface {
	Today(ctx context.Context, now time.Time) (today, yesterday []byte, err error)
}

// Geo resolves a country code.
type Geo interface {
	Country(ip netip.Addr) string
}

// Queue accepts events for the worker.
type Queue interface {
	Push(Event) bool
}

// Handler serves /s/{id}.js and /e.
type Handler struct {
	Sites    Sites
	Salts    Salts
	Geo      Geo
	Queue    Queue
	IPs      clientip.Resolver
	Limiter  *ratelimit.Limiter // 30 requests per 10 seconds per address
	Counters *Counters
	Logger   *slog.Logger
	Now      func() time.Time
}

// payload is what the tracker sends. Field names are one letter to keep the script small.
type payload struct {
	Site     string `json:"s"`
	Kind     string `json:"n"`
	URL      string `json:"u"`
	Referrer string `json:"r"`
	Width    int    `json:"w"`
	Engaged  int64  `json:"e"`
}

// Script serves the tracker with the site ID written in. Browsers and CDNs cache it for an hour.
func (h *Handler) Script(w http.ResponseWriter, r *http.Request, id string) {
	site, ok := h.Sites.Lookup(id)
	if !ok {
		w.Header().Set("Cache-Control", "public, max-age=60")
		http.Error(w, "// unknown site\n", http.StatusNotFound)
		return
	}
	body := tracker.Script(site.ID, site.IsLocal())
	hdr := w.Header()
	hdr.Set("Content-Type", "text/javascript; charset=utf-8")
	hdr.Set("Cache-Control", "public, max-age=3600")
	hdr.Set("Cross-Origin-Resource-Policy", "cross-origin")
	hdr.Set("X-Content-Type-Options", "nosniff")
	hdr.Set("Vary", "Accept-Encoding")
	if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		var buf bytes.Buffer
		zw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
		_, _ = zw.Write(body)
		_ = zw.Close()
		hdr.Set("Content-Encoding", "gzip")
		body = buf.Bytes()
	}
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(body)
}

// Event handles POST /e. Responses carry no detail beyond a short code: the tracker ignores them,
// and they are only there for someone debugging with curl.
func (h *Handler) Event(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "POST")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, MaxBodyBytes))
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			h.reject(w, http.StatusRequestEntityTooLarge, "too_large", "", "")
			return
		}
		h.reject(w, http.StatusBadRequest, "unreadable", "", "")
		return
	}

	ip := h.IPs.Addr(r)
	if !h.Limiter.Allow(clientip.RateKey(ip)) {
		h.Counters.RateLimited.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
		return
	}

	var p payload
	if err := json.Unmarshal(body, &p); err != nil {
		h.reject(w, http.StatusBadRequest, "invalid_json", "", "")
		return
	}
	if p.Kind != KindPageview && p.Kind != KindEngagement {
		h.reject(w, http.StatusBadRequest, "invalid_kind", p.Site, "")
		return
	}
	page, err := url.Parse(p.URL)
	if err != nil || (page.Scheme != "http" && page.Scheme != "https") || page.Host == "" {
		h.reject(w, http.StatusBadRequest, "invalid_url", p.Site, "")
		return
	}

	site, ok := h.Sites.Lookup(p.Site)
	if !ok {
		h.reject(w, http.StatusNotFound, "unknown_site", p.Site, "")
		return
	}
	host := strings.ToLower(page.Hostname())
	if !site.HostAllowed(host) {
		h.reject(w, http.StatusForbidden, "hostname_mismatch", site.ID, host)
		return
	}

	ua := r.UserAgent()
	if IsBot(ua) {
		h.Counters.Bots.Add(1)
		w.WriteHeader(http.StatusAccepted)
		return
	}

	now := h.Now()
	today, yesterday, err := h.Salts.Today(r.Context(), now)
	if err != nil {
		h.Logger.Error("ingest: salt unavailable", "error", err)
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	ev := Event{
		SiteID:    site.ID,
		Kind:      p.Kind,
		TS:        now,
		VisitorID: visitor.Hash(today, site.ID, ip, ua),
		Path:      normalizePath(page),
		Hostname:  strings.TrimPrefix(host, "www."),
	}
	if yesterday != nil {
		ev.PrevVisitorID = visitor.Hash(yesterday, site.ID, ip, ua)
	}
	if p.Kind == KindEngagement {
		if p.Engaged <= 0 {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		ev.EngagedMS = min(p.Engaged, maxEngagedMS)
	} else {
		q := page.Query()
		ev.Dims = Dims{
			Hostname:    ev.Hostname,
			UTMSource:   clip(strings.TrimSpace(q.Get("utm_source")), 200),
			UTMMedium:   clip(strings.TrimSpace(q.Get("utm_medium")), 200),
			UTMCampaign: clip(strings.TrimSpace(q.Get("utm_campaign")), 200),
			Country:     h.Geo.Country(ip),
		}
		siteDomain := site.Domain
		if site.IsLocal() {
			siteDomain = host
		}
		ev.Dims.Source, ev.Dims.Referrer = Source(ev.Dims.UTMSource, q.Get("ref"), p.Referrer, siteDomain)
		ev.Dims.Browser, ev.Dims.OS, ev.Dims.Device = Client(ua, p.Width)
	}

	if !h.Queue.Push(ev) {
		h.Counters.Dropped.Add(1)
		w.WriteHeader(http.StatusAccepted)
		return
	}
	h.Counters.Accepted.Add(1)
	w.WriteHeader(http.StatusAccepted)
}

// reject answers with a code. Mismatched hostnames are logged, because they are how a misconfigured
// domain shows up; the address is never logged.
func (h *Handler) reject(w http.ResponseWriter, status int, code, siteID, host string) {
	h.Counters.Rejected.Add(1)
	if code == "hostname_mismatch" || code == "unknown_site" {
		h.Logger.Info("ingest: event rejected", "reason", code, "site", clip(siteID, 20), "hostname", clip(host, 100))
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, code+"\n")
}

func normalizePath(u *url.URL) string {
	p := u.EscapedPath()
	if p == "" {
		p = "/"
	}
	if unescaped, err := url.PathUnescape(p); err == nil {
		p = unescaped
	}
	return clip(p, maxPathBytes)
}
