package middleware

import (
	"context"
	"time"

	"github.com/chmenegatti/goxpress"
)

// Timeout returns middleware that attaches a deadline to the request context.
//
// The timeout is cooperative: handlers that honor c.Request.Context() (for
// example database calls or outbound HTTP requests) are cancelled when the
// deadline passes. It does not forcibly interrupt CPU-bound handlers that
// never check for cancellation.
func Timeout(d time.Duration) goxpress.HandlerFunc {
	return func(c *goxpress.Context) error {
		ctx, cancel := context.WithTimeout(c.Request.Context(), d)
		defer cancel()

		c.Request = c.Request.WithContext(ctx)
		return c.Next()
	}
}
