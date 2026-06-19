package goxpress

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

// nonFlushWriter is an http.ResponseWriter with no Flush method and no Unwrap,
// so http.ResponseController cannot find a Flusher.
type nonFlushWriter struct{ h http.Header }

func (w *nonFlushWriter) Header() http.Header         { return w.h }
func (w *nonFlushWriter) Write(b []byte) (int, error) { return len(b), nil }
func (w *nonFlushWriter) WriteHeader(int)             {}

func TestContextSimpleAccessors(t *testing.T) {
	c, _ := newContext(http.MethodPatch, "/a/b?x=1", "")
	c.Request.Header.Set("X-Test", "val")

	if c.Method() != http.MethodPatch {
		t.Errorf("Method = %q", c.Method())
	}
	if c.Path() != "/a/b" {
		t.Errorf("Path = %q", c.Path())
	}
	if c.GetHeader("X-Test") != "val" {
		t.Errorf("GetHeader = %q", c.GetHeader("X-Test"))
	}
	if c.QueryDefault("x", "def") != "1" {
		t.Errorf("QueryDefault present = %q", c.QueryDefault("x", "def"))
	}
}

func TestContextCookie(t *testing.T) {
	c, _ := newContext(http.MethodGet, "/", "")
	c.Request.AddCookie(&http.Cookie{Name: "sid", Value: "abc"})

	if v, err := c.Cookie("sid"); err != nil || v != "abc" {
		t.Errorf("Cookie(sid) = %q, %v", v, err)
	}
	if _, err := c.Cookie("missing"); err == nil {
		t.Error("Cookie(missing) want error")
	}
}

func TestContextIsAbortedAndSetHeaderDelete(t *testing.T) {
	c, _ := newContext(http.MethodGet, "/", "")
	if c.IsAborted() {
		t.Error("fresh context reported aborted")
	}
	c.Abort()
	if !c.IsAborted() {
		t.Error("Abort did not mark context aborted")
	}

	c2, w := newContext(http.MethodGet, "/", "")
	c2.SetHeader("X-Gone", "here")
	c2.SetHeader("X-Gone", "") // empty value deletes
	if w.Header().Get("X-Gone") != "" {
		t.Errorf("SetHeader empty did not delete: %q", w.Header().Get("X-Gone"))
	}
}

