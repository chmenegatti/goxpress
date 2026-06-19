package goxpress

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// FuzzRouterMatch feeds arbitrary paths through a router carrying a mix of
// static, parameter and catch-all routes. The matcher must never panic and must
// always return a valid HTTP status, regardless of input.
func FuzzRouterMatch(f *testing.F) {
	r := New()
	h := func(c *Context) error { return c.String(http.StatusOK, "ok") }
	r.Get("/", h)
	r.Get("/users/:id", h)
	r.Get("/users/:id/posts/:pid", h)
	r.Get("/files/*path", h)
	r.Post("/users", h)

	seeds := []string{"/", "/users/1", "/users/1/posts/2", "/files/a/b.txt", "//", "/users/", "/x/%2e%2e", "/users/:id"}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, path string) {
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		// Build the request directly so the raw fuzz string lands in URL.Path
		// (what the matcher reads) without net/url escape validation rejecting
		// it before it reaches the router.
		req := &http.Request{
			Method: http.MethodGet,
			URL:    &url.URL{Path: path},
			Host:   "example.com",
			Header: make(http.Header),
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code < 100 || w.Code >= 600 {
			t.Fatalf("invalid status %d for path %q", w.Code, path)
		}
	})
}

func TestRouterPatchVerb(t *testing.T) {
	r := New()
	r.Patch("/x", func(c *Context) error { return c.String(http.StatusOK, "patched") })
	if w := serve(r, http.MethodPatch, "/x"); w.Body.String() != "patched" {
		t.Errorf("Patch = %q", w.Body.String())
	}
}

func TestGroupPrefixPanic(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("Group with bad prefix did not panic")
		}
	}()
	New().Group("bad")
}

func TestBindFormMultiValue(t *testing.T) {
	c, _ := newContext(http.MethodPost, "/", "tags=a&tags=b&n=3")
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var v struct {
		Tags []string `form:"tags"`
		N    int      `form:"n"`
	}
	if err := c.BindForm(&v); err != nil {
		t.Fatalf("BindForm: %v", err)
	}
	if len(v.Tags) != 2 || v.N != 3 {
		t.Errorf("bound = %+v", v)
	}
}
