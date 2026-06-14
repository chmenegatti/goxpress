package middleware

import (
	"net"
	"strings"

	"github.com/chmenegatti/goxpress"
)

// Headers consulted, in order, for the originating client address.
const (
	headerXForwardedFor = "X-Forwarded-For"
	headerXRealIP       = "X-Real-IP"
)

// RealIP rewrites the request's RemoteAddr from the X-Real-IP or
// X-Forwarded-For headers when present, so downstream handlers and logging see
// the originating client address rather than the immediate proxy.
//
// Only enable RealIP when the server sits behind a trusted proxy that sets
// these headers; otherwise clients can spoof their address.
func RealIP() goxpress.HandlerFunc {
	return func(c *goxpress.Context) error {
		if ip := clientIP(c); ip != "" {
			c.Request.RemoteAddr = ip
		}
		return c.Next()
	}
}

// clientIP extracts the first valid client address from the forwarding headers.
func clientIP(c *goxpress.Context) string {
	if xrip := c.GetHeader(headerXRealIP); xrip != "" {
		if ip := net.ParseIP(strings.TrimSpace(xrip)); ip != nil {
			return ip.String()
		}
	}
	if xff := c.GetHeader(headerXForwardedFor); xff != "" {
		// The left-most entry is the original client.
		first, _, _ := strings.Cut(xff, ",")
		if ip := net.ParseIP(strings.TrimSpace(first)); ip != nil {
			return ip.String()
		}
	}
	return ""
}
