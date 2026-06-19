package goxpress

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBuiltinMatchers(t *testing.T) {
	cases := []struct {
		matcher string
		value   string
		want    bool
	}{
		{"int", "42", true},
		{"int", "-7", true},
		{"int", "+7", true},
		{"int", "", false},
		{"int", "12a", false},
		{"int", "-", false},
		{"uint", "0", true},
		{"uint", "-1", false},
		{"alpha", "abcXYZ", true},
		{"alpha", "abc1", false},
		{"alpha", "", false},
		{"alnum", "abc123", true},
		{"alnum", "abc-1", false},
		{"slug", "my-post-1", true},
		{"slug", "My-Post", false},
		{"uuid", "123e4567-e89b-12d3-a456-426614174000", true},
		{"uuid", "123e4567e89b12d3a456426614174000", false},
		{"uuid", "123e4567-e89b-12d3-a456-42661417400X", false},
	}
	for _, tc := range cases {
		m := defaultParamMatchers[tc.matcher]
		if m == nil {
			t.Fatalf("no built-in matcher %q", tc.matcher)
		}
		if got := m(tc.value); got != tc.want {
			t.Errorf("%s(%q) = %v, want %v", tc.matcher, tc.value, got, tc.want)
		}
	}
}

func TestConstrainedParamRouting(t *testing.T) {
	r := New()
	r.Get("/users/:id|int", func(c *Context) error {
		return c.String(http.StatusOK, "id=%s", c.Param("id"))
	})

	// Valid: digits match.
	if w := serve(r, http.MethodGet, "/users/42"); w.Code != http.StatusOK || w.Body.String() != "id=42" {
		t.Errorf("valid int = %d %q", w.Code, w.Body.String())
	}
	// Invalid: non-digits fall through to 404.
	if w := serve(r, http.MethodGet, "/users/abc"); w.Code != http.StatusNotFound {
		t.Errorf("non-int = %d, want 404", w.Code)
	}
}

func TestConstrainedParamWithSuffix(t *testing.T) {
	r := New()
	r.Get("/users/:id|int/posts", func(c *Context) error {
		return c.String(http.StatusOK, "%s", c.Param("id"))
	})

	if w := serve(r, http.MethodGet, "/users/9/posts"); w.Body.String() != "9" {
		t.Errorf("constrained+suffix = %q", w.Body.String())
	}
	if w := serve(r, http.MethodGet, "/users/x/posts"); w.Code != http.StatusNotFound {
		t.Errorf("invalid constrained+suffix = %d, want 404", w.Code)
	}
}

func TestCustomParamMatcher(t *testing.T) {
	r := New()
	r.Param("even", func(s string) bool {
		return len(s) > 0 && (s[len(s)-1]-'0')%2 == 0
	})
	r.Get("/n/:n|even", func(c *Context) error { return c.String(http.StatusOK, "ok") })

	if w := serve(r, http.MethodGet, "/n/4"); w.Code != http.StatusOK {
		t.Errorf("even value = %d, want 200", w.Code)
	}
	if w := serve(r, http.MethodGet, "/n/3"); w.Code != http.StatusNotFound {
		t.Errorf("odd value = %d, want 404", w.Code)
	}
}

func TestCustomMatcherOverridesBuiltin(t *testing.T) {
	r := New()
	// Override "int" to also reject zero.
	r.Param("int", func(s string) bool { return isInt(s) && s != "0" })
	r.Get("/x/:v|int", func(c *Context) error { return c.String(http.StatusOK, "ok") })

	if w := serve(r, http.MethodGet, "/x/5"); w.Code != http.StatusOK {
		t.Errorf("nonzero = %d", w.Code)
	}
	if w := serve(r, http.MethodGet, "/x/0"); w.Code != http.StatusNotFound {
		t.Errorf("overridden zero = %d, want 404", w.Code)
	}
}

func TestUnknownMatcherPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("unknown matcher did not panic")
		}
	}()
	New().Get("/x/:v|nope", func(c *Context) error { return nil })
}

func TestUnconstrainedParamUnaffected(t *testing.T) {
	r := New()
	r.Get("/users/:id", func(c *Context) error { return c.String(http.StatusOK, "%s", c.Param("id")) })
	if w := serve(r, http.MethodGet, "/users/anything"); w.Body.String() != "anything" {
		t.Errorf("unconstrained param = %q", w.Body.String())
	}
}

func TestConstrainedParamOpenAPIPath(t *testing.T) {
	r := New()
	r.Get("/users/:id|int", func(*Context) error { return nil })

	doc := r.buildOpenAPI()
	if _, ok := doc.Paths["/users/{id}"]; !ok {
		t.Errorf("constraint not stripped from path template: %v", doc.Paths)
	}
	// The path parameter name must be the bare "id".
	op := doc.Paths["/users/{id}"]["get"]
	if len(op.Parameters) != 1 || op.Parameters[0].Name != "id" {
		t.Errorf("path param = %+v", op.Parameters)
	}
}

func BenchmarkRouterParamConstrained(b *testing.B) {
	r := New()
	r.Get("/users/:id|int", func(*Context) error { return nil })
	req := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	w := httptest.NewRecorder()

	b.ReportAllocs()
	for b.Loop() {
		r.ServeHTTP(w, req)
	}
}
