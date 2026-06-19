package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"

	"github.com/chmenegatti/goxpress"
)

// Decompress returns middleware that transparently gunzips request bodies sent
// with Content-Encoding: gzip, so downstream handlers and binders read the
// decoded payload. It is the request-side counterpart to Compress.
//
// On success the Content-Encoding header is removed and Content-Length is reset
// to unknown, since both describe the compressed form. A body that is not valid
// gzip is rejected with 400 Bad Request.
func Decompress() goxpress.HandlerFunc {
	return func(c *goxpress.Context) error {
		if !strings.EqualFold(c.GetHeader("Content-Encoding"), "gzip") {
			return c.Next()
		}

		zr, err := gzip.NewReader(c.Request.Body)
		if err != nil {
			return goxpress.NewHTTPError(http.StatusBadRequest, "invalid gzip request body")
		}

		c.Request.Body = &gzipReadCloser{zr: zr, body: c.Request.Body}
		c.Request.Header.Del("Content-Encoding")
		c.Request.ContentLength = -1
		return c.Next()
	}
}

// gzipReadCloser reads through a gzip.Reader and closes both it and the
// underlying body.
type gzipReadCloser struct {
	zr   *gzip.Reader
	body io.ReadCloser
}

func (g *gzipReadCloser) Read(p []byte) (int, error) { return g.zr.Read(p) }

func (g *gzipReadCloser) Close() error {
	zerr := g.zr.Close()
	berr := g.body.Close()
	if zerr != nil {
		return zerr
	}
	return berr
}
