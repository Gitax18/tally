package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Gitax18/tally/internal/filter"
	"github.com/Gitax18/tally/internal/httpx"
	"github.com/Gitax18/tally/internal/sites"
	"github.com/Gitax18/tally/internal/stats"
)

// periodJSON tells the dashboard which ranges a response covers, in the site's time zone.
type periodJSON struct {
	Name         string `json:"name"`
	From         string `json:"from"`
	To           string `json:"to"`
	PreviousFrom string `json:"previous_from"`
	PreviousTo   string `json:"previous_to"`
	Timezone     string `json:"timezone"`
	Interval     string `json:"interval"`
}

func describe(p stats.Period, site sites.Site, interval string) periodJSON {
	f := func(t time.Time) string { return t.In(p.Loc).Format(time.RFC3339) }
	return periodJSON{
		Name: p.Name, From: f(p.Current.From), To: f(p.Current.To),
		PreviousFrom: f(p.Previous.From), PreviousTo: f(p.Previous.To),
		Timezone: site.Timezone, Interval: interval,
	}
}

// statsRequest reads the site and the shared query parameters (period, from, to, filters). It
// writes the error response itself and returns ok=false on failure.
func (h *handlers) statsRequest(w http.ResponseWriter, r *http.Request) (sites.Site, stats.Period, filter.Filters, bool) {
	site, err := h.d.Sites.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		h.writeStoreError(w, r, err)
		return sites.Site{}, stats.Period{}, nil, false
	}
	q := r.URL.Query()
	p, err := stats.ParsePeriod(q.Get("period"), q.Get("from"), q.Get("to"), site.Location(), h.now())
	if err != nil {
		httpx.WriteErrorMessage(w, http.StatusBadRequest, "invalid_period", err.Error())
		return sites.Site{}, stats.Period{}, nil, false
	}
	fs, err := filter.Parse(q.Get("filters"))
	if err != nil {
		httpx.WriteErrorMessage(w, http.StatusBadRequest, "invalid_filters", err.Error())
		return sites.Site{}, stats.Period{}, nil, false
	}
	return site, p, fs, true
}

func (h *handlers) now() time.Time {
	if h.d.Now != nil {
		return h.d.Now()
	}
	return time.Now()
}

func statsSite(s sites.Site) stats.Site { return stats.Site{ID: s.ID, Timezone: s.Timezone} }

func (h *handlers) summary(w http.ResponseWriter, r *http.Request) {
	site, p, fs, ok := h.statsRequest(w, r)
	if !ok {
		return
	}
	res, err := h.d.Stats.Summary(r.Context(), statsSite(site), p, fs, h.now())
	if err != nil {
		h.writeStoreError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"period":   describe(p, site, p.Interval),
		"filters":  fs.String(),
		"current":  res.Current,
		"previous": res.Previous,
		"change":   res.Change,
	})
}

func (h *handlers) timeseries(w http.ResponseWriter, r *http.Request) {
	site, p, fs, ok := h.statsRequest(w, r)
	if !ok {
		return
	}
	interval := r.URL.Query().Get("interval")
	if interval == "" {
		interval = p.Interval
	}
	if err := stats.CheckInterval(interval, p.Current, p.Loc); err != nil {
		httpx.WriteErrorMessage(w, http.StatusBadRequest, "invalid_interval", err.Error())
		return
	}
	res, err := h.d.Stats.Timeseries(r.Context(), statsSite(site), p, interval, fs)
	if err != nil {
		h.writeStoreError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"period":   describe(p, site, interval),
		"interval": res.Interval,
		"current":  res.Current,
		"previous": res.Previous,
	})
}

func (h *handlers) breakdown(w http.ResponseWriter, r *http.Request) {
	site, p, fs, ok := h.statsRequest(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	dim, ok := filter.Lookup(q.Get("dimension"))
	if !ok {
		httpx.WriteErrorMessage(w, http.StatusBadRequest, "invalid_dimension", "unknown dimension")
		return
	}
	limit := 10
	if raw := q.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > stats.MaxBreakdownLimit {
			httpx.WriteErrorMessage(w, http.StatusBadRequest, "invalid_limit", "limit must be 1 to 1000")
			return
		}
		limit = n
	}
	search := q.Get("search")
	if len(search) > 200 {
		search = search[:200]
	}
	res, err := h.d.Stats.Breakdown(r.Context(), statsSite(site), p, dim, fs, limit, search)
	if err != nil {
		h.writeStoreError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

func (h *handlers) realtime(w http.ResponseWriter, r *http.Request) {
	site, err := h.d.Sites.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		h.writeStoreError(w, r, err)
		return
	}
	n, err := h.d.Stats.Realtime(r.Context(), statsSite(site), h.now())
	if err != nil {
		h.writeStoreError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"visitors": n, "last_event_at": site.LastEventAt})
}
