package goxpress

import "strings"

// ParamMatcher reports whether a captured route-parameter value is acceptable.
// A param segment may be constrained by appending "|name" to it, e.g.
// "/users/:id|int"; the named matcher must accept the captured value for the
// route to match, otherwise the request falls through to 404.
type ParamMatcher func(string) bool

// matcherResolver looks a matcher up by name, reporting whether it exists. It
// is supplied by the Router at registration time so constraints resolve against
// both the built-ins and any custom matchers.
type matcherResolver func(name string) (ParamMatcher, bool)

// defaultParamMatchers holds the built-in constraints, available to every
// router without registration.
var defaultParamMatchers = map[string]ParamMatcher{
	"int":   isInt,
	"uint":  isUint,
	"alpha": isAlpha,
	"alnum": isAlnum,
	"slug":  isSlug,
	"uuid":  isUUID,
}

// Param registers a custom route-parameter matcher under name, usable as a
// "|name" constraint on a ":param" segment. It overrides a built-in of the same
// name. Register matchers during setup, before declaring the routes that use
// them.
func (r *Router) Param(name string, matcher ParamMatcher) {
	if r.paramMatchers == nil {
		r.paramMatchers = make(map[string]ParamMatcher)
	}
	r.paramMatchers[name] = matcher
}

// paramMatcher resolves a matcher name against the router's custom matchers
// first, then the built-ins.
func (r *Router) paramMatcher(name string) (ParamMatcher, bool) {
	if r.paramMatchers != nil {
		if m, ok := r.paramMatchers[name]; ok {
			return m, true
		}
	}
	m, ok := defaultParamMatchers[name]
	return m, ok
}

// parseParam splits a wildcard segment (e.g. ":id|int") into its parameter name
// and optional matcher, panicking on an unknown matcher name since that is a
// setup-time programming error.
func parseParam(wildcard string, resolve matcherResolver, fullPath string) (name string, matcher ParamMatcher) {
	body := wildcard[1:] // strip the ':' / '*' marker

	i := strings.IndexByte(body, '|')
	if i < 0 {
		return body, nil
	}

	name = body[:i]
	cname := body[i+1:]

	var ok bool
	if resolve != nil {
		matcher, ok = resolve(cname)
	}
	if !ok {
		matcher, ok = defaultParamMatchers[cname]
	}
	if !ok {
		panic("goxpress: unknown route param matcher '" + cname + "' in path '" + fullPath + "'")
	}
	return name, matcher
}

// paramName strips an optional "|constraint" suffix from a parameter name,
// returning just the name (used for OpenAPI path templates and parameters).
func paramName(body string) string {
	if i := strings.IndexByte(body, '|'); i >= 0 {
		return body[:i]
	}
	return body
}

// --- built-in matchers ----------------------------------------------------

func isInt(s string) bool {
	if s == "" {
		return false
	}
	if s[0] == '-' || s[0] == '+' {
		s = s[1:]
	}
	return s != "" && allDigits(s)
}

func isUint(s string) bool { return s != "" && allDigits(s) }

func allDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func isAlpha(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') {
			return false
		}
	}
	return true
}

func isAlnum(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') {
			return false
		}
	}
	return true
}

func isSlug(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
			return false
		}
	}
	return true
}

// isUUID accepts the canonical 8-4-4-4-12 hex form, case-insensitively.
func isUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
			continue
		}
		if !isHex(c) {
			return false
		}
	}
	return true
}

func isHex(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}
