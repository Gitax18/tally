package ingest

import "testing"

func TestSource(t *testing.T) {
	tests := []struct {
		name, utm, ref, referrer string
		wantSource, wantRef      string
	}{
		{"no referrer is direct", "", "", "", "Direct", ""},
		{"google", "", "", "https://www.google.com/", "Google", "google.com"},
		{"google country domain", "", "", "https://www.google.co.uk/", "Google", "google.co.uk"},
		{"github with path", "", "", "https://github.com/someone/repo", "GitHub", "github.com/someone/repo"},
		{"hacker news", "", "", "https://news.ycombinator.com/item?id=123", "Hacker News", "news.ycombinator.com/item"},
		{"t.co is X", "", "", "https://t.co/abc", "X", "t.co/abc"},
		{"reddit subdomain", "", "", "https://np.reddit.com/r/golang", "Reddit", "np.reddit.com/r/golang"},
		{"unknown host is the host", "", "", "https://blog.someone.dev/post", "blog.someone.dev", "blog.someone.dev/post"},
		{"utm_source wins over referrer", "newsletter", "", "https://github.com/", "newsletter", "github.com"},
		{"utm_source alias", "hn", "", "", "Hacker News", ""},
		{"utm_source host alias", "chatgpt.com", "", "", "ChatGPT", ""},
		{"ref param", "", "producthunt", "", "Product Hunt", ""},
		{"utm_source beats ref", "test", "github", "", "test", ""},
		{"self referral is direct", "", "", "https://example.com/other", "Direct", ""},
		{"subdomain self referral is direct", "", "", "https://docs.example.com/", "Direct", ""},
		{"self referral keeps campaign", "test", "", "https://example.com/", "test", ""},
		{"android app referrer", "", "", "android-app://com.google.android.gm/", "Gmail", "com.google.android.gm"},
		{"garbage referrer is direct", "", "", "not a url", "Direct", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src, ref := Source(tt.utm, tt.ref, tt.referrer, "example.com")
			if src != tt.wantSource || ref != tt.wantRef {
				t.Fatalf("got (%q, %q), want (%q, %q)", src, ref, tt.wantSource, tt.wantRef)
			}
		})
	}
}

func TestClipKeepsUTF8(t *testing.T) {
	if got := clip("héllo", 2); got != "h" {
		t.Fatalf("got %q", got)
	}
}
