package middleware

import (
	"compress/gzip"
	"io"
	"strings"
	"sync"

	"github.com/chmenegatti/goxpress"
)

// gzipPool recycles gzip writers across requests.
var gzipPool = sync.Pool{
	New: func() any {
		w, _ := gzip.NewWriterLevel(io.Discard, gzip.DefaultCompression)
		return w
	},
}

// Compress returns middleware that gzip-compresses responses when the client
// advertises gzip support via Accept-Encoding, using the default compression
// level.
func Compress() goxpress.HandlerFunc {
	return func(c *goxpress.Context) error {
		if !strings.Contains(c.GetHeader("Accept-Encoding"), "gzip") {
			return c.Next()
		}

		c.SetHeader("Content-Encoding", "gzip")
		c.SetHeader("Vary", "Accept-Encoding")

		gz := gzipPool.Get().(*gzip.Writer)
		gz.Reset(c.Writer)

		original := c.Writer
		c.Writer = &gzipResponseWriter{ResponseWriter: original, gz: gz}

		err := c.Next()

		// Flush and recycle the gzip writer, then restore the writer so any
		// later code sees the underlying one.
		_ = gz.Close()
		gzipPool.Put(gz)
		c.Writer = original
		return err
	}
}

// gzipResponseWriter wraps a goxpress.ResponseWriter, redirecting body writes
// through a gzip.Writer while preserving the status/size introspection.
type gzipResponseWriter struct {
	goxpress.ResponseWriter
	gz *gzip.Writer
}

// WriteHeader drops any Content-Length (it no longer matches the compressed
// body) before delegating.
func (w *gzipResponseWriter) WriteHeader(code int) {
	w.Header().Del("Content-Length")
	w.ResponseWriter.WriteHeader(code)
}

// Write compresses the body.
func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	if !w.Written() {
		w.WriteHeader(200)
	}
	return w.gz.Write(b)
}
