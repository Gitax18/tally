package ingest

import (
	"net/url"
	"strings"
	"unicode/utf8"
)

// Direct is the source of visits with no referrer and no campaign parameters.
const Direct = "Direct"

// knownHosts maps a referrer host (without "www.") to a source name. Hosts not listed here are
// shown as the host itself.
var knownHosts = map[string]string{
	"bing.com":              "Bing",
	"duckduckgo.com":        "DuckDuckGo",
	"search.brave.com":      "Brave Search",
	"ecosia.org":            "Ecosia",
	"kagi.com":              "Kagi",
	"baidu.com":             "Baidu",
	"yandex.ru":             "Yandex",
	"yandex.com":            "Yandex",
	"search.yahoo.com":      "Yahoo",
	"yahoo.com":             "Yahoo",
	"startpage.com":         "Startpage",
	"github.com":            "GitHub",
	"news.ycombinator.com":  "Hacker News",
	"reddit.com":            "Reddit",
	"old.reddit.com":        "Reddit",
	"out.reddit.com":        "Reddit",
	"t.co":                  "X",
	"twitter.com":           "X",
	"x.com":                 "X",
	"facebook.com":          "Facebook",
	"m.facebook.com":        "Facebook",
	"l.facebook.com":        "Facebook",
	"lm.facebook.com":       "Facebook",
	"instagram.com":         "Instagram",
	"l.instagram.com":       "Instagram",
	"linkedin.com":          "LinkedIn",
	"lnkd.in":               "LinkedIn",
	"youtube.com":           "YouTube",
	"m.youtube.com":         "YouTube",
	"producthunt.com":       "Product Hunt",
	"lobste.rs":             "Lobsters",
	"dev.to":                "DEV",
	"medium.com":            "Medium",
	"stackoverflow.com":     "Stack Overflow",
	"bsky.app":              "Bluesky",
	"mastodon.social":       "Mastodon",
	"threads.net":           "Threads",
	"t.me":                  "Telegram",
	"web.telegram.org":      "Telegram",
	"discord.com":           "Discord",
	"slack.com":             "Slack",
	"app.slack.com":         "Slack",
	"chatgpt.com":           "ChatGPT",
	"chat.openai.com":       "ChatGPT",
	"perplexity.ai":         "Perplexity",
	"claude.ai":             "Claude",
	"gemini.google.com":     "Gemini",
	"copilot.microsoft.com": "Copilot",
	"mail.google.com":       "Gmail",
	"com.google.android.gm": "Gmail",
	"com.google.android.googlequicksearchbox": "Google",
	"com.reddit.frontpage":                    "Reddit",
	"com.linkedin.android":                    "LinkedIn",
	"org.telegram.messenger":                  "Telegram",
}

// aliases maps lowercase utm_source / ref values people commonly type to a source name, so
// "?ref=hn" and a referrer from news.ycombinator.com land in the same row.
var aliases = map[string]string{
	"google":      "Google",
	"bing":        "Bing",
	"duckduckgo":  "DuckDuckGo",
	"github":      "GitHub",
	"hn":          "Hacker News",
	"hackernews":  "Hacker News",
	"hacker news": "Hacker News",
	"reddit":      "Reddit",
	"twitter":     "X",
	"x":           "X",
	"facebook":    "Facebook",
	"fb":          "Facebook",
	"instagram":   "Instagram",
	"linkedin":    "LinkedIn",
	"youtube":     "YouTube",
	"producthunt": "Product Hunt",
	"lobsters":    "Lobsters",
	"devto":       "DEV",
	"dev.to":      "DEV",
	"bluesky":     "Bluesky",
	"mastodon":    "Mastodon",
	"chatgpt":     "ChatGPT",
	"perplexity":  "Perplexity",
}

// Source decides the visit's source. Campaign parameters win over the referrer, because they are
// what the site owner deliberately set: utm_source first, then ref. A referrer from the site
// itself is internal navigation, not a source.
func Source(utmSource, ref, referrer, siteDomain string) (source, referrerOut string) {
	host, path := referrerParts(referrer)
	if host != "" && (host == siteDomain || strings.HasSuffix(host, "."+siteDomain)) {
		host, path = "", ""
	}
	if host != "" {
		referrerOut = clip(host+path, 500)
	}

	switch {
	case strings.TrimSpace(utmSource) != "":
		return named(utmSource), referrerOut
	case strings.TrimSpace(ref) != "":
		return named(ref), referrerOut
	case host != "":
		return hostSource(host), referrerOut
	}
	return Direct, ""
}

// named maps a campaign value to a known source name, or keeps it as typed.
func named(v string) string {
	v = strings.TrimSpace(v)
	key := strings.TrimPrefix(strings.ToLower(v), "www.")
	if name, ok := aliases[key]; ok {
		return name
	}
	if name, ok := knownHosts[key]; ok {
		return name
	}
	return clip(v, 100)
}

func hostSource(host string) string {
	if name, ok := knownHosts[host]; ok {
		return name
	}
	// google.com, google.co.uk, google.de, ...
	if strings.HasPrefix(host, "google.") || strings.Contains(host, ".google.") {
		return "Google"
	}
	if strings.HasSuffix(host, ".reddit.com") {
		return "Reddit"
	}
	return clip(host, 100)
}

// referrerParts returns the referrer's host (lowercase, without "www.") and its path without the
// query string. A bare "/" path is dropped.
func referrerParts(raw string) (host, path string) {
	if raw == "" {
		return "", ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", ""
	}
	host = strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	path = u.EscapedPath()
	if path == "/" {
		path = ""
	}
	return host, path
}

// clip shortens s to at most n bytes without splitting a UTF-8 character.
func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	s = s[:n]
	for len(s) > 0 && !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
}
