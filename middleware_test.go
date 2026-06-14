package goxpress

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUseGlobalMiddlewareOrder(t *testing.T) {
	r := New()
	var trace []string

	r.Use(func(c *Context) error {
		trace = append(trace, "m1-in")
		err := c.Next()
		trace = append(trace, "m1-out")
		return err
	})
	r.Use(func(c *Context) error {
		trace = append(trace, "m2-in")
		err := c.Next()
		trace = append(trace, "m2-out")
		return err
	})
	r.Get("/", func(c *Context) error {
		trace = append(trace, "handler")
		return nil
	})

	serve(r, http.MethodGet, "/")

	want := []string{"m1-in", "m2-in", "handler", "m2-out", "m1-out"}
	if len(trace) != len(want) {
		t.Fatalf("trace = %v, want %v", trace, want)
	}
	for i := range want {
		if trace[i] != want[i] {
			t.Fatalf("trace = %v, want %v", trace, want)
		}
	}
}

func TestMiddlewareAbortStopsChain(t *testing.T) {
	r := New()
	reached := false

	r.Use(func(c *Context) error {
		c.AbortWithStatus(http.StatusUnauthorized)
		return nil
	})
	r.Get("/", func(c *Context) error {
		reached = true
		return nil
	})

	w := serve(r, http.MethodGet, "/")
	if reached {
		t.Error("handler ran after Abort")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("code = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestMiddlewareErrorPropagation(t *testing.T) {
	r := New()
	r.Use(func(c *Context) error { return c.Next() })
	r.Get("/", func(c *Context) error { return http.ErrAbortHandler })

	w := serve(r, http.MethodGet, "/")
	if w.Code != http.StatusInternalServerError {
		t.Errorf("code = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestUseAfterRouteDoesNotApply(t *testing.T) {
	r := New()
	ran := false
	r.Get("/", func(c *Context) error { return nil })
	r.Use(func(c *Context) error { ran = true; return c.Next() })

	serve(r, http.MethodGet, "/")
	if ran {
		t.Error("middleware registered after route should not apply")
	}
}

func TestMiddlewareRunsForNotFound(t *testing.T) {
	r := New()
	ran := false
	r.Use(func(c *Context) error { ran = true; return c.Next() })

	serve(r, http.MethodGet, "/missing")
	if !ran {
		t.Error("global middleware should run for unmatched routes")
	}
}

func TestWrapHAndWrapF(t *testing.T) {
	r := New()
	r.Get("/h", WrapH(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("from-h"))
	})))
	r.Get("/f", WrapF(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("from-f"))
	}))

	if got := serve(r, http.MethodGet, "/h").Body.String(); got != "from-h" {
		t.Errorf("WrapH body = %q, want %q", got, "from-h")
	}
	if got := serve(r, http.MethodGet, "/f").Body.String(); got != "from-f" {
		t.Errorf("WrapF body = %q, want %q", got, "from-f")
	}
}

func TestFromStdMiddleware(t *testing.T) {
	r := New()
	std := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("X-Std", "yes")
			next.ServeHTTP(w, req)
		})
	}
	r.Use(FromStd(std))
	r.Get("/", func(c *Context) error { return c.String(http.StatusOK, "ok") })

	w := serve(r, http.MethodGet, "/")
	if w.Header().Get("X-Std") != "yes" {
		t.Error("standard middleware header not applied")
	}
	if w.Body.String() != "ok" {
		t.Errorf("body = %q, want %q", w.Body.String(), "ok")
	}
}

func BenchmarkRouterWithMiddleware(b *testing.B) {
	r := New()
	for range 3 {
		r.Use(func(c *Context) error { return c.Next() })
	}
	r.Get("/users/:id", func(c *Context) error { return nil })
	req := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	w := httptest.NewRecorder()

	b.ReportAllocs()
	for b.Loop() {
		r.ServeHTTP(w, req)
	}
}
