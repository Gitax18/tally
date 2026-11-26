// Package geo maps an address to a country code with DB-IP's free "IP to Country Lite"
// database (CC BY 4.0, https://db-ip.com; no account or key needed).
//
// The database is downloaded once at boot, in the background. Until it is ready, or when the
// download fails, every lookup returns "" (shown as Unknown). Heroku restarts dynos daily, which
// also keeps the monthly database fresh.
package geo

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/netip"
	"sync/atomic"
	"time"

	"github.com/oschwald/maxminddb-golang/v2"
)

// maxDownloadBytes bounds the decompressed database (the real file is under 10 MB).
const maxDownloadBytes = 64 << 20

// DB looks up countries. The zero value answers "" for every address.
type DB struct {
	reader atomic.Pointer[maxminddb.Reader]
}

// Open loads a local .mmdb file (GEOIP_PATH).
func Open(path string) (*DB, error) {
	r, err := maxminddb.Open(path)
	if err != nil {
		return nil, err
	}
	g := &DB{}
	g.reader.Store(r)
	return g, nil
}

// FromBytes loads a database held in memory.
func FromBytes(b []byte) (*DB, error) {
	r, err := maxminddb.OpenBytes(b)
	if err != nil {
		return nil, err
	}
	g := &DB{}
	g.reader.Store(r)
	return g, nil
}

// Ready reports whether a database is loaded.
func (g *DB) Ready() bool { return g != nil && g.reader.Load() != nil }

// Country returns the ISO 3166-1 alpha-2 code for ip, or "" when unknown.
func (g *DB) Country(ip netip.Addr) string {
	if g == nil || !ip.IsValid() {
		return ""
	}
	r := g.reader.Load()
	if r == nil {
		return ""
	}
	var code string
	if err := r.Lookup(ip).DecodePath(&code, "country", "iso_code"); err != nil || len(code) != 2 {
		return ""
	}
	return code
}

// DownloadInBackground fetches this month's database (or last month's, early in a month before
// the new file is published) and swaps it in when ready. Failures are logged, not fatal.
func (g *DB) DownloadInBackground(ctx context.Context, logger *slog.Logger, client *http.Client) {
	go func() {
		now := time.Now().UTC()
		for _, month := range []time.Time{now, now.AddDate(0, -1, 0)} {
			url := fmt.Sprintf("https://download.db-ip.com/free/dbip-country-lite-%s.mmdb.gz", month.Format("2006-01"))
			b, err := download(ctx, client, url)
			if err != nil {
				logger.Warn("geo: download failed", "url", url, "error", err)
				continue
			}
			r, err := maxminddb.OpenBytes(b)
			if err != nil {
				logger.Warn("geo: database unreadable", "url", url, "error", err)
				continue
			}
			g.reader.Store(r)
			logger.Info("geo: database loaded", "url", url, "bytes", len(b))
			return
		}
		logger.Warn("geo: no database; countries will be Unknown until the next restart")
	}()
}

func download(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "tally-analytics")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	zr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return nil, err
	}
	b, err := io.ReadAll(io.LimitReader(zr, maxDownloadBytes+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxDownloadBytes {
		return nil, errors.New("database is larger than expected")
	}
	return b, nil
}
