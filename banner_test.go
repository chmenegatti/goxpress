package goxpress

import (
	"bytes"
	"strings"
	"testing"
)

func TestDisplayURL(t *testing.T) {
	cases := map[string]string{
		":3200":          "http://127.0.0.1:3200",
		"0.0.0.0:8080":   "http://127.0.0.1:8080",
		"127.0.0.1:3000": "http://127.0.0.1:3000",
		"localhost:80":   "http://localhost:80",
		"[::]:3200":      "http://127.0.0.1:3200",
		"unix/socket":    "http://unix/socket",
	}
	for addr, want := range cases {
		if got := displayURL(addr); got != want {
			t.Errorf("displayURL(%q) = %q, want %q", addr, got, want)
		}
	}
}

func TestRouteCount(t *testing.T) {
	r := New()
	if got := r.routeCount(); got != 0 {
		t.Fatalf("empty router routeCount = %d, want 0", got)
	}
	r.Get("/", noopHandler)
	r.Get("/users/:id", noopHandler)
	r.Post("/users", noopHandler)
	if got := r.routeCount(); got != 3 {
		t.Errorf("routeCount = %d, want 3", got)
	}
}

func TestPrintBannerAligned(t *testing.T) {
	r := New()
	r.Get("/", noopHandler)

	var b bytes.Buffer
	r.printBanner(&b, ":3200")

	out := strings.TrimRight(b.String(), "\n")
	lines := strings.Split(out, "\n")
	if len(lines) != 6 {
		t.Fatalf("banner has %d lines, want 6", len(lines))
	}
	for i, line := range lines {
		if n := len([]rune(line)); n != bannerWidth+2 {
			t.Errorf("line %d width = %d runes, want %d: %q", i, n, bannerWidth+2, line)
		}
	}
	if !strings.Contains(out, "goXpress v"+Version) {
		t.Error("banner missing version line")
	}
	if !strings.Contains(out, "http://127.0.0.1:3200") {
		t.Error("banner missing URL line")
	}
}

func TestPrintBannerDisabled(t *testing.T) {
	r := New()
	r.Banner = false

	var b bytes.Buffer
	r.printBanner(&b, ":3200")
	if b.Len() != 0 {
		t.Errorf("disabled banner wrote %q, want nothing", b.String())
	}
}

func noopHandler(*Context) error { return nil }
