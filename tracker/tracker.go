// Package tracker serves the browser script with the site ID written into it.
package tracker

import (
	"bytes"
	_ "embed"
	"strings"
)

//go:generate go run ./gen

//go:embed t.min.js
var minified []byte

// Placeholders that t.js passes to its own IIFE. The server replaces them per site.
const (
	sitePlaceholder  = `"__TALLY_SITE__"`
	localPlaceholder = `"__TALLY_LOCAL__"`
)

var prefix, middle, suffix []byte

func init() {
	s := string(minified)
	i := strings.Index(s, sitePlaceholder)
	j := strings.Index(s, localPlaceholder)
	if i < 0 || j < i || strings.Count(s, sitePlaceholder) != 1 || strings.Count(s, localPlaceholder) != 1 {
		panic("tracker: t.min.js must contain each placeholder exactly once; run go generate ./tracker")
	}
	prefix = []byte(s[:i])
	middle = []byte(s[i+len(sitePlaceholder) : j])
	suffix = []byte(s[j+len(localPlaceholder):])
}

// Script returns the tracker for one site. siteID must already be validated as [a-z0-9]+, so it
// is safe to place inside a JavaScript string literal.
func Script(siteID string, allowLocal bool) []byte {
	local := `"0"`
	if allowLocal {
		local = `"1"`
	}
	var b bytes.Buffer
	b.Grow(len(minified) + len(siteID))
	b.Write(prefix)
	b.WriteString(`"` + siteID + `"`)
	b.Write(middle)
	b.WriteString(local)
	b.Write(suffix)
	return b.Bytes()
}
