package ingest

import (
	"strings"

	"github.com/mileusna/useragent"
	"github.com/x-way/crawlerdetect"
)

// IsBot reports whether a user agent belongs to a crawler, a headless browser, or nothing at all.
func IsBot(ua string) bool {
	if len(ua) < 10 {
		return true
	}
	if strings.Contains(ua, "HeadlessChrome") || strings.Contains(ua, "PhantomJS") {
		return true
	}
	if crawlerdetect.IsCrawler(ua) {
		return true
	}
	return useragent.Parse(ua).Bot
}

// Client returns the browser, operating system and device type. Device comes from the screen
// width the tracker sends (<576 mobile, <992 tablet), falling back to the user agent.
func Client(ua string, screenWidth int) (browser, os, device string) {
	p := useragent.Parse(ua)
	browser = normalizeBrowser(p.Name)
	os = normalizeOS(p.OS)
	switch {
	case screenWidth > 0 && screenWidth < 576:
		device = "Mobile"
	case screenWidth > 0 && screenWidth < 992:
		device = "Tablet"
	case screenWidth > 0:
		device = "Desktop"
	case p.Tablet:
		device = "Tablet"
	case p.Mobile:
		device = "Mobile"
	case p.Desktop:
		device = "Desktop"
	}
	return browser, os, device
}

func normalizeBrowser(name string) string {
	switch name {
	case "":
		return ""
	case useragent.MobileSafari:
		return "Safari"
	case useragent.OperaMini, useragent.OperaTouch:
		return "Opera"
	case useragent.Msie:
		return useragent.InternetExplorer
	}
	return clip(name, 50)
}

func normalizeOS(name string) string {
	switch name {
	case useragent.WindowsNT:
		return useragent.Windows
	case useragent.CrOS:
		return useragent.ChromeOS
	case useragent.WindowsPhoneOS:
		return useragent.WindowsPhone
	}
	return clip(name, 50)
}
