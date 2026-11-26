package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Gitax18/tally/internal/httpx"
	"github.com/Gitax18/tally/internal/sites"
)

// siteWithStats is a site in the sites list: its last-24h visitors and a 7-day sparkline.
type siteWithStats struct {
	sites.Site
	Visitors24h int64   `json:"visitors_24h"`
	Sparkline   []int64 `json:"sparkline"`
}

func (h *handlers) listSites(w http.ResponseWriter, r *http.Request) {
	list, err := h.d.Sites.List(r.Context())
	if err != nil {
		h.writeStoreError(w, r, err)
		return
	}
	out := make([]siteWithStats, 0, len(list))
	for _, s := range list {
		visitors, spark, err := h.d.Stats.Overview(r.Context(), statsSite(s), h.now())
		if err != nil {
			h.writeStoreError(w, r, err)
			return
		}
		out = append(out, siteWithStats{Site: s, Visitors24h: visitors, Sparkline: spark})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"sites": out})
}

type createSiteRequest struct {
	Name     string `json:"name"`
	Domain   string `json:"domain"`
	Timezone string `json:"timezone"`
}

func (h *handlers) createSite(w http.ResponseWriter, r *http.Request) {
	var req createSiteRequest
	if err := httpx.DecodeJSON(w, r, maxJSONBytes, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	site, err := h.d.Sites.Create(r.Context(), req.Name, req.Domain, req.Timezone)
	if err != nil {
		h.writeStoreError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, site)
}

func (h *handlers) getSite(w http.ResponseWriter, r *http.Request) {
	site, err := h.d.Sites.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		h.writeStoreError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, site)
}

func (h *handlers) updateSite(w http.ResponseWriter, r *http.Request) {
	var p sites.Patch
	if err := httpx.DecodeJSON(w, r, maxJSONBytes, &p); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	site, err := h.d.Sites.Update(r.Context(), chi.URLParam(r, "id"), p)
	if err != nil {
		h.writeStoreError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, site)
}

func (h *handlers) deleteSite(w http.ResponseWriter, r *http.Request) {
	if err := h.d.Sites.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		h.writeStoreError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// timezones lists the zones a site can use, and the legacy names browsers may report for them.
func (h *handlers) timezones(w http.ResponseWriter, r *http.Request) {
	zones, err := h.d.Sites.Timezones(r.Context())
	if err != nil {
		h.writeStoreError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"timezones": zones, "aliases": sites.LegacyZones})
}
