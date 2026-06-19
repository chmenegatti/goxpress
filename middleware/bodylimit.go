package middleware

import (
	"net/http"

	"github.com/chmenegatti/goxpress"
)

// BodyLimit returns middleware that caps the request body at limit bytes.
//
// When the request advertises a Content-Length larger than limit it is
// rejected immediately with 413 Request Entity Too Large. Otherwise the body is
// wrapped with http.MaxBytesReader, so a handler that reads past the limit
// (including bodies sent without a Content-Length) gets an error on Read.
func BodyLimit(limit int64) goxpress.HandlerFunc {
	return func(c *goxpress.Context) error {
		if c.Request.ContentLength > limit {
			return goxpress.NewHTTPError(http.StatusRequestEntityTooLarge)
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		return c.Next()
	}
}
