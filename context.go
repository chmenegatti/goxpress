package goxpress

import "net/http"

// HandlerFunc is the signature for goXpress handlers and middleware.
//
// Handlers receive the per-request Context and return an error. A non-nil
// error is forwarded to the router's error handler, removing the repetitive
// "if err != nil { write 500 }" boilerplate from request handling.
type HandlerFunc func(c *Context) error

// Context carries the request-scoped state for a single HTTP request as it
// flows through the matched handler chain.
//
// This is the minimal form introduced alongside the routing engine; response
// helpers, query/header accessors, a value store and middleware flow control
// are layered on in later stages.
type Context struct {
	// Writer is the response writer for the current request.
	Writer http.ResponseWriter
	// Request is the incoming HTTP request.
	Request *http.Request

	// params holds the URL parameters captured for the matched route.
	params Params
}

// Param returns the value of the URL parameter with the given name, or the
// empty string when the current route declares no such parameter.
func (c *Context) Param(name string) string {
	return c.params.ByName(name)
}

// reset prepares a Context for reuse with a new request, clearing any state
// carried over from a previous one. It backs the router's Context pool.
func (c *Context) reset(w http.ResponseWriter, r *http.Request) {
	c.Writer = w
	c.Request = r
	c.params = c.params[:0]
}
