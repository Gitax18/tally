// Package filter parses dashboard filters ("country:IN,source:GitHub") and turns them into SQL
// conditions.
//
// Only whitelisted dimensions are accepted, and they map to fixed column names; filter values
// only ever reach SQL as bind parameters. Values are percent-encoded by the dashboard
// (encodeURIComponent), so a value may contain "," or ":".
package filter

import (
	"fmt"
	"net/url"
	"strings"
)

// Level says which table a dimension lives on.
type Level int

const (
	// VisitAttr dimensions are a visit's attributes, stored on sessions and copied onto every
	// event, so both tables can filter on them directly.
	VisitAttr Level = iota
	// Page is the event's own path; visit metrics match visits that include that page.
	Page
	// VisitOnly dimensions (entry and exit page) exist only on sessions; event metrics match
	// events of those visits.
	VisitOnly
)

// Dimension is one filterable, breakdown-able attribute.
type Dimension struct {
	Name   string // as used in the API and the URL
	Column string // fixed column name; never user input
	Level  Level
}

var dimensions = map[string]Dimension{
	"source":       {"source", "source", VisitAttr},
	"referrer":     {"referrer", "referrer", VisitAttr},
	"utm_source":   {"utm_source", "utm_source", VisitAttr},
	"utm_medium":   {"utm_medium", "utm_medium", VisitAttr},
	"utm_campaign": {"utm_campaign", "utm_campaign", VisitAttr},
	"country":      {"country", "country", VisitAttr},
	"browser":      {"browser", "browser", VisitAttr},
	"os":           {"os", "os", VisitAttr},
	"device":       {"device", "device", VisitAttr},
	"hostname":     {"hostname", "hostname", VisitAttr},
	"page":         {"page", "path", Page},
	"entry_page":   {"entry_page", "entry_path", VisitOnly},
	"exit_page":    {"exit_page", "exit_path", VisitOnly},
}

// Lookup returns a whitelisted dimension.
func Lookup(name string) (Dimension, bool) {
	d, ok := dimensions[name]
	return d, ok
}

// Filter is one "is" match.
type Filter struct {
	Dim   Dimension
	Value string
}

// Filters are combined with AND.
type Filters []Filter

const (
	maxFilters    = 10
	maxValueBytes = 1000
)

// Parse reads "dim:value,dim:value". An empty string is no filters.
func Parse(raw string) (Filters, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	if len(parts) > maxFilters {
		return nil, fmt.Errorf("at most %d filters", maxFilters)
	}
	var out Filters
	seen := map[string]bool{}
	for _, part := range parts {
		name, encoded, ok := strings.Cut(part, ":")
		if !ok {
			return nil, fmt.Errorf("filter %q must look like dimension:value", clip(part))
		}
		dim, ok := Lookup(name)
		if !ok {
			return nil, fmt.Errorf("unknown filter dimension %q", clip(name))
		}
		if seen[name] {
			return nil, fmt.Errorf("dimension %q is filtered twice", name)
		}
		seen[name] = true
		value, err := url.PathUnescape(encoded)
		if err != nil {
			return nil, fmt.Errorf("filter %q has a malformed escape", name)
		}
		if len(value) > maxValueBytes {
			return nil, fmt.Errorf("filter %q is too long", name)
		}
		out = append(out, Filter{Dim: dim, Value: value})
	}
	return out, nil
}

// String renders filters back into the URL form.
func (fs Filters) String() string {
	parts := make([]string, len(fs))
	for i, f := range fs {
		parts[i] = f.Dim.Name + ":" + url.PathEscape(f.Value)
	}
	return strings.Join(parts, ",")
}

// Args collects bind parameters and hands out their placeholders.
type Args struct{ vals []any }

// Add appends v and returns its placeholder ($1, $2, ...).
func (a *Args) Add(v any) string {
	a.vals = append(a.vals, v)
	return fmt.Sprintf("$%d", len(a.vals))
}

// Values are the parameters in order.
func (a *Args) Values() []any { return a.vals }

// Scope is the query a condition is for: the site and time range placeholders already added.
type Scope struct {
	Site, From, To string
}

// Events returns " AND ..." conditions for a query on events.
func (fs Filters) Events(a *Args, s Scope) string {
	var b strings.Builder
	for _, f := range fs {
		ph := a.Add(f.Value)
		switch f.Dim.Level {
		case VisitAttr, Page:
			fmt.Fprintf(&b, " AND %s = %s", f.Dim.Column, ph)
		case VisitOnly:
			// Visits that started up to a day before the range can still have events in it.
			fmt.Fprintf(&b, ` AND session_id IN (SELECT id FROM sessions WHERE site_id = %s
				AND started_at >= %s::timestamptz - interval '1 day' AND started_at < %s AND %s = %s)`,
				s.Site, s.From, s.To, f.Dim.Column, ph)
		}
	}
	return b.String()
}

// Sessions returns " AND ..." conditions for a query on sessions.
func (fs Filters) Sessions(a *Args, s Scope) string {
	var b strings.Builder
	for _, f := range fs {
		ph := a.Add(f.Value)
		switch f.Dim.Level {
		case VisitAttr, VisitOnly:
			fmt.Fprintf(&b, " AND %s = %s", f.Dim.Column, ph)
		case Page:
			fmt.Fprintf(&b, ` AND id IN (SELECT session_id FROM events WHERE site_id = %s
				AND name = 'pageview' AND ts >= %s AND ts < %s AND path = %s)`,
				s.Site, s.From, s.To, ph)
		}
	}
	return b.String()
}

func clip(s string) string {
	if len(s) > 40 {
		return s[:40] + "…"
	}
	return s
}
