package goxpress

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newContext builds a Context wired to a recorder and request, as the router
// would, for unit-testing the helpers in isolation.
func newContext(method, target string, body string) (*Context, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	w := httptest.NewRecorder()
	c := &Context{}
	c.reset(w, req)
	return c, w
}

func TestContextQuery(t *testing.T) {
	c, _ := newContext(http.MethodGet, "/search?q=go&tag=a&tag=b", "")

	if got := c.Query("q"); got != "go" {
		t.Errorf("Query(q) = %q, want %q", got, "go")
	}
	if got := c.QueryDefault("missing", "def"); got != "def" {
		t.Errorf("QueryDefault(missing) = %q, want %q", got, "def")
	}
	if got := c.QueryArray("tag"); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("QueryArray(tag) = %v, want [a b]", got)
	}
}

func TestContextPostForm(t *testing.T) {
	c, _ := newContext(http.MethodPost, "/submit", "name=goxpress&empty=")
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if got := c.PostForm("name"); got != "goxpress" {
		t.Errorf("PostForm(name) = %q, want %q", got, "goxpress")
	}
	if got := c.PostFormDefault("missing", "def"); got != "def" {
		t.Errorf("PostFormDefault(missing) = %q, want %q", got, "def")
	}
}

func TestContextJSON(t *testing.T) {
	c, w := newContext(http.MethodGet, "/", "")

	if err := c.JSON(http.StatusCreated, map[string]int{"n": 1}); err != nil {
		t.Fatalf("JSON returned error: %v", err)
	}
	if w.Code != http.StatusCreated {
		t.Errorf("code = %d, want %d", w.Code, http.StatusCreated)
	}
	if ct := w.Header().Get("Content-Type"); ct != MIMEJSON {
		t.Errorf("Content-Type = %q, want %q", ct, MIMEJSON)
	}
	var out map[string]int
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || out["n"] != 1 {
		t.Errorf("body = %q, decode err = %v", w.Body.String(), err)
	}
}

func TestContextString(t *testing.T) {
	c, w := newContext(http.MethodGet, "/", "")

	if err := c.String(http.StatusOK, "hello %s", "world"); err != nil {
		t.Fatalf("String returned error: %v", err)
	}
	if w.Body.String() != "hello world" {
		t.Errorf("body = %q, want %q", w.Body.String(), "hello world")
	}
}

func TestContextStatusAndSize(t *testing.T) {
	c, _ := newContext(http.MethodGet, "/", "")

	c.Status(http.StatusAccepted)
	if !c.Writer.Written() {
		t.Error("Written() = false after Status")
	}
	if c.Writer.Status() != http.StatusAccepted {
		t.Errorf("Status() = %d, want %d", c.Writer.Status(), http.StatusAccepted)
	}

	n, _ := c.Writer.Write([]byte("abc"))
	if n != 3 || c.Writer.Size() != 3 {
		t.Errorf("Size() = %d, want 3", c.Writer.Size())
	}
}

func TestContextRedirect(t *testing.T) {
	c, w := newContext(http.MethodGet, "/old", "")

	if err := c.Redirect(http.StatusFound, "/new"); err != nil {
		t.Fatalf("Redirect returned error: %v", err)
	}
	if w.Code != http.StatusFound || w.Header().Get("Location") != "/new" {
		t.Errorf("code = %d, Location = %q", w.Code, w.Header().Get("Location"))
	}

	if err := c.Redirect(http.StatusOK, "/bad"); err == nil {
		t.Error("expected error for non-3xx redirect code")
	}
}

func TestContextStore(t *testing.T) {
	c, _ := newContext(http.MethodGet, "/", "")

	c.Set("user", "alice")
	if v, ok := c.Get("user"); !ok || v != "alice" {
		t.Errorf("Get(user) = (%v, %v), want (alice, true)", v, ok)
	}
	if got := c.GetString("user"); got != "alice" {
		t.Errorf("GetString(user) = %q, want %q", got, "alice")
	}
	if _, ok := c.Get("missing"); ok {
		t.Error("Get(missing) returned ok = true")
	}
	if c.MustGet("user") != "alice" {
		t.Error("MustGet(user) mismatch")
	}
}

func TestContextMustGetPanics(t *testing.T) {
	c, _ := newContext(http.MethodGet, "/", "")
	defer func() {
		if recover() == nil {
			t.Error("MustGet on missing key did not panic")
		}
	}()
	c.MustGet("missing")
}

func TestContextResetClearsState(t *testing.T) {
	c, _ := newContext(http.MethodGet, "/?a=1", "")
	c.Set("k", "v")
	c.params = append(c.params, Param{Key: "id", Value: "1"})
	_ = c.Query("a")

	c.reset(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/?b=2", nil))

	if _, ok := c.Get("k"); ok {
		t.Error("store not cleared after reset")
	}
	if len(c.params) != 0 {
		t.Error("params not cleared after reset")
	}
	if c.Query("a") != "" || c.Query("b") != "2" {
		t.Error("query cache not refreshed after reset")
	}
}
