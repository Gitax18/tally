package ingest

import "testing"

const (
	uaMacSafari  = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.1 Safari/605.1.15"
	uaWinChrome  = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36"
	uaIPhone     = "Mozilla/5.0 (iPhone; CPU iPhone OS 18_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.1 Mobile/15E148 Safari/604.1"
	uaAndroidFF  = "Mozilla/5.0 (Android 15; Mobile; rv:133.0) Gecko/133.0 Firefox/133.0"
	uaGooglebot  = "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)"
	uaHeadless   = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) HeadlessChrome/141.0.0.0 Safari/537.36"
	uaCurl       = "curl/8.7.1"
	uaPythonReqs = "python-requests/2.32.3"
)

func TestIsBot(t *testing.T) {
	for ua, want := range map[string]bool{
		uaMacSafari:  false,
		uaWinChrome:  false,
		uaIPhone:     false,
		uaAndroidFF:  false,
		uaGooglebot:  true,
		uaHeadless:   true,
		uaCurl:       true,
		uaPythonReqs: true,
		"":           true,
	} {
		if got := IsBot(ua); got != want {
			t.Errorf("IsBot(%q) = %v, want %v", ua, got, want)
		}
	}
}

func TestClient(t *testing.T) {
	tests := []struct {
		ua                string
		width             int
		browser, os, kind string
	}{
		{uaMacSafari, 1512, "Safari", "macOS", "Desktop"},
		{uaWinChrome, 1920, "Chrome", "Windows", "Desktop"},
		{uaIPhone, 390, "Safari", "iOS", "Mobile"},
		{uaAndroidFF, 412, "Firefox", "Android", "Mobile"},
		{uaWinChrome, 800, "Chrome", "Windows", "Tablet"},
		{uaIPhone, 0, "Safari", "iOS", "Mobile"},
	}
	for _, tt := range tests {
		b, o, d := Client(tt.ua, tt.width)
		if b != tt.browser || o != tt.os || d != tt.kind {
			t.Errorf("Client(%.40q, %d) = %q %q %q; want %q %q %q", tt.ua, tt.width, b, o, d, tt.browser, tt.os, tt.kind)
		}
	}
}
