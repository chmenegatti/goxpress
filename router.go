package goxpress

import (
	"net/http"
	"strings"
	"sync"
)

// Router is the central goXpress multiplexer. It holds one radix tree per HTTP
// method and implements http.Handler, so it drops directly into the standard
// library and any net/http-compatible server.
//
// A Router is safe for concurrent use once routes have been registered. Routes
// should be declared during setup, before the server starts serving requests.
type Router struct {
	// trees maps an HTTP method to the root of its radix tree.
	trees map[string]*node

	// pool recycles Context values across requests to avoid per-request
	// allocations on the hot path.
	pool sync.Pool

	// maxParams is the largest number of parameters any registered route
	// declares, used to size pooled parameter slices.
	maxParams int

	// RedirectTrailingSlash, when true, replies with a redirect to the
	// trailing-slash variant of a path (or vice versa) when that variant has a
	// registered handler. Enabled by default.
	RedirectTrailingSlash bool

	// NotFound handles requests that match no route. When nil, a plain
	// 404 response is sent.
	NotFound HandlerFunc

	// MethodNotAllowed handles requests whose path exists for other methods
	// but not the requested one. When nil, a plain 405 response is sent.
	MethodNotAllowed HandlerFunc
}

// New creates a Router with sensible defaults.
func New() *Router {
	r := &Router{
		trees:                 make(map[string]*node),
		RedirectTrailingSlash: true,
	}
	r.pool.New = func() any {
		return &Context{params: make(Params, 0, r.maxParams)}
	}
	return r
}

// Handle registers handlers for the given method and path. The final handler
// is the route handler; any preceding entries act as route-scoped middleware
// and run in order before it.
//
// It panics if method or path is empty, if path does not start with '/', or if
// no handler is supplied — all of which are setup-time programming errors.
func (r *Router) Handle(method, path string, handlers ...HandlerFunc) {
	switch {
	case method == "":
		panic("goxpress: HTTP method must not be empty")
	case len(path) == 0 || path[0] != '/':
		panic("goxpress: path must begin with '/', got '" + path + "'")
	case len(handlers) == 0:
		panic("goxpress: at least one handler is required for '" + path + "'")
	}

	root := r.trees[method]
	if root == nil {
		root = &node{}
		r.trees[method] = root
	}
	root.addRoute(path, handlers)

	if c := countParams(path); c > r.maxParams {
		r.maxParams = c
	}
}

// Get registers handlers for the GET method.
func (r *Router) Get(path string, handlers ...HandlerFunc) {
	r.Handle(http.MethodGet, path, handlers...)
}

// Post registers handlers for the POST method.
func (r *Router) Post(path string, handlers ...HandlerFunc) {
	r.Handle(http.MethodPost, path, handlers...)
}

// Put registers handlers for the PUT method.
func (r *Router) Put(path string, handlers ...HandlerFunc) {
	r.Handle(http.MethodPut, path, handlers...)
}

// Patch registers handlers for the PATCH method.
func (r *Router) Patch(path string, handlers ...HandlerFunc) {
	r.Handle(http.MethodPatch, path, handlers...)
}

// Delete registers handlers for the DELETE method.
func (r *Router) Delete(path string, handlers ...HandlerFunc) {
	r.Handle(http.MethodDelete, path, handlers...)
}

// Head registers handlers for the HEAD method.
func (r *Router) Head(path string, handlers ...HandlerFunc) {
	r.Handle(http.MethodHead, path, handlers...)
}

// Options registers handlers for the OPTIONS method.
func (r *Router) Options(path string, handlers ...HandlerFunc) {
	r.Handle(http.MethodOptions, path, handlers...)
}

// ServeHTTP implements http.Handler, routing the request to its matching
// handler chain.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	c := r.pool.Get().(*Context)
	c.reset(w, req)

	r.dispatch(c)

	r.pool.Put(c)
}

// dispatch resolves the route for the current request and runs the matched
// handler chain, falling back to redirect / 404 / 405 handling.
func (r *Router) dispatch(c *Context) {
	method := c.Request.Method
	path := c.Request.URL.Path

	if root := r.trees[method]; root != nil {
		res := root.getValue(path, c.params)
		if res.handlers != nil {
			c.params = res.params
			r.run(c, res.handlers)
			return
		}

		if path != "/" && r.RedirectTrailingSlash && res.tsr {
			redirectTrailingSlash(c.Writer, c.Request)
			return
		}
	}

	// The path may exist for other methods (405) rather than not at all (404).
	if allowed := r.allowedMethods(path, method); len(allowed) > 0 {
		c.Writer.Header().Set("Allow", strings.Join(allowed, ", "))
		r.handleError(c, r.MethodNotAllowed, http.StatusMethodNotAllowed)
		return
	}

	r.handleError(c, r.NotFound, http.StatusNotFound)
}

// run executes a handler chain in order, stopping at the first error. Rich
// middleware flow control (Next/Abort) is layered on in a later stage; for now
// an error aborts the chain and yields a 500 response.
func (r *Router) run(c *Context, handlers []HandlerFunc) {
	for _, h := range handlers {
		if err := h(c); err != nil {
			http.Error(c.Writer, http.StatusText(http.StatusInternalServerError),
				http.StatusInternalServerError)
			return
		}
	}
}

// handleError invokes a user-supplied handler when present, otherwise writes a
// plain status response with the given code.
func (r *Router) handleError(c *Context, h HandlerFunc, code int) {
	if h != nil {
		_ = h(c)
		return
	}
	http.Error(c.Writer, http.StatusText(code), code)
}

// allowedMethods returns the methods, other than the excluded one, that have a
// handler registered for path.
func (r *Router) allowedMethods(path, exclude string) []string {
	var allowed []string
	for method, root := range r.trees {
		if method == exclude {
			continue
		}
		if root.getValue(path, nil).handlers != nil {
			allowed = append(allowed, method)
		}
	}
	return allowed
}

// Listen starts an HTTP server on addr using this router as the handler. It
// blocks until the server stops and returns the resulting error.
func (r *Router) Listen(addr string) error {
	return http.ListenAndServe(addr, r)
}

// redirectTrailingSlash issues a permanent redirect toggling the trailing
// slash on the request path.
func redirectTrailingSlash(w http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	if len(path) > 1 && path[len(path)-1] == '/' {
		path = path[:len(path)-1]
	} else {
		path += "/"
	}

	code := http.StatusMovedPermanently
	if req.Method != http.MethodGet {
		code = http.StatusPermanentRedirect
	}

	u := *req.URL
	u.Path = path
	http.Redirect(w, req, u.String(), code)
}

// countParams returns the number of ':' and '*' wildcard segments in path.
func countParams(path string) int {
	var n int
	for i := 0; i < len(path); i++ {
		if path[i] == ':' || path[i] == '*' {
			n++
		}
	}
	return n
}
