package sites

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/Gitax18/tally/internal/testdb"
)

func TestNormalizeDomain(t *testing.T) {
	ok := map[string]string{
		"example.com":                          "example.com",
		"  Example.COM ":                       "example.com",
		"https://www.example.com/blog?x=1#top": "example.com",
		"http://blog.example.co.uk:8080/":      "blog.example.co.uk",
		"example.com.":                         "example.com",
		"localhost":                            "localhost",
		"http://localhost:3000":                "localhost",
		"xn--bcher-kva.example":                "xn--bcher-kva.example",
	}
	for in, want := range ok {
		got, err := NormalizeDomain(in)
		if err != nil || got != want {
			t.Errorf("NormalizeDomain(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "example", "-bad.com", "bad-.com", "a..com", "exa mple.com", "bücher.example", "ex_ample.com"} {
		if _, err := NormalizeDomain(bad); err == nil {
			t.Errorf("NormalizeDomain(%q) should fail", bad)
		}
	}
}

func TestHostAllowed(t *testing.T) {
	s := Site{Domain: "example.com"}
	for host, want := range map[string]bool{
		"example.com":         true,
		"www.example.com":     true,
		"blog.example.com":    true,
		"EXAMPLE.com.":        true,
		"notexample.com":      false,
		"example.com.evil.io": false,
		"localhost":           false,
	} {
		if got := s.HostAllowed(host); got != want {
			t.Errorf("HostAllowed(%q) = %v, want %v", host, got, want)
		}
	}
	local := Site{Domain: "localhost"}
	if !local.HostAllowed("localhost") || !local.HostAllowed("127.0.0.1") || local.HostAllowed("example.com") {
		t.Error("a localhost site should accept loopback hosts only")
	}
}

func TestNewID(t *testing.T) {
	seen := map[string]bool{}
	for range 1000 {
		id := NewID()
		if !ValidID(id) {
			t.Fatalf("invalid id %q", id)
		}
		seen[id] = true
	}
	if len(seen) != 1000 {
		t.Fatal("IDs collided")
	}
}

func TestStoreCRUD(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	s := NewStore(pool, slog.New(slog.NewTextHandler(io.Discard, nil)))

	site, err := s.Create(ctx, "Blog", "https://www.Example.com/", "Asia/Kolkata")
	if err != nil {
		t.Fatal(err)
	}
	if site.Domain != "example.com" || site.Timezone != "Asia/Kolkata" || !ValidID(site.ID) {
		t.Fatalf("unexpected site %+v", site)
	}
	if _, ok := s.Lookup(site.ID); !ok {
		t.Fatal("a created site should be in the cache at once")
	}

	var verr *ValidationError
	if _, err := s.Create(ctx, "X", "example.com", "Mars/Olympus"); !errors.As(err, &verr) || verr.Field != "timezone" {
		t.Fatalf("want a timezone validation error, got %v", err)
	}

	name := "My blog"
	updated, err := s.Update(ctx, site.ID, Patch{Name: &name})
	if err != nil || updated.Name != "My blog" || updated.Domain != "example.com" {
		t.Fatalf("update: %+v %v", updated, err)
	}
	if cached, _ := s.Lookup(site.ID); cached.Name != "My blog" {
		t.Fatal("the cache was not updated")
	}

	list, err := s.List(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %v %v", list, err)
	}

	if err := s.Delete(ctx, site.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, site.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	if _, ok := s.Lookup(site.ID); ok {
		t.Fatal("a deleted site should leave the cache at once")
	}
	if err := s.Delete(ctx, site.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleting twice: want ErrNotFound, got %v", err)
	}
}
