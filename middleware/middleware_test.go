package middleware_test

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chmenegatti/goxpress"
	"github.com/chmenegatti/goxpress/middleware"
)

func serve(r *goxpress.Router, req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestRequestID(t *testing.T) {
	r := goxpress.New()
	r.Use(middleware.RequestID())
	var seen string
	r.Get("/", func(c *goxpress.Context) error {
		seen = middleware.GetRequestID(c)
		return nil
	})

	w := serve(r, httptest.NewRequest(http.MethodGet, "/", nil))
	if seen == "" {
		t.Error("request ID not stored on context")
	}
	if w.Header().Get(middleware.RequestIDHeader) != seen {
		t.Errorf("response header = %q, want %q", w.Header().Get(middleware.RequestIDHeader), seen)
	}
}

func TestRequestIDReusesIncoming(t *testing.T) {
	r := goxpress.New()
	r.Use(middleware.RequestID())
	r.Get("/", func(c *goxpress.Context) error { return nil })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(middleware.RequestIDHeader, "abc-123")
	w := serve(r, req)
	if w.Header().Get(middleware.RequestIDHeader) != "abc-123" {
		t.Errorf("got %q, want incoming id echoed", w.Header().Get(middleware.RequestIDHeader))
	}
}

func TestRealIP(t *testing.T) {
	r := goxpress.New()
	r.Use(middleware.RealIP())
	var got string
	r.Get("/", func(c *goxpress.Context) error { got = c.Request.RemoteAddr; return nil })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.7, 10.0.0.1")
	serve(r, req)
	if got != "203.0.113.7" {
		t.Errorf("RemoteAddr = %q, want %q", got, "203.0.113.7")
	}
}

func TestLogger(t *testing.T) {
	var buf bytes.Buffer
	r := goxpress.New()
	r.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{Output: &buf}))
	r.Get("/hello", func(c *goxpress.Context) error { return c.String(http.StatusOK, "hi") })

	serve(r, httptest.NewRequest(http.MethodGet, "/hello", nil))
	line := buf.String()
	if !strings.Contains(line, "GET") || !strings.Contains(line, "/hello") || !strings.Contains(line, "200") {
		t.Errorf("log line missing fields: %q", line)
	}
}

func TestRecoverer(t *testing.T) {
	var buf bytes.Buffer
	r := goxpress.New()
	r.Recovery = false // ensure the middleware is what recovers
	r.Use(middleware.RecovererWithConfig(middleware.RecovererConfig{Output: &buf, StackTrace: true}))
	r.Get("/panic", func(c *goxpress.Context) error { panic("boom") })

	w := serve(r, httptest.NewRequest(http.MethodGet, "/panic", nil))
	if w.Code != http.StatusInternalServerError {
		t.Errorf("code = %d, want 500", w.Code)
	}
	if !strings.Contains(buf.String(), "boom") {
		t.Errorf("panic not logged: %q", buf.String())
	}
}

func TestCORSPreflight(t *testing.T) {
	r := goxpress.New()
	r.Use(middleware.CORS())
	r.Post("/items", func(c *goxpress.Context) error { return nil })

	req := httptest.NewRequest(http.MethodOptions, "/items", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	w := serve(r, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("preflight code = %d, want 204", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("allow-origin = %q, want *", w.Header().Get("Access-Control-Allow-Origin"))
	}
	if !strings.Contains(w.Header().Get("Access-Control-Allow-Methods"), "POST") {
		t.Errorf("allow-methods = %q", w.Header().Get("Access-Control-Allow-Methods"))
	}
}

func TestCORSAllowListRejectsUnknown(t *testing.T) {
	r := goxpress.New()
	r.Use(middleware.CORSWithConfig(middleware.CORSConfig{AllowOrigins: []string{"https://ok.com"}}))
	r.Get("/", func(c *goxpress.Context) error { return nil })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://evil.com")
	w := serve(r, req)
	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("allow-origin = %q, want empty for disallowed origin", w.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCompress(t *testing.T) {
	r := goxpress.New()
	r.Use(middleware.Compress())
	body := strings.Repeat("goxpress ", 100)
	r.Get("/", func(c *goxpress.Context) error { return c.String(http.StatusOK, "%s", body) })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := serve(r, req)

	if w.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", w.Header().Get("Content-Encoding"))
	}
	gz, err := gzip.NewReader(w.Body)
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	got, _ := io.ReadAll(gz)
	if string(got) != body {
		t.Errorf("decompressed body mismatch")
	}
}

func TestCompressSkippedWithoutAcceptEncoding(t *testing.T) {
	r := goxpress.New()
	r.Use(middleware.Compress())
	r.Get("/", func(c *goxpress.Context) error { return c.String(http.StatusOK, "plain") })

	w := serve(r, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Header().Get("Content-Encoding") == "gzip" {
		t.Error("response gzipped without Accept-Encoding")
	}
	if w.Body.String() != "plain" {
		t.Errorf("body = %q, want plain", w.Body.String())
	}
}

func TestTimeoutSetsDeadline(t *testing.T) {
	r := goxpress.New()
	r.Use(middleware.Timeout(50 * time.Millisecond))
	var hadDeadline bool
	r.Get("/", func(c *goxpress.Context) error {
		_, hadDeadline = c.Request.Context().Deadline()
		return nil
	})

	serve(r, httptest.NewRequest(http.MethodGet, "/", nil))
	if !hadDeadline {
		t.Error("request context has no deadline under Timeout middleware")
	}
}
