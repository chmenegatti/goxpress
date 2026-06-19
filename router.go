package goxpress

import (
	"net/http"
	"os"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"time"
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

	// middleware holds the global middleware applied to every route registered
	// after the middleware was added.
	middleware []HandlerFunc

	// routes records metadata for every registered route, in registration
	// order, for OpenAPI document generation.
	routes []*Route

	// paramMatchers holds custom route-parameter constraint matchers
	// registered via Param, overriding the built-ins of the same name.
	paramMatchers map[string]ParamMatcher

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

	// ErrorHandler renders responses for errors returned by, or panics
	// recovered from, the handler chain. Defaults to DefaultErrorHandler.
	ErrorHandler ErrorHandler

	// Recovery, when true (the default), recovers panics in the handler chain
	// and routes them through ErrorHandler as a *PanicError instead of letting
	// them crash the server.
	Recovery bool

	// ShutdownTimeout bounds how long ListenAndServe waits for in-flight
	// requests to drain on a shutdown signal. When zero, DefaultShutdownTimeout
	// is used.
	ShutdownTimeout time.Duration

	// HandleHEAD, when true (the default), answers HEAD requests that have no
	// explicit HEAD route using the matching GET handler, discarding the body.
	HandleHEAD bool

	// HandleOPTIONS, when true (the default), answers OPTIONS requests that have
	// no explicit OPTIONS route with an empty 204 response whose Allow header
	// lists the methods registered for the path.
	HandleOPTIONS bool

	// Banner, when true (the default), prints a startup banner to standard
	// output when ListenAndServe or Listen begins serving.
	Banner bool

	// Renderer renders named templates for Context.HTML. It is nil by default;
	// set it (for example to a *TemplateRenderer) to enable HTML rendering.
	Renderer Renderer
}

// New creates a Router with sensible defaults.
func New() *Router {
	r := &Router{
		trees:                 make(map[string]*node),
		RedirectTrailingSlash: true,
		ErrorHandler:          DefaultErrorHandler,
		Recovery:              true,
		HandleHEAD:            true,
		HandleOPTIONS:         true,
		Banner:                true,
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
func (r *Router) Handle(method, path string, handlers ...HandlerFunc) *Route {
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
	root.addRoute(path, r.compose(handlers), r.paramMatcher)

	if c := countParams(path); c > r.maxParams {
		r.maxParams = c
	}

	rt := newRoute(method, path)
	r.routes = append(r.routes, rt)
	return rt
}

// Use registers global middleware. Middleware runs, in registration order,
// before the handlers of every route declared afterwards. Calling Use after a
// route has been registered does not affect that route.
func (r *Router) Use(middleware ...HandlerFunc) {
	r.middleware = append(r.middleware, middleware...)
}

// compose returns a fresh chain consisting of the current global middleware
// followed by handlers, so later mutations to either slice cannot alias it.
func (r *Router) compose(handlers []HandlerFunc) []HandlerFunc {
	chain := make([]HandlerFunc, 0, len(r.middleware)+len(handlers))
	chain = append(chain, r.middleware...)
	chain = append(chain, handlers...)
	return chain
}

// Get registers handlers for the GET method and returns the route for optional
// OpenAPI metadata chaining.
func (r *Router) Get(path string, handlers ...HandlerFunc) *Route {
	return r.Handle(http.MethodGet, path, handlers...)
}

// Post registers handlers for the POST method.
func (r *Router) Post(path string, handlers ...HandlerFunc) *Route {
	return r.Handle(http.MethodPost, path, handlers...)
}

// Put registers handlers for the PUT method.
func (r *Router) Put(path string, handlers ...HandlerFunc) *Route {
	return r.Handle(http.MethodPut, path, handlers...)
}

// Patch registers handlers for the PATCH method.
func (r *Router) Patch(path string, handlers ...HandlerFunc) *Route {
	return r.Handle(http.MethodPatch, path, handlers...)
}

// Delete registers handlers for the DELETE method.
func (r *Router) Delete(path string, handlers ...HandlerFunc) *Route {
	return r.Handle(http.MethodDelete, path, handlers...)
}

// Head registers handlers for the HEAD method.
func (r *Router) Head(path string, handlers ...HandlerFunc) *Route {
	return r.Handle(http.MethodHead, path, handlers...)
}

// Options registers handlers for the OPTIONS method.
func (r *Router) Options(path string, handlers ...HandlerFunc) *Route {
	return r.Handle(http.MethodOptions, path, handlers...)
}

// ServeHTTP implements http.Handler, routing the request to its matching
// handler chain.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	c := r.pool.Get().(*Context)
	c.reset(w, req)
	c.renderer = r.Renderer

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
			r.runChain(c, res.handlers)
			return
		}

		if path != "/" && r.RedirectTrailingSlash && res.tsr {
			redirectTrailingSlash(c.Writer, c.Request)
			return
		}
	}

	// Automatic HEAD: answer with the GET handler but discard its body.
	if method == http.MethodHead && r.HandleHEAD {
		if root := r.trees[http.MethodGet]; root != nil {
			res := root.getValue(path, c.params)
			if res.handlers != nil {
				c.params = res.params
				c.writer.suppressBody = true
				r.runChain(c, res.handlers)
				return
			}
		}
	}

	// The path may exist for other methods (405) rather than not at all (404).
	allow := r.allowHeader(path)

	// Automatic OPTIONS: reply with the Allow header and no body. The default
	// handler runs through the global middleware chain so middleware such as
	// CORS can intercept the preflight before it is reached.
	if method == http.MethodOptions && r.HandleOPTIONS && allow != nil {
		allowList := strings.Join(allow, ", ")
		r.runChain(c, r.compose([]HandlerFunc{func(c *Context) error {
			c.Writer.Header().Set("Allow", allowList)
			c.Writer.WriteHeader(http.StatusNoContent)
			return nil
		}}))
		return
	}

	if allow != nil {
		c.Writer.Header().Set("Allow", strings.Join(allow, ", "))
		r.runChain(c, r.compose([]HandlerFunc{r.notFoundHandler(http.StatusMethodNotAllowed, r.MethodNotAllowed)}))
		return
	}

	r.runChain(c, r.compose([]HandlerFunc{r.notFoundHandler(http.StatusNotFound, r.NotFound)}))
}

