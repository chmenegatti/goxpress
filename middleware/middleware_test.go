package middleware_test

import (
	"bytes"
	"compress/gzip"
	"crypto/tls"
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

func TestBasicAuth(t *testing.T) {
	r := goxpress.New()
	r.Use(middleware.BasicAuth(func(user, pass string, _ *goxpress.Context) bool {
		return user == "admin" && pass == "secret"
	}))
	r.Get("/", func(c *goxpress.Context) error { return c.String(http.StatusOK, "ok") })

	// No credentials: 401 with challenge.
	w := serve(r, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Code != http.StatusUnauthorized {
		t.Errorf("missing creds code = %d, want 401", w.Code)
	}
	if !strings.HasPrefix(w.Header().Get("WWW-Authenticate"), "Basic realm=") {
		t.Errorf("challenge = %q", w.Header().Get("WWW-Authenticate"))
	}

	// Wrong credentials: 401.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetBasicAuth("admin", "wrong")
	if w := serve(r, req); w.Code != http.StatusUnauthorized {
		t.Errorf("bad creds code = %d, want 401", w.Code)
	}

	// Correct credentials: 200.
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetBasicAuth("admin", "secret")
	if w := serve(r, req); w.Code != http.StatusOK {
		t.Errorf("good creds code = %d, want 200", w.Code)
	}
}

func TestSecureHeaders(t *testing.T) {
	r := goxpress.New()
	r.Use(middleware.SecureHeadersWithConfig(middleware.SecureHeadersConfig{
		XContentTypeOptions:   "nosniff",
		XFrameOptions:         "DENY",
		ContentSecurityPolicy: "default-src 'self'",
		ReferrerPolicy:        "no-referrer",
		HSTSMaxAge:            3600,
	}))
	r.Get("/", func(c *goxpress.Context) error { return nil })

	w := serve(r, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q", w.Header().Get("X-Content-Type-Options"))
	}
	if w.Header().Get("X-Frame-Options") != "DENY" {
		t.Errorf("X-Frame-Options = %q", w.Header().Get("X-Frame-Options"))
	}
	if w.Header().Get("Content-Security-Policy") != "default-src 'self'" {
		t.Errorf("CSP = %q", w.Header().Get("Content-Security-Policy"))
	}
	// HSTS must be absent over plain HTTP.
	if w.Header().Get("Strict-Transport-Security") != "" {
		t.Errorf("HSTS sent over non-TLS request")
	}
}

func TestSecureHeadersHSTSOverTLS(t *testing.T) {
	r := goxpress.New()
	r.Use(middleware.SecureHeadersWithConfig(middleware.SecureHeadersConfig{
		HSTSMaxAge:            3600,
		HSTSIncludeSubdomains: true,
	}))
	r.Get("/", func(c *goxpress.Context) error { return nil })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.TLS = &tls.ConnectionState{}
	w := serve(r, req)
	if got := w.Header().Get("Strict-Transport-Security"); got != "max-age=3600; includeSubDomains" {
		t.Errorf("HSTS = %q", got)
	}
}

func TestRateLimit(t *testing.T) {
	r := goxpress.New()
	r.Use(middleware.RateLimitWithConfig(middleware.RateLimitConfig{Rate: 1, Burst: 2}))
	r.Get("/", func(c *goxpress.Context) error { return nil })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.5:1234"

	// Burst of 2 allowed, third denied.
	for i := range 2 {
		if w := serve(r, req); w.Code != http.StatusOK {
			t.Fatalf("request %d code = %d, want 200", i, w.Code)
		}
	}
	w := serve(r, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("over-limit code = %d, want 429", w.Code)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Error("missing Retry-After header on 429")
	}
}

func TestRateLimitPerKey(t *testing.T) {
	r := goxpress.New()
	r.Use(middleware.RateLimitWithConfig(middleware.RateLimitConfig{Rate: 1, Burst: 1}))
	r.Get("/", func(c *goxpress.Context) error { return nil })

	// Two distinct IPs each get their own bucket.
	for _, ip := range []string{"10.0.0.1:1", "10.0.0.2:1"} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = ip
		if w := serve(r, req); w.Code != http.StatusOK {
			t.Errorf("ip %s code = %d, want 200", ip, w.Code)
		}
	}
}

func TestBodyLimitContentLength(t *testing.T) {
	r := goxpress.New()
	r.Use(middleware.BodyLimit(8))
	r.Post("/", func(c *goxpress.Context) error { return nil })

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("way too long body"))
	w := serve(r, req)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("code = %d, want 413", w.Code)
	}
}

