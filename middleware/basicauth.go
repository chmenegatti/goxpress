package middleware

import (
	"net/http"
	"strconv"

	"github.com/chmenegatti/goxpress"
)

// BasicAuthConfig configures the BasicAuth middleware.
type BasicAuthConfig struct {
	// Validator authenticates a username/password pair extracted from the
	// Authorization header, returning true when the credentials are valid. It is
	// required. Use crypto/subtle.ConstantTimeCompare inside the validator to
	// guard against timing attacks when comparing against fixed secrets.
	Validator func(user, pass string, c *goxpress.Context) bool
	// Realm is the protection space sent in the WWW-Authenticate challenge.
	// Defaults to "Restricted".
	Realm string
}

// BasicAuth returns HTTP Basic authentication middleware that delegates
// credential checking to validator. Requests without valid credentials receive
// a 401 with a WWW-Authenticate challenge.
func BasicAuth(validator func(user, pass string, c *goxpress.Context) bool) goxpress.HandlerFunc {
	return BasicAuthWithConfig(BasicAuthConfig{Validator: validator})
}

// BasicAuthWithConfig returns HTTP Basic authentication middleware using cfg.
func BasicAuthWithConfig(cfg BasicAuthConfig) goxpress.HandlerFunc {
	if cfg.Validator == nil {
		panic("goxpress/middleware: BasicAuth requires a Validator")
	}
	if cfg.Realm == "" {
		cfg.Realm = "Restricted"
	}
	// strconv.Quote escapes the realm so it is a valid quoted-string token.
	challenge := "Basic realm=" + strconv.Quote(cfg.Realm)

	return func(c *goxpress.Context) error {
		user, pass, ok := c.Request.BasicAuth()
		if ok && cfg.Validator(user, pass, c) {
			return c.Next()
		}
		c.SetHeader("WWW-Authenticate", challenge)
		return goxpress.NewHTTPError(http.StatusUnauthorized)
	}
}