func TestContextBlob(t *testing.T) {
	c, w := newContext(http.MethodGet, "/", "")
	if err := c.Blob(http.StatusTeapot, "", []byte("raw")); err != nil {
		t.Fatal(err)
	}
	if w.Code != http.StatusTeapot || w.Body.String() != "raw" {
		t.Errorf("Blob = %d %q", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != MIMEBytes {
		t.Errorf("Blob default content type = %q, want %q", ct, MIMEBytes)
	}
}

func TestContextFlushNoFlusher(t *testing.T) {
	c := &Context{}
	c.reset(&nonFlushWriter{h: http.Header{}}, httptest.NewRequest(http.MethodGet, "/", nil))
	if err := c.Flush(); err != nil {
		t.Errorf("Flush on non-flusher writer = %v, want nil", err)
	}
}

func TestBindScalarKinds(t *testing.T) {
	type all struct {
		S    string  `query:"s"`
		B    bool    `query:"b"`
		I    int     `query:"i"`
		U    uint    `query:"u"`
		F    float64 `query:"f"`
		List []int   `query:"list"`
		Skip string  `query:"-"`
	}
	c, _ := newContext(http.MethodGet, "/?s=hi&b=true&i=-5&u=7&f=2.5&list=1&list=2&-=nope", "")
	var v all
	if err := c.BindQuery(&v); err != nil {
		t.Fatalf("BindQuery: %v", err)
	}
	if v.S != "hi" || !v.B || v.I != -5 || v.U != 7 || v.F != 2.5 {
		t.Errorf("scalars = %+v", v)
	}
	if len(v.List) != 2 || v.List[0] != 1 || v.List[1] != 2 {
		t.Errorf("slice = %v", v.List)
	}
	if v.Skip != "" {
		t.Errorf("tag '-' field bound: %q", v.Skip)
	}
}

func TestBindScalarErrors(t *testing.T) {
	cases := []struct {
		query string
		dst   any
	}{
		{"?b=notbool", &struct {
			B bool `query:"b"`
		}{}},
		{"?i=x", &struct {
			I int `query:"i"`
		}{}},
		{"?u=-1", &struct {
			U uint `query:"u"`
		}{}},
		{"?f=nan!", &struct {
			F float64 `query:"f"`
		}{}},
		{"?c=1", &struct {
			C complex128 `query:"c"`
		}{}},
	}
	for _, tc := range cases {
		c, _ := newContext(http.MethodGet, "/"+tc.query, "")
		if err := c.BindQuery(tc.dst); err == nil {
			t.Errorf("BindQuery(%s) want error", tc.query)
		}
	}
}

func TestBindTargetValidation(t *testing.T) {
	c, _ := newContext(http.MethodGet, "/?x=1", "")
	if err := c.BindQuery(struct{}{}); err == nil {
		t.Error("non-pointer target want error")
	}
	notStruct := 0
	if err := c.BindQuery(&notStruct); err == nil {
		t.Error("pointer-to-non-struct want error")
	}
}

func TestBindDispatch(t *testing.T) {
	// JSON
	cj, _ := newContext(http.MethodPost, "/", `{"name":"a"}`)
	cj.Request.Header.Set("Content-Type", "application/json; charset=utf-8")
	var j struct {
		Name string `json:"name"`
	}
	if err := cj.Bind(&j); err != nil || j.Name != "a" {
		t.Errorf("Bind JSON = %+v, %v", j, err)
	}

	// Form
	cf, _ := newContext(http.MethodPost, "/", "name=b")
	cf.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var f struct {
		Name string `form:"name"`
	}
	if err := cf.Bind(&f); err != nil || f.Name != "b" {
		t.Errorf("Bind form = %+v, %v", f, err)
	}

	// Query (no/other content type)
	cq, _ := newContext(http.MethodGet, "/?name=c", "")
	var q struct {
		Name string `query:"name"`
	}
	if err := cq.Bind(&q); err != nil || q.Name != "c" {
		t.Errorf("Bind query = %+v, %v", q, err)
	}
}

func TestBindJSONErrors(t *testing.T) {
	c, _ := newContext(http.MethodPost, "/", "{bad")
	if err := c.BindJSON(&struct{}{}); err == nil {
		t.Error("invalid JSON want error")
	}
	cn, _ := newContext(http.MethodPost, "/", "")
	cn.Request.Body = nil
	if err := cn.BindJSON(&struct{}{}); err == nil {
		t.Error("nil body want error")
	}
}

func TestErrorTypesMessages(t *testing.T) {
	he := NewHTTPError(http.StatusBadRequest, "bad")
	if he.Error() != "goxpress: 400 bad" {
		t.Errorf("HTTPError.Error = %q", he.Error())
	}
	wrapped := he.WithInternal(errors.New("cause"))
	if !strings.Contains(wrapped.Error(), "cause") {
		t.Errorf("wrapped HTTPError.Error = %q", wrapped.Error())
	}
	pe := &PanicError{Value: "boom"}
	if !strings.Contains(pe.Error(), "boom") {
		t.Errorf("PanicError.Error = %q", pe.Error())
	}
	be := &bindError{"x"}
	if be.Error() != "goxpress: x" {
		t.Errorf("bindError.Error = %q", be.Error())
	}
}

func TestDefaultErrorHandlerGeneric(t *testing.T) {
	c, w := newContext(http.MethodGet, "/", "")
	DefaultErrorHandler(c, errors.New("plain"))
	if w.Code != http.StatusInternalServerError {
		t.Errorf("generic error code = %d, want 500", w.Code)
	}

	// Already written: handler writes nothing.
	c2, w2 := newContext(http.MethodGet, "/", "")
	c2.Writer.WriteHeader(http.StatusOK)
	DefaultErrorHandler(c2, NewHTTPError(http.StatusBadRequest))
	if w2.Code != http.StatusOK {
		t.Errorf("written response overwritten: %d", w2.Code)
	}
}

func TestGroupVerbsAndUse(t *testing.T) {
	r := New()
	g := r.Group("/api/")
	g.Use(func(c *Context) error { c.SetHeader("X-G", "1"); return c.Next() })

	verbs := map[string]func(string, ...HandlerFunc) *Route{
		http.MethodGet:     g.Get,
		http.MethodPost:    g.Post,
		http.MethodPut:     g.Put,
		http.MethodPatch:   g.Patch,
		http.MethodDelete:  g.Delete,
		http.MethodHead:    g.Head,
		http.MethodOptions: g.Options,
	}
	for m, reg := range verbs {
		reg("/r", func(c *Context) error { return c.String(http.StatusOK, "%s", m) })
	}

	for m := range verbs {
		w := serve(r, m, "/api/r")
		if w.Code != http.StatusOK {
			t.Errorf("%s /api/r = %d", m, w.Code)
		}
		if w.Header().Get("X-G") != "1" {
			t.Errorf("%s missing group middleware header", m)
		}
	}

	// Nested group inherits prefix + middleware.
	sub := g.Group("/v2", func(c *Context) error { c.SetHeader("X-Sub", "1"); return c.Next() })
	sub.Get("/ping", func(c *Context) error { return c.String(http.StatusOK, "ok") })
	w := serve(r, http.MethodGet, "/api/v2/ping")
	if w.Code != http.StatusOK || w.Header().Get("X-G") != "1" || w.Header().Get("X-Sub") != "1" {
		t.Errorf("nested group = %d %q %q", w.Code, w.Header().Get("X-G"), w.Header().Get("X-Sub"))
	}
}

func TestMount(t *testing.T) {
	sub := New()
	sub.Get("/users", func(c *Context) error { return c.String(http.StatusOK, "users") })
	sub.Get("/", func(c *Context) error { return c.String(http.StatusOK, "subroot") })

	r := New()
	r.Mount("/admin", sub)

	if w := serve(r, http.MethodGet, "/admin/users"); w.Body.String() != "users" {
		t.Errorf("mount sub path = %q", w.Body.String())
	}
	if w := serve(r, http.MethodGet, "/admin"); w.Body.String() != "subroot" {
		t.Errorf("mount root = %q", w.Body.String())
	}
}

func TestMountPanics(t *testing.T) {
	for _, p := range []string{"/", "rel"} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("Mount(%q) did not panic", p)
				}
			}()
			New().Mount(p, New())
		}()
	}
}

