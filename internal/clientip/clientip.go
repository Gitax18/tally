// Package clientip finds the visitor's address.
//
// On Heroku every request arrives through the router, which appends the address it received the
// request from to X-Forwarded-For. So the LAST entry is the visitor as Heroku saw them. Entries
// to its left were sent by the client and can say anything, so they are never used.
//
// Without a proxy in front (local development), X-Forwarded-For is ignored entirely and the TCP
// peer is the client.
package clientip

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// Resolver picks the client address of a request.
type Resolver struct {
	// TrustProxy says a proxy that appends to X-Forwarded-For (the Heroku router) is in front.
	TrustProxy bool
}

// Addr returns the client address, or the zero Addr when none is usable.
func (r Resolver) Addr(req *http.Request) netip.Addr {
	if r.TrustProxy {
		if a, ok := lastHop(req.Header.Values("X-Forwarded-For")); ok {
			return a
		}
	}
	return peer(req.RemoteAddr)
}

// lastHop returns the right-most entry of all X-Forwarded-For headers.
func lastHop(values []string) (netip.Addr, bool) {
	for i := len(values) - 1; i >= 0; i-- {
		v := values[i]
		if j := strings.LastIndexByte(v, ','); j >= 0 {
			v = v[j+1:]
		}
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		a, err := netip.ParseAddr(v)
		if err != nil {
			return netip.Addr{}, false
		}
		return a.Unmap(), true
	}
	return netip.Addr{}, false
}

func peer(remote string) netip.Addr {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		host = remote
	}
	a, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}
	}
	return a.Unmap()
}

// RateKey is the part of an address that rate limits count against. An IPv6 user usually
// controls a whole /64, so limiting single IPv6 addresses would be easy to evade.
func RateKey(a netip.Addr) string {
	if !a.IsValid() {
		return "unknown"
	}
	if a.Is4() {
		return a.String()
	}
	p, _ := a.Prefix(64)
	return p.String()
}
