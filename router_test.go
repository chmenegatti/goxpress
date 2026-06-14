package goxpress

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// serve runs a single request through the router and returns the recorder.
func serve(r *Router, method, target string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestRouterStaticAndParams(t *testing.T) {
	r := New()
	r.Get("/", func(c *Context) error {
		_, err := c.Writer.Write([]byte("root"))
		return err
	})
	r.Get("/users/:id", func(c *Context) error {
		_, err := c.Writer.Write([]byte("user:" + c.Param("id")))
		return err
	})
	r.Post("/users", func(c *Context) error {
		c.Writer.WriteHeader(http.StatusCreated)
		return nil
	})

	tests := []struct {
		method, path string
		code         int
		body         string
	}{
		{http.MethodGet, "/", http.StatusOK, "root"},
		{http.MethodGet, "/users/99", http.StatusOK, "user:99"},
		{http.MethodPost, "/users", http.StatusCreated, ""},
	}
	for _, tt := range tests {
		w := serve(r, tt.method, tt.path)
		if w.Code != tt.code {
			t.Errorf("%s %s: code = %d, want %d", tt.method, tt.path, w.Code, tt.code)
		}
		if tt.body != "" && w.Body.String() != tt.body {
			t.Errorf("%s %s: body = %q, want %q", tt.method, tt.path, w.Body.String(), tt.body)
		}
	}
}

func TestRouterNotFound(t *testing.T) {
	r := New()
	r.Get("/exists", func(*Context) error { return nil })

	w := serve(r, http.MethodGet, "/missing")
	if w.Code != http.StatusNotFound {
		t.Errorf("code = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestRouterMethodNotAllowed(t *testing.T) {
	r := New()
	r.Get("/resource", func(*Context) error { return nil })
	r.Put("/resource", func(*Context) error { return nil })

	w := serve(r, http.MethodPost, "/resource")
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("code = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
	if allow := w.Header().Get("Allow"); allow == "" {
		t.Error("expected Allow header to be set")
	}
}

func TestRouterTrailingSlashRedirect(t *testing.T) {
	r := New()
	r.Get("/users", func(*Context) error { return nil })

	w := serve(r, http.MethodGet, "/users/")
	if w.Code != http.StatusMovedPermanently {
		t.Errorf("code = %d, want %d", w.Code, http.StatusMovedPermanently)
	}
	if loc := w.Header().Get("Location"); loc != "/users" {
		t.Errorf("Location = %q, want %q", loc, "/users")
	}
}

func TestRouterCustomNotFound(t *testing.T) {
	r := New()
	r.NotFound = func(c *Context) error {
		c.Writer.WriteHeader(http.StatusNotFound)
		_, err := c.Writer.Write([]byte("nope"))
		return err
	}

	w := serve(r, http.MethodGet, "/missing")
	if w.Body.String() != "nope" {
		t.Errorf("body = %q, want %q", w.Body.String(), "nope")
	}
}

func TestRouterHandlerError(t *testing.T) {
	r := New()
	r.Get("/boom", func(*Context) error { return http.ErrAbortHandler })

	w := serve(r, http.MethodGet, "/boom")
	if w.Code != http.StatusInternalServerError {
		t.Errorf("code = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestRouterMiddlewareChainOrder(t *testing.T) {
	r := New()
	var order []string
	r.Get("/chain",
		func(c *Context) error { order = append(order, "a"); return nil },
		func(c *Context) error { order = append(order, "b"); return nil },
	)

	serve(r, http.MethodGet, "/chain")
	if len(order) != 2 || order[0] != "a" || order[1] != "b" {
		t.Errorf("chain order = %v, want [a b]", order)
	}
}

func TestRouterPanicsOnBadRegistration(t *testing.T) {
	tests := []struct {
		name string
		fn   func()
	}{
		{"empty method", func() { New().Handle("", "/x", fakeHandler()...) }},
		{"no leading slash", func() { New().Get("x", func(*Context) error { return nil }) }},
		{"no handler", func() { New().Handle(http.MethodGet, "/x") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Errorf("%s: expected panic", tt.name)
				}
			}()
			tt.fn()
		})
	}
}

func BenchmarkRouterStatic(b *testing.B) {
	r := New()
	r.Get("/users/profile/settings", func(*Context) error { return nil })
	req := httptest.NewRequest(http.MethodGet, "/users/profile/settings", nil)
	w := httptest.NewRecorder()

	b.ReportAllocs()
	for b.Loop() {
		r.ServeHTTP(w, req)
	}
}

func BenchmarkRouterParam(b *testing.B) {
	r := New()
	r.Get("/users/:id/posts/:postID", func(*Context) error { return nil })
	req := httptest.NewRequest(http.MethodGet, "/users/42/posts/7", nil)
	w := httptest.NewRecorder()

	b.ReportAllocs()
	for b.Loop() {
		r.ServeHTTP(w, req)
	}
}
