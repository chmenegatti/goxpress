package goxpress

import (
	"net/http"
	"strings"
)

// allMethods lists the HTTP methods that Mount registers a delegating route
// for.
var allMethods = []string{
	http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch,
	http.MethodDelete, http.MethodHead, http.MethodOptions,
}

// RouteGroup registers routes that share a common path prefix and a common set
// of middleware. Groups can be nested to compose prefixes and middleware.
type RouteGroup struct {
	prefix     string
	middleware []HandlerFunc
	router     *Router
}

// Group creates a route group rooted at prefix with optional group-scoped
// middleware. The prefix must begin with '/'.
func (r *Router) Group(prefix string, middleware ...HandlerFunc) *RouteGroup {
	if len(prefix) == 0 || prefix[0] != '/' {
		panic("goxpress: group prefix must begin with '/', got '" + prefix + "'")
	}
	return &RouteGroup{
		prefix:     strings.TrimSuffix(prefix, "/"),
		middleware: middleware,
		router:     r,
	}
}

// Group creates a nested group, concatenating prefixes and appending
// middleware to those inherited from the parent group.
func (g *RouteGroup) Group(prefix string, middleware ...HandlerFunc) *RouteGroup {
	if len(prefix) == 0 || prefix[0] != '/' {
		panic("goxpress: group prefix must begin with '/', got '" + prefix + "'")
	}
	mw := make([]HandlerFunc, 0, len(g.middleware)+len(middleware))
	mw = append(mw, g.middleware...)
	mw = append(mw, middleware...)
	return &RouteGroup{
		prefix:     g.prefix + strings.TrimSuffix(prefix, "/"),
		middleware: mw,
		router:     g.router,
	}
}

// Use adds middleware to the group. It applies only to routes registered on
// the group afterwards.
func (g *RouteGroup) Use(middleware ...HandlerFunc) {
	g.middleware = append(g.middleware, middleware...)
}

// Handle registers handlers for method and path within the group, prefixing
// the path and prepending the group's middleware.
func (g *RouteGroup) Handle(method, path string, handlers ...HandlerFunc) *Route {
	full := joinPaths(g.prefix, path)
	chain := make([]HandlerFunc, 0, len(g.middleware)+len(handlers))
	chain = append(chain, g.middleware...)
	chain = append(chain, handlers...)
	return g.router.Handle(method, full, chain...)
}

// Get registers handlers for the GET method within the group.
func (g *RouteGroup) Get(path string, handlers ...HandlerFunc) *Route {
	return g.Handle(http.MethodGet, path, handlers...)
}

// Post registers handlers for the POST method within the group.
func (g *RouteGroup) Post(path string, handlers ...HandlerFunc) *Route {
	return g.Handle(http.MethodPost, path, handlers...)
}

// Put registers handlers for the PUT method within the group.
func (g *RouteGroup) Put(path string, handlers ...HandlerFunc) *Route {
	return g.Handle(http.MethodPut, path, handlers...)
}

// Patch registers handlers for the PATCH method within the group.
func (g *RouteGroup) Patch(path string, handlers ...HandlerFunc) *Route {
	return g.Handle(http.MethodPatch, path, handlers...)
}

// Delete registers handlers for the DELETE method within the group.
func (g *RouteGroup) Delete(path string, handlers ...HandlerFunc) *Route {
	return g.Handle(http.MethodDelete, path, handlers...)
}

// Head registers handlers for the HEAD method within the group.
func (g *RouteGroup) Head(path string, handlers ...HandlerFunc) *Route {
	return g.Handle(http.MethodHead, path, handlers...)
}

// Options registers handlers for the OPTIONS method within the group.
func (g *RouteGroup) Options(path string, handlers ...HandlerFunc) *Route {
	return g.Handle(http.MethodOptions, path, handlers...)
}

// Mount attaches an http.Handler (commonly another *Router) under prefix. All
// requests whose path begins with prefix are delegated to the handler with the
// prefix stripped from the request path, mirroring chi's Mount.
//
// Global middleware registered on the parent router still runs before the
// mounted handler.
func (r *Router) Mount(prefix string, handler http.Handler) {
	prefix = strings.TrimSuffix(prefix, "/")
	if prefix == "" {
		panic("goxpress: mount prefix must not be empty or '/'")
	}
	if prefix[0] != '/' {
		panic("goxpress: mount prefix must begin with '/', got '" + prefix + "'")
	}

	delegate := func(c *Context) error {
		original := c.Request.URL.Path
		stripped := strings.TrimPrefix(original, prefix)
		if stripped == "" {
			stripped = "/"
		}
		c.Request.URL.Path = stripped
		handler.ServeHTTP(c.Writer, c.Request)
		c.Request.URL.Path = original
		return nil
	}

	for _, m := range allMethods {
		// Hide the delegating routes from the OpenAPI document: they are an
		// implementation detail of mounting, not part of the public API surface.
		r.Handle(m, prefix, delegate).Hide()
		r.Handle(m, prefix+"/*mountpath", delegate).Hide()
	}
}

// joinPaths concatenates a group prefix and a route path into a single clean
// path with exactly one separating slash.
func joinPaths(prefix, path string) string {
	if path == "" || path == "/" {
		if prefix == "" {
			return "/"
		}
		return prefix
	}
	return strings.TrimSuffix(prefix, "/") + "/" + strings.TrimPrefix(path, "/")
}
