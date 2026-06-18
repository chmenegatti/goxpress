package goxpress

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strings"
)

// Static serves the files under the local directory root beneath the URL
// prefix. It is a convenience wrapper over StaticFS using os.DirFS(root).
//
//	app.Static("/assets", "./public")
//
// A request for /assets/css/app.css is served from ./public/css/app.css.
func (r *Router) Static(prefix, root string) {
	r.StaticFS(prefix, os.DirFS(root))
}

// StaticFS serves the files of fsys beneath the URL prefix, registering a
// catch-all GET (and, implicitly, HEAD) route. Path traversal is prevented by
// http.FileServer, which rejects paths escaping the filesystem root; missing
// files yield 404 and content types are detected automatically.
func (r *Router) StaticFS(prefix string, fsys fs.FS) {
	if len(prefix) == 0 || prefix[0] != '/' {
		panic("goxpress: static prefix must begin with '/', got '" + prefix + "'")
	}

	fileServer := http.StripPrefix(strings.TrimRight(prefix, "/"), http.FileServer(http.FS(fsys)))
	handler := func(c *Context) error {
		fileServer.ServeHTTP(c.Writer, c.Request)
		return nil
	}

	r.Get(strings.TrimRight(prefix, "/")+"/*filepath", handler)
}

// File registers a GET route at path that serves the single local file at
// filepath.
//
//	app.File("/favicon.ico", "./public/favicon.ico")
func (r *Router) File(path, filepath string) {
	r.Get(path, func(c *Context) error {
		return c.File(filepath)
	})
}

// File serves the local file at filepath, detecting its content type and
// honoring conditional and range requests via http.ServeFile. It responds with
// 404 when the file does not exist.
func (c *Context) File(filepath string) error {
	http.ServeFile(c.Writer, c.Request, filepath)
	return nil
}

// FileFromFS serves filepath from fsys, mirroring File for an arbitrary
// filesystem such as an embed.FS or an in-memory fstest.MapFS.
func (c *Context) FileFromFS(filepath string, fsys fs.FS) error {
	http.ServeFileFS(c.Writer, c.Request, fsys, filepath)
	return nil
}

// Attachment serves the local file at filepath as a download, setting the
// Content-Disposition header so the browser saves it under name.
func (c *Context) Attachment(filepath, name string) error {
	c.SetHeader("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
	return c.File(filepath)
}
