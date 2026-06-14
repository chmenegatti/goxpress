package goxpress

import (
	"testing"
)

// fakeHandler returns a handler chain whose single handler is identifiable by
// the closure it captures; tests only check for presence, not behavior.
func fakeHandler() []HandlerFunc {
	return []HandlerFunc{func(*Context) error { return nil }}
}

func TestTreeStaticRoutes(t *testing.T) {
	tree := &node{}
	routes := []string{"/", "/hi", "/contact", "/co", "/c", "/a", "/ab", "/doc/", "/doc/go_faq.html", "/doc/go1.html"}
	for _, r := range routes {
		tree.addRoute(r, fakeHandler())
	}

	for _, r := range routes {
		res := tree.getValue(r, nil)
		if res.handlers == nil {
			t.Errorf("route %q: expected handlers, got none", r)
		}
	}

	if res := tree.getValue("/missing", nil); res.handlers != nil {
		t.Errorf("route /missing: expected no handlers")
	}
}

func TestTreeParamRoutes(t *testing.T) {
	tree := &node{}
	routes := []string{
		"/users/:id",
		"/users/:id/posts/:postID",
		"/files/*filepath",
		"/static/:dir/:file",
	}
	for _, r := range routes {
		tree.addRoute(r, fakeHandler())
	}

	tests := []struct {
		path   string
		want   bool
		params map[string]string
	}{
		{"/users/42", true, map[string]string{"id": "42"}},
		{"/users/42/posts/7", true, map[string]string{"id": "42", "postID": "7"}},
		{"/files/css/app.css", true, map[string]string{"filepath": "/css/app.css"}},
		{"/static/img/logo.png", true, map[string]string{"dir": "img", "file": "logo.png"}},
		{"/users", false, nil},
	}

	for _, tt := range tests {
		res := tree.getValue(tt.path, nil)
		if (res.handlers != nil) != tt.want {
			t.Errorf("path %q: handlers present = %v, want %v", tt.path, res.handlers != nil, tt.want)
			continue
		}
		for k, v := range tt.params {
			if got := res.params.ByName(k); got != v {
				t.Errorf("path %q: param %q = %q, want %q", tt.path, k, got, v)
			}
		}
	}
}

func TestTreeDuplicateRoutePanics(t *testing.T) {
	tree := &node{}
	tree.addRoute("/users/:id", fakeHandler())

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate route registration")
		}
	}()
	tree.addRoute("/users/:id", fakeHandler())
}

func TestTreeWildcardConflictPanics(t *testing.T) {
	tree := &node{}
	tree.addRoute("/users/:id", fakeHandler())

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on conflicting wildcard")
		}
	}()
	tree.addRoute("/users/:name", fakeHandler())
}

func TestTreeTrailingSlashRedirect(t *testing.T) {
	tree := &node{}
	tree.addRoute("/users", fakeHandler())
	tree.addRoute("/posts/", fakeHandler())

	if res := tree.getValue("/users/", nil); res.handlers != nil || !res.tsr {
		t.Errorf("/users/: want tsr without handlers, got handlers=%v tsr=%v", res.handlers != nil, res.tsr)
	}
	if res := tree.getValue("/posts", nil); res.handlers != nil || !res.tsr {
		t.Errorf("/posts: want tsr without handlers, got handlers=%v tsr=%v", res.handlers != nil, res.tsr)
	}
}

func TestLongestCommonPrefix(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"abc", "abd", 2},
		{"abc", "abc", 3},
		{"abc", "xyz", 0},
		{"abcdef", "abc", 3},
	}
	for _, tt := range tests {
		if got := longestCommonPrefix(tt.a, tt.b); got != tt.want {
			t.Errorf("longestCommonPrefix(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestFindWildcard(t *testing.T) {
	tests := []struct {
		path     string
		wildcard string
		idx      int
		valid    bool
	}{
		{"/users/:id", ":id", 7, true},
		{"/files/*path", "*path", 7, true},
		{"/static", "", -1, false},
		{"/bad/:a:b", ":a:b", 5, false},
	}
	for _, tt := range tests {
		w, idx, valid := findWildcard(tt.path)
		if w != tt.wildcard || idx != tt.idx || valid != tt.valid {
			t.Errorf("findWildcard(%q) = (%q, %d, %v), want (%q, %d, %v)",
				tt.path, w, idx, valid, tt.wildcard, tt.idx, tt.valid)
		}
	}
}
