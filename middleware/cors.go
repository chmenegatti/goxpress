package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/chmenegatti/goxpress"
)

// CORSConfig configures the CORS middleware.
type CORSConfig struct {
	// AllowOrigins is the list of permitted origins. The wildcard "*" allows
	// any origin. Defaults to {"*"}.
	AllowOrigins []string
	// AllowMethods is the list of permitted methods for preflight responses.
	AllowMethods []string
	// AllowHeaders is the list of permitted request headers for preflight
	// responses.
	AllowHeaders []string
	// ExposeHeaders is the list of response headers exposed to the browser.
	ExposeHeaders []string
	// AllowCredentials sets Access-Control-Allow-Credentials when true. It is
	// incompatible with the "*" origin, which is then echoed back instead.
	AllowCredentials bool
	// MaxAge is the preflight cache duration in seconds.
	MaxAge int
}

// DefaultCORSConfig is a permissive configuration suitable for development.
var DefaultCORSConfig = CORSConfig{
	AllowOrigins: []string{"*"},
	AllowMethods: []string{
		http.MethodGet, http.MethodPost, http.MethodPut,
		http.MethodPatch, http.MethodDelete, http.MethodHead, http.MethodOptions,
	},
}

// CORS returns CORS middleware with the permissive DefaultCORSConfig.
func CORS() goxpress.HandlerFunc {
	return CORSWithConfig(DefaultCORSConfig)
}

// CORSWithConfig returns CORS middleware using cfg. It sets the appropriate
// Access-Control-* headers and answers preflight (OPTIONS) requests with 204.
func CORSWithConfig(cfg CORSConfig) goxpress.HandlerFunc {
	if len(cfg.AllowOrigins) == 0 {
		cfg.AllowOrigins = DefaultCORSConfig.AllowOrigins
	}
	if len(cfg.AllowMethods) == 0 {
		cfg.AllowMethods = DefaultCORSConfig.AllowMethods
	}
	allowMethods := strings.Join(cfg.AllowMethods, ", ")
	allowHeaders := strings.Join(cfg.AllowHeaders, ", ")
	exposeHeaders := strings.Join(cfg.ExposeHeaders, ", ")
	maxAge := strconv.Itoa(cfg.MaxAge)

	return func(c *goxpress.Context) error {
		origin := c.GetHeader("Origin")
		allowOrigin := resolveOrigin(cfg, origin)

		if allowOrigin != "" {
			c.SetHeader("Access-Control-Allow-Origin", allowOrigin)
			c.SetHeader("Vary", "Origin")
			if cfg.AllowCredentials {
				c.SetHeader("Access-Control-Allow-Credentials", "true")
			}
			if exposeHeaders != "" {
				c.SetHeader("Access-Control-Expose-Headers", exposeHeaders)
			}
		}

		// Preflight request.
		if c.Method() == http.MethodOptions && c.GetHeader("Access-Control-Request-Method") != "" {
			c.SetHeader("Access-Control-Allow-Methods", allowMethods)
			if allowHeaders != "" {
				c.SetHeader("Access-Control-Allow-Headers", allowHeaders)
			} else if reqHeaders := c.GetHeader("Access-Control-Request-Headers"); reqHeaders != "" {
				c.SetHeader("Access-Control-Allow-Headers", reqHeaders)
			}
			if cfg.MaxAge > 0 {
				c.SetHeader("Access-Control-Max-Age", maxAge)
			}
			c.AbortWithStatus(http.StatusNoContent)
			return nil
		}

		return c.Next()
	}
}

// resolveOrigin returns the value to send in Access-Control-Allow-Origin for
// the given request origin, or "" when the origin is not permitted.
func resolveOrigin(cfg CORSConfig, origin string) string {
	if origin == "" {
		return ""
	}
	for _, o := range cfg.AllowOrigins {
		if o == "*" {
			// "*" cannot be combined with credentials; echo the origin instead.
			if cfg.AllowCredentials {
				return origin
			}
			return "*"
		}
		if strings.EqualFold(o, origin) {
			return origin
		}
	}
	return ""
}
