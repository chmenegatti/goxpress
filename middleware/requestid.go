package middleware

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/chmenegatti/goxpress"
)

// RequestIDHeader is the header carrying the request identifier.
const RequestIDHeader = "X-Request-ID"

// requestIDKey is the Context store key under which the request ID is saved.
const requestIDKey = "request_id"

// RequestID ensures every request carries a unique identifier. It reuses an
// incoming X-Request-ID header when present, otherwise it generates one. The
// id is echoed in the response header and stored on the Context under the key
// retrievable with GetRequestID.
func RequestID() goxpress.HandlerFunc {
	return func(c *goxpress.Context) error {
		id := c.GetHeader(RequestIDHeader)
		if id == "" {
			id = newRequestID()
		}
		c.Set(requestIDKey, id)
		c.SetHeader(RequestIDHeader, id)
		return c.Next()
	}
}

// GetRequestID returns the request ID stored on the Context by RequestID, or
// the empty string if none was set.
func GetRequestID(c *goxpress.Context) string {
	return c.GetString(requestIDKey)
}

// newRequestID returns a random 128-bit hex identifier.
func newRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand should never fail; fall back to a fixed marker rather
		// than panicking inside request handling.
		return "0000000000000000"
	}
	return hex.EncodeToString(b[:])
}
