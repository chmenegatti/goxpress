package goxpress_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chmenegatti/goxpress"
)

func TestSSEvent(t *testing.T) {
	app := goxpress.New()
	app.Get("/stream", func(c *goxpress.Context) error {
		if err := c.SSEvent("greeting", "hello"); err != nil {
			return err
		}
		// Anonymous event with multiline data.
		return c.SSEvent("", "line1\nline2")
	})

	req := httptest.NewRequest(http.MethodGet, "/stream", nil)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("code = %d, want 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != goxpress.MIMEEventStream {
		t.Errorf("Content-Type = %q, want %q", ct, goxpress.MIMEEventStream)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", cc)
	}

	want := "event: greeting\ndata: hello\n\n" +
		"data: line1\ndata: line2\n\n"
	if got := w.Body.String(); got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
}

func TestSSEventFlushed(t *testing.T) {
	app := goxpress.New()
	app.Get("/stream", func(c *goxpress.Context) error {
		return c.SSEvent("tick", "1")
	})

	req := httptest.NewRequest(http.MethodGet, "/stream", nil)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	if !w.Flushed {
		t.Error("SSEvent did not flush the response")
	}
	if !strings.HasPrefix(w.Body.String(), "event: tick\n") {
		t.Errorf("body = %q", w.Body.String())
	}
}
