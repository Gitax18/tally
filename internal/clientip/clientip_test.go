package clientip

import (
	"net/http/httptest"
	"testing"
)

func TestAddr(t *testing.T) {
	tests := []struct {
		name   string
		trust  bool
		remote string
		xff    []string
		want   string
	}{
		{"no proxy uses the peer", false, "203.0.113.9:5555", nil, "203.0.113.9"},
		{"no proxy ignores a forged header", false, "203.0.113.9:5555", []string{"1.2.3.4"}, "203.0.113.9"},
		{"heroku: the single hop", true, "10.0.0.1:5555", []string{"198.51.100.7"}, "198.51.100.7"},
		{"heroku: the LAST hop, not the spoofable first", true, "10.0.0.1:5555", []string{"1.2.3.4, 198.51.100.7"}, "198.51.100.7"},
		{"heroku: last of several headers", true, "10.0.0.1:5555", []string{"1.2.3.4", "5.6.7.8, 198.51.100.7"}, "198.51.100.7"},
		{"heroku: spaces are trimmed", true, "10.0.0.1:5555", []string{"1.2.3.4 ,  198.51.100.7  "}, "198.51.100.7"},
		{"heroku: ipv6", true, "10.0.0.1:5555", []string{"2001:db8::1"}, "2001:db8::1"},
		{"heroku: v4-mapped v6 is unmapped", true, "10.0.0.1:5555", []string{"::ffff:198.51.100.7"}, "198.51.100.7"},
		{"heroku: garbage falls back to the peer", true, "10.0.0.1:5555", []string{"1.2.3.4, nonsense"}, "10.0.0.1"},
		{"heroku: no header falls back to the peer", true, "10.0.0.1:5555", nil, "10.0.0.1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/e", nil)
			req.RemoteAddr = tt.remote
			for _, v := range tt.xff {
				req.Header.Add("X-Forwarded-For", v)
			}
			got := Resolver{TrustProxy: tt.trust}.Addr(req)
			if got.String() != tt.want {
				t.Fatalf("got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestRateKey(t *testing.T) {
	req := httptest.NewRequest("POST", "/e", nil)
	req.Header.Set("X-Forwarded-For", "2001:db8:1:2:3:4:5:6")
	a := Resolver{TrustProxy: true}.Addr(req)
	if got := RateKey(a); got != "2001:db8:1:2::/64" {
		t.Fatalf("got %s", got)
	}
}
