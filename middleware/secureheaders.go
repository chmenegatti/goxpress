package middleware

import (
	"strconv"

	"github.com/chmenegatti/goxpress"
)

// SecureHeadersConfig configures the SecureHeaders middleware. A header is
// emitted only when its corresponding field is non-empty (or, for HSTS,
// non-zero), so individual headers can be disabled by leaving them blank.
type SecureHeadersConfig struct {
	// XContentTypeOptions sets X-Content-Type-Options. Defaults to "nosniff".
	XContentTypeOptions string
	// XFrameOptions sets X-Frame-Options. Defaults to "SAMEORIGIN".
	XFrameOptions string
	// ContentSecurityPolicy sets Content-Security-Policy. Disabled by default.
	ContentSecurityPolicy string
	// ReferrerPolicy sets Referrer-Policy. Disabled by default.
	ReferrerPolicy string
	// HSTSMaxAge sets the Strict-Transport-Security max-age, in seconds. When
	// zero the header is omitted. HSTS is sent only on TLS connections, since
	// browsers ignore it over plain HTTP.
	HSTSMaxAge int
	// HSTSIncludeSubdomains appends includeSubDomains to the HSTS header.
	HSTSIncludeSubdomains bool
	// HSTSPreload appends preload to the HSTS header.
	HSTSPreload bool
}

// DefaultSecureHeadersConfig sets the two headers that are safe to apply
// unconditionally to any response.
var DefaultSecureHeadersConfig = SecureHeadersConfig{
	XContentTypeOptions: "nosniff",
	XFrameOptions:       "SAMEORIGIN",
}

// SecureHeaders returns middleware that sets common security response headers
// using DefaultSecureHeadersConfig.
func SecureHeaders() goxpress.HandlerFunc {
	return SecureHeadersWithConfig(DefaultSecureHeadersConfig)
}

// SecureHeadersWithConfig returns security-header middleware using cfg.
func SecureHeadersWithConfig(cfg SecureHeadersConfig) goxpress.HandlerFunc {
	// Precompute the static HSTS value once.
	var hsts string
	if cfg.HSTSMaxAge > 0 {
		hsts = "max-age=" + strconv.Itoa(cfg.HSTSMaxAge)
		if cfg.HSTSIncludeSubdomains {
			hsts += "; includeSubDomains"
		}
		if cfg.HSTSPreload {
			hsts += "; preload"
		}
	}

	return func(c *goxpress.Context) error {
		if cfg.XContentTypeOptions != "" {
			c.SetHeader("X-Content-Type-Options", cfg.XContentTypeOptions)
		}
		if cfg.XFrameOptions != "" {
			c.SetHeader("X-Frame-Options", cfg.XFrameOptions)
		}
		if cfg.ContentSecurityPolicy != "" {
			c.SetHeader("Content-Security-Policy", cfg.ContentSecurityPolicy)
		}
		if cfg.ReferrerPolicy != "" {
			c.SetHeader("Referrer-Policy", cfg.ReferrerPolicy)
		}
		if hsts != "" && c.Request.TLS != nil {
			c.SetHeader("Strict-Transport-Security", hsts)
		}
		return c.Next()
	}
}
