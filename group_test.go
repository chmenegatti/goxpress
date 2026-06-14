package goxpress

import (
	"net/http"
	"testing"
)

func TestGroupPrefixAndParams(t *testing.T) {
	r := New()
	v1 := r.Group("/api/v1")
	v1.Get("/users/:id", func(c *Context) error {
		return c.String(http.StatusOK, "user:%s", c.Param("id"))
	})

	w := serve(r, http.MethodGet, "/api/v1/users/7")
	if w.Code != http.StatusOK || w.Body.String() != "user:7" {
		t.Errorf("code=%d body=%q, want 200 user:7", w.Code, w.Body.String())
	}
}

func TestGroupMiddleware(t *testing.T) {
	r := New()
	var trace []string
	r.Use(func(c *Context) error { trace = append(trace, "global"); return c.Next() })

	admin := r.Group("/admin", func(c *Context) error {
		trace = append(trace, "group")
		return c.Next()
	})
	admin.Get("/panel", func(c *Context) error {
		trace = append(trace, "handler")
		return nil
	})

	serve(r, http.MethodGet, "/admin/panel")
	want := []string{"global", "group", "handler"}
	if len(trace) != 3 || trace[0] != want[0] || trace[1] != want[1] || trace[2] != want[2] {
		t.Errorf("trace = %v, want %v", trace, want)
	}
}

func TestNestedGroups(t *testing.T) {
	r := New()
	var order []string
	api := r.Group("/api", func(c *Context) error { order = append(order, "api"); return c.Next() })
	v1 := api.Group("/v1", func(c *Context) error { order = append(order, "v1"); return c.Next() })
	v1.Get("/ping", func(c *Context) error { order = append(order, "ping"); return c.NoContent(http.StatusOK) })

	w := serve(r, http.MethodGet, "/api/v1/ping")
	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", w.Code)
	}
	if len(order) != 3 || order[0] != "api" || order[1] != "v1" || order[2] != "ping" {
		t.Errorf("order = %v, want [api v1 ping]", order)
	}
}

func TestGroupRootPath(t *testing.T) {
	r := New()
	g := r.Group("/shop")
	g.Get("/", func(c *Context) error { return c.String(http.StatusOK, "shop-root") })

	w := serve(r, http.MethodGet, "/shop")
	if w.Code != http.StatusOK || w.Body.String() != "shop-root" {
		t.Errorf("code=%d body=%q, want 200 shop-root", w.Code, w.Body.String())
	}
}

func TestMountSubRouter(t *testing.T) {
	sub := New()
	sub.Get("/users", func(c *Context) error { return c.String(http.StatusOK, "sub-users") })
	sub.Get("/users/:id", func(c *Context) error { return c.String(http.StatusOK, "sub-user-%s", c.Param("id")) })

	r := New()
	var hits int
	r.Use(func(c *Context) error { hits++; return c.Next() })
	r.Mount("/api", sub)

	tests := []struct {
		path string
		body string
	}{
		{"/api/users", "sub-users"},
		{"/api/users/42", "sub-user-42"},
	}
	for _, tt := range tests {
		w := serve(r, http.MethodGet, tt.path)
		if w.Code != http.StatusOK || w.Body.String() != tt.body {
			t.Errorf("%s: code=%d body=%q, want 200 %q", tt.path, w.Code, w.Body.String(), tt.body)
		}
	}
	if hits != len(tests) {
		t.Errorf("parent middleware hits = %d, want %d", hits, len(tests))
	}
}

func TestMountPanicsOnRootPrefix(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("expected panic mounting at '/'")
		}
	}()
	New().Mount("/", New())
}

func TestJoinPaths(t *testing.T) {
	tests := []struct {
		prefix, path, want string
	}{
		{"/v1", "/users", "/v1/users"},
		{"/v1", "/", "/v1"},
		{"/v1", "", "/v1"},
		{"/v1/", "/users", "/v1/users"},
		{"", "/users", "/users"},
	}
	for _, tt := range tests {
		if got := joinPaths(tt.prefix, tt.path); got != tt.want {
			t.Errorf("joinPaths(%q, %q) = %q, want %q", tt.prefix, tt.path, got, tt.want)
		}
	}
}
