package goxpress

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"
)

func TestHTTPErrorResponse(t *testing.T) {
	r := New()
	r.Get("/teapot", func(c *Context) error {
		return NewHTTPError(http.StatusTeapot, "no coffee here")
	})

	w := serve(r, http.MethodGet, "/teapot")
	if w.Code != http.StatusTeapot {
		t.Errorf("code = %d, want %d", w.Code, http.StatusTeapot)
	}
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["error"] != "no coffee here" {
		t.Errorf("error = %q, want %q", body["error"], "no coffee here")
	}
}

func TestHTTPErrorDefaultMessage(t *testing.T) {
	e := NewHTTPError(http.StatusNotFound)
	if e.Message != http.StatusText(http.StatusNotFound) {
		t.Errorf("Message = %q, want %q", e.Message, http.StatusText(http.StatusNotFound))
	}
}

func TestHTTPErrorUnwrap(t *testing.T) {
	sentinel := errors.New("db down")
	e := NewHTTPError(http.StatusServiceUnavailable).WithInternal(sentinel)
	if !errors.Is(e, sentinel) {
		t.Error("errors.Is did not find the internal cause")
	}
}

func TestGenericErrorYields500(t *testing.T) {
	r := New()
	r.Get("/boom", func(c *Context) error { return errors.New("kaboom") })

	w := serve(r, http.MethodGet, "/boom")
	if w.Code != http.StatusInternalServerError {
		t.Errorf("code = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestPanicRecovery(t *testing.T) {
	r := New()
	r.Get("/panic", func(c *Context) error { panic("unexpected") })

	w := serve(r, http.MethodGet, "/panic")
	if w.Code != http.StatusInternalServerError {
		t.Errorf("code = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestPanicRecoveryRoutesPanicError(t *testing.T) {
	r := New()
	var captured error
	r.ErrorHandler = func(c *Context, err error) {
		captured = err
		c.Writer.WriteHeader(http.StatusInternalServerError)
	}
	r.Get("/panic", func(c *Context) error { panic("boom") })

	serve(r, http.MethodGet, "/panic")

	var pe *PanicError
	if !errors.As(captured, &pe) {
		t.Fatalf("captured error = %v, want *PanicError", captured)
	}
	if pe.Value != "boom" {
		t.Errorf("PanicError.Value = %v, want %q", pe.Value, "boom")
	}
	if len(pe.Stack) == 0 {
		t.Error("PanicError.Stack is empty")
	}
}

func TestRecoveryCanBeDisabled(t *testing.T) {
	r := New()
	r.Recovery = false
	r.Get("/panic", func(c *Context) error { panic("boom") })

	defer func() {
		if recover() == nil {
			t.Error("expected panic to propagate when Recovery is disabled")
		}
	}()
	serve(r, http.MethodGet, "/panic")
}

func TestCustomErrorHandler(t *testing.T) {
	r := New()
	r.ErrorHandler = func(c *Context, err error) {
		_ = c.JSON(http.StatusBadGateway, map[string]string{"custom": err.Error()})
	}
	r.Get("/x", func(c *Context) error { return errors.New("upstream") })

	w := serve(r, http.MethodGet, "/x")
	if w.Code != http.StatusBadGateway {
		t.Errorf("code = %d, want %d", w.Code, http.StatusBadGateway)
	}
}
