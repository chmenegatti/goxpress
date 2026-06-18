package goxpress

import (
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestStaticFSServesFile(t *testing.T) {
	fsys := fstest.MapFS{
		"css/app.css": {Data: []byte("body{color:red}")},
	}
	r := New()
	r.StaticFS("/assets", fsys)

	w := serve(r, http.MethodGet, "/assets/css/app.css")
	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", w.Code)
	}
	if w.Body.String() != "body{color:red}" {
		t.Errorf("body = %q", w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/css") {
		t.Errorf("Content-Type = %q, want text/css", ct)
	}
}

func TestStaticFSMissingFile(t *testing.T) {
	r := New()
	r.StaticFS("/assets", fstest.MapFS{"a.txt": {Data: []byte("a")}})

	w := serve(r, http.MethodGet, "/assets/missing.txt")
	if w.Code != http.StatusNotFound {
		t.Errorf("code = %d, want 404", w.Code)
	}
}

func TestStaticFSBlocksTraversal(t *testing.T) {
	// The served filesystem is "public"; "secret.txt" sits outside it and must
	// never be reachable through a traversal attempt.
	full := fstest.MapFS{
		"public/app.css": {Data: []byte("ok")},
		"secret.txt":     {Data: []byte("TOP SECRET")},
	}
	sub, err := fs.Sub(full, "public")
	if err != nil {
		t.Fatal(err)
	}
	r := New()
	r.StaticFS("/assets", sub)

	w := serve(r, http.MethodGet, "/assets/../secret.txt")
	if w.Code == http.StatusOK && strings.Contains(w.Body.String(), "SECRET") {
		t.Fatalf("traversal leaked secret: code=%d body=%q", w.Code, w.Body.String())
	}
}

func TestStaticFSPanicsOnBadPrefix(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("expected panic for prefix without leading slash")
		}
	}()
	New().StaticFS("assets", fstest.MapFS{})
}

func TestFileAndAttachment(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.txt")
	if err := os.WriteFile(path, []byte("hello file"), 0o600); err != nil {
		t.Fatal(err)
	}

	r := New()
	r.File("/report", path)
	r.Get("/download", func(c *Context) error { return c.Attachment(path, "out.txt") })

	w := serve(r, http.MethodGet, "/report")
	if w.Code != http.StatusOK || w.Body.String() != "hello file" {
		t.Errorf("File: code=%d body=%q", w.Code, w.Body.String())
	}

	w = serve(r, http.MethodGet, "/download")
	if w.Body.String() != "hello file" {
		t.Errorf("Attachment body = %q", w.Body.String())
	}
	if cd := w.Header().Get("Content-Disposition"); cd != `attachment; filename="out.txt"` {
		t.Errorf("Content-Disposition = %q", cd)
	}
}

func TestFileFromFS(t *testing.T) {
	fsys := fstest.MapFS{"data.json": {Data: []byte(`{"ok":true}`)}}
	r := New()
	r.Get("/data", func(c *Context) error { return c.FileFromFS("data.json", fsys) })

	w := serve(r, http.MethodGet, "/data")
	if w.Code != http.StatusOK || w.Body.String() != `{"ok":true}` {
		t.Errorf("code=%d body=%q", w.Code, w.Body.String())
	}
}