func TestBodyLimitReadError(t *testing.T) {
	r := goxpress.New()
	r.Use(middleware.BodyLimit(4))
	var readErr error
	r.Post("/", func(c *goxpress.Context) error {
		_, readErr = io.ReadAll(c.Request.Body)
		return nil
	})

	// ContentLength unknown (-1) bypasses the early check; the read trips the cap.
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("123456789"))
	req.ContentLength = -1
	serve(r, req)
	if readErr == nil {
		t.Error("expected read error past body limit, got nil")
	}
}

func TestDecompress(t *testing.T) {
	r := goxpress.New()
	r.Use(middleware.Decompress())
	var got string
	r.Post("/", func(c *goxpress.Context) error {
		b, _ := io.ReadAll(c.Request.Body)
		got = string(b)
		return nil
	})

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	_, _ = gz.Write([]byte("hello gzip"))
	_ = gz.Close()

	req := httptest.NewRequest(http.MethodPost, "/", &buf)
	req.Header.Set("Content-Encoding", "gzip")
	serve(r, req)
	if got != "hello gzip" {
		t.Errorf("decompressed body = %q, want %q", got, "hello gzip")
	}
}

func TestDecompressInvalidBody(t *testing.T) {
	r := goxpress.New()
	r.Use(middleware.Decompress())
	r.Post("/", func(c *goxpress.Context) error { return nil })

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("not gzip"))
	req.Header.Set("Content-Encoding", "gzip")
	w := serve(r, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400", w.Code)
	}
}

// TestDefaultConstructors exercises the convenience constructors that delegate
// to their *WithConfig counterparts with default configuration.
func TestDefaultConstructors(t *testing.T) {
	r := goxpress.New()
	r.Use(
		middleware.Logger(),
		middleware.Recoverer(),
		middleware.SecureHeaders(),
		middleware.RateLimit(1000),
	)
	r.Get("/", func(c *goxpress.Context) error { return c.String(http.StatusOK, "ok") })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.0.2.1:9999"
	w := serve(r, req)
	if w.Code != http.StatusOK {
		t.Errorf("code = %d, want 200", w.Code)
	}
	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("SecureHeaders default not applied")
	}
}

func TestRealIPVariants(t *testing.T) {
	cases := []struct {
		name   string
		header string
		value  string
		want   string
	}{
		{"x-real-ip", "X-Real-IP", "198.51.100.9", "198.51.100.9"},
		{"x-forwarded-for first", "X-Forwarded-For", "198.51.100.7, 10.0.0.1", "198.51.100.7"},
		{"invalid ignored", "X-Real-IP", "not-an-ip", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := goxpress.New()
			r.Use(middleware.RealIP())
			var got string
			r.Get("/", func(c *goxpress.Context) error { got = c.Request.RemoteAddr; return nil })

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = "10.1.1.1:5000"
			req.Header.Set(tc.header, tc.value)
			serve(r, req)
			if tc.want != "" && got != tc.want {
				t.Errorf("RemoteAddr = %q, want %q", got, tc.want)
			}
			if tc.want == "" && got != "10.1.1.1:5000" {
				t.Errorf("invalid header changed RemoteAddr to %q", got)
			}
		})
	}
}

func TestCORSCredentialsEchoesOrigin(t *testing.T) {
	r := goxpress.New()
	r.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     []string{"*"},
		AllowCredentials: true,
		ExposeHeaders:    []string{"X-Total"},
	}))
	r.Get("/", func(c *goxpress.Context) error { return nil })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://app.example.com")
	w := serve(r, req)
	// With credentials the wildcard is replaced by the echoed origin.
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Errorf("allow-origin = %q, want echoed origin", got)
	}
	if w.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Error("missing allow-credentials")
	}
	if w.Header().Get("Access-Control-Expose-Headers") != "X-Total" {
		t.Error("missing expose-headers")
	}
}

func TestDecompressClosesBody(t *testing.T) {
	r := goxpress.New()
	r.Use(middleware.Decompress())
	r.Post("/", func(c *goxpress.Context) error {
		b, _ := io.ReadAll(c.Request.Body)
		if string(b) != "payload" {
			t.Errorf("body = %q", b)
		}
		// Closing the wrapped body exercises gzipReadCloser.Close.
		return c.Request.Body.Close()
	})

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	_, _ = gz.Write([]byte("payload"))
	_ = gz.Close()

	req := httptest.NewRequest(http.MethodPost, "/", &buf)
	req.Header.Set("Content-Encoding", "gzip")
	if w := serve(r, req); w.Code >= 400 {
		t.Errorf("code = %d", w.Code)
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