func TestFromStdWriterSubstitution(t *testing.T) {
	r := New()
	sink := httptest.NewRecorder()
	ran := false

	// A std middleware that swaps in a plain http.ResponseWriter backed by an
	// independent sink (not a goxpress.ResponseWriter), exercising the reset
	// branch in FromStd.
	r.Use(FromStd(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(&plainWriter{ResponseWriter: sink}, req)
		})
	}))
	r.Get("/", func(c *Context) error {
		ran = true
		return c.String(http.StatusOK, "ok")
	})

	serve(r, http.MethodGet, "/")
	if !ran {
		t.Error("handler did not run after writer substitution")
	}
	if sink.Body.String() != "ok" {
		t.Errorf("substituted writer body = %q, want ok", sink.Body.String())
	}
}

func TestFromStdRequestSubstitution(t *testing.T) {
	r := New()
	var gotVal string
	r.Use(FromStd(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			req.Header.Set("X-Injected", "yes")
			next.ServeHTTP(w, req)
		})
	}))
	r.Get("/", func(c *Context) error {
		gotVal = c.GetHeader("X-Injected")
		return nil
	})

	serve(r, http.MethodGet, "/")
	if gotVal != "yes" {
		t.Errorf("request substitution: header = %q, want yes", gotVal)
	}
}

// plainWriter is an http.ResponseWriter that is not a goxpress.ResponseWriter.
type plainWriter struct{ http.ResponseWriter }

func TestRedirectTrailingSlashNonGet(t *testing.T) {
	r := New()
	r.Post("/items", func(c *Context) error { return nil })
	// POST /items/ should redirect to /items with 308 (permanent redirect).
	w := serve(r, http.MethodPost, "/items/")
	if w.Code != http.StatusPermanentRedirect {
		t.Errorf("trailing-slash POST redirect = %d, want 308", w.Code)
	}
}

func TestParamsGet(t *testing.T) {
	ps := Params{{Key: "a", Value: "1"}}
	if v, ok := ps.Get("a"); !ok || v != "1" {
		t.Errorf("Get(a) = %q %v", v, ok)
	}
	if _, ok := ps.Get("missing"); ok {
		t.Error("Get(missing) reported found")
	}
}

func TestStaticAndFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "app.css"), []byte("body{}"), 0o600); err != nil {
		t.Fatal(err)
	}

	r := New()
	r.Static("/assets", dir)
	r.File("/style", filepath.Join(dir, "app.css"))

	if w := serve(r, http.MethodGet, "/assets/app.css"); w.Code != http.StatusOK || w.Body.String() != "body{}" {
		t.Errorf("Static = %d %q", w.Code, w.Body.String())
	}
	if w := serve(r, http.MethodGet, "/style"); w.Body.String() != "body{}" {
		t.Errorf("File = %q", w.Body.String())
	}
}

func TestStaticFSAndAttachment(t *testing.T) {
	fsys := fstest.MapFS{
		"data.txt": {Data: []byte("hello fs")},
	}

	r := New()
	r.StaticFS("/static/", fsys)
	r.Get("/dl", func(c *Context) error { return c.FileFromFS("data.txt", fsys) })
	r.Get("/att", func(c *Context) error { return c.Attachment(testTempFile(t), "out.txt") })

	if w := serve(r, http.MethodGet, "/static/data.txt"); w.Body.String() != "hello fs" {
		t.Errorf("StaticFS = %q", w.Body.String())
	}
	if w := serve(r, http.MethodGet, "/dl"); w.Body.String() != "hello fs" {
		t.Errorf("FileFromFS = %q", w.Body.String())
	}
	w := serve(r, http.MethodGet, "/att")
	if cd := w.Header().Get("Content-Disposition"); !strings.Contains(cd, `filename="out.txt"`) {
		t.Errorf("Attachment Content-Disposition = %q", cd)
	}
}

func TestStaticFSPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("StaticFS with bad prefix did not panic")
		}
	}()
	New().StaticFS("bad", fstest.MapFS{})
}

func testTempFile(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "att.txt")
	if err := os.WriteFile(p, []byte("download me"), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}