// runChain executes a composed handler chain through Context.Next. Errors
// returned by the chain, and (when Recovery is enabled) panics raised within
// it, are routed through ErrorHandler.
func (r *Router) runChain(c *Context, handlers []HandlerFunc) {
	c.handlers = handlers
	c.index = -1

	if r.Recovery {
		defer r.recover(c)
	}

	if err := c.Next(); err != nil {
		r.ErrorHandler(c, err)
	}
}

// recover converts a panic in the handler chain into a *PanicError and routes
// it through ErrorHandler. http.ErrAbortHandler is re-raised so the standard
// server can handle connection abortion as usual.
func (r *Router) recover(c *Context) {
	rec := recover()
	if rec == nil {
		return
	}
	if rec == http.ErrAbortHandler {
		panic(rec)
	}
	r.ErrorHandler(c, &PanicError{Value: rec, Stack: debug.Stack()})
}

// notFoundHandler returns the user-supplied handler when set, otherwise a
// handler that writes a plain status response with the given code.
func (r *Router) notFoundHandler(code int, h HandlerFunc) HandlerFunc {
	if h != nil {
		return h
	}
	return func(c *Context) error {
		http.Error(c.Writer, http.StatusText(code), code)
		return nil
	}
}

// allowHeader returns the sorted list of methods to advertise in an Allow
// header for path, including HEAD and OPTIONS when their automatic handling is
// enabled. It returns nil when no route is registered for path under any
// method, letting the caller fall through to a 404.
func (r *Router) allowHeader(path string) []string {
	var methods []string
	for method, root := range r.trees {
		if root.getValue(path, nil).handlers != nil {
			methods = append(methods, method)
		}
	}
	if methods == nil {
		return nil
	}

	if r.HandleHEAD &&
		slices.Contains(methods, http.MethodGet) &&
		!slices.Contains(methods, http.MethodHead) {
		methods = append(methods, http.MethodHead)
	}
	if r.HandleOPTIONS && !slices.Contains(methods, http.MethodOptions) {
		methods = append(methods, http.MethodOptions)
	}

	slices.Sort(methods)
	return methods
}

// Listen starts an HTTP server on addr using this router as the handler. It
// blocks until the server stops and returns the resulting error.
func (r *Router) Listen(addr string) error {
	r.printBanner(os.Stdout, addr)
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
