package goxpress

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// MIME types used by the response helpers.
const (
	MIMEJSON  = "application/json; charset=utf-8"
	MIMEText  = "text/plain; charset=utf-8"
	MIMEHTML  = "text/html; charset=utf-8"
	MIMEBytes = "application/octet-stream"
)

// HandlerFunc is the signature for goXpress handlers and middleware.
//
// Handlers receive the per-request Context and return an error. A non-nil
// error is forwarded to the router's error handler, removing the repetitive
// "if err != nil { write 500 }" boilerplate from request handling.
type HandlerFunc func(c *Context) error

// Context carries the request-scoped state for a single HTTP request as it
// flows through the matched handler chain.
//
// A Context must not be retained beyond the lifetime of its handler chain: the
// router recycles Contexts through a pool once the response is complete.
type Context struct {
	// Writer is the response writer for the current request.
	Writer ResponseWriter
	// Request is the incoming HTTP request.
	Request *http.Request

	// writer is the pooled concrete ResponseWriter backing Writer.
	writer responseWriter

	// params holds the URL parameters captured for the matched route.
	params Params

	// handlers is the chain (middleware followed by the route handler) being
	// executed for the current request.
	handlers []HandlerFunc

	// index is the position of the currently executing handler within the
	// chain. It starts at -1 before the first Next advances it.
	index int

	// queryCache memoizes the parsed URL query across repeated lookups.
	queryCache url.Values

	// store holds arbitrary request-scoped values shared between handlers.
	store map[string]any
}

// abortIndex is well past any realistic chain length; setting index to it stops
// the chain from advancing. It stays clear of integer overflow so the trailing
// index++ in Next cannot wrap around.
const abortIndex = 1 << 30

// reset prepares a Context for reuse with a new request, clearing any state
// carried over from a previous one. It backs the router's Context pool.
func (c *Context) reset(w http.ResponseWriter, r *http.Request) {
	c.writer.reset(w)
	c.Writer = &c.writer
	c.Request = r
	c.params = c.params[:0]
	c.handlers = nil
	c.index = -1
	c.queryCache = nil
	clear(c.store)
}

// ---------------------------------------------------------------------------
// Middleware flow control
// ---------------------------------------------------------------------------

// Next runs the remaining handlers in the chain. Middleware calls Next to
// invoke downstream handlers and regain control afterwards; code before the
// Next call runs on the way in, code after it runs on the way out.
//
// Execution stops at the first handler that returns a non-nil error, which Next
// returns to its caller and ultimately to the router's error handler.
func (c *Context) Next() error {
	c.index++
	for c.index < len(c.handlers) {
		if err := c.handlers[c.index](c); err != nil {
			return err
		}
		c.index++
	}
	return nil
}

// Abort prevents any not-yet-executed handlers in the chain from running. It
// does not interrupt the current handler, which should return after calling
// Abort.
func (c *Context) Abort() {
	c.index = abortIndex
}

// AbortWithStatus aborts the chain and writes the given status code.
func (c *Context) AbortWithStatus(code int) {
	c.Abort()
	c.Writer.WriteHeader(code)
}

// IsAborted reports whether the chain has been aborted.
func (c *Context) IsAborted() bool {
	return c.index >= abortIndex
}

// ---------------------------------------------------------------------------
// Request data
// ---------------------------------------------------------------------------

// Param returns the value of the URL parameter with the given name, or the
// empty string when the current route declares no such parameter.
func (c *Context) Param(name string) string {
	return c.params.ByName(name)
}

// Query returns the first value of the named URL query parameter, or the empty
// string if it is absent.
func (c *Context) Query(key string) string {
	return c.queryValues().Get(key)
}

// QueryDefault returns the first value of the named URL query parameter, or
// def if it is absent.
func (c *Context) QueryDefault(key, def string) string {
	if v, ok := c.queryValues()[key]; ok && len(v) > 0 {
		return v[0]
	}
	return def
}

// QueryArray returns all values of the named URL query parameter.
func (c *Context) QueryArray(key string) []string {
	return c.queryValues()[key]
}

// queryValues lazily parses and caches the request's URL query.
func (c *Context) queryValues() url.Values {
	if c.queryCache == nil {
		if c.Request != nil && c.Request.URL != nil {
			c.queryCache = c.Request.URL.Query()
		} else {
			c.queryCache = url.Values{}
		}
	}
	return c.queryCache
}

// PostForm returns the first value of the named form field from the request
// body, or the empty string if it is absent.
func (c *Context) PostForm(key string) string {
	return c.PostFormDefault(key, "")
}

// PostFormDefault returns the first value of the named form field, or def if it
// is absent.
func (c *Context) PostFormDefault(key, def string) string {
	if err := c.Request.ParseForm(); err != nil {
		return def
	}
	if v, ok := c.Request.PostForm[key]; ok && len(v) > 0 {
		return v[0]
	}
	return def
}

// GetHeader returns the named request header value.
func (c *Context) GetHeader(key string) string {
	return c.Request.Header.Get(key)
}

// Cookie returns the named cookie's value, or an error if it is not present.
func (c *Context) Cookie(name string) (string, error) {
	ck, err := c.Request.Cookie(name)
	if err != nil {
		return "", err
	}
	return ck.Value, nil
}

// Method returns the request's HTTP method.
func (c *Context) Method() string { return c.Request.Method }

// Path returns the request's URL path.
func (c *Context) Path() string { return c.Request.URL.Path }

// ---------------------------------------------------------------------------
// Response helpers
// ---------------------------------------------------------------------------

// SetHeader sets a response header. An empty value deletes the header.
func (c *Context) SetHeader(key, value string) {
	if value == "" {
		c.Writer.Header().Del(key)
		return
	}
	c.Writer.Header().Set(key, value)
}

// Status writes the HTTP status code and returns the Context for chaining.
func (c *Context) Status(code int) *Context {
	c.Writer.WriteHeader(code)
	return c
}

// JSON serializes v as JSON, sets the Content-Type and writes it with the
// given status code.
func (c *Context) JSON(code int, v any) error {
	c.Writer.Header().Set("Content-Type", MIMEJSON)
	c.Writer.WriteHeader(code)
	return json.NewEncoder(c.Writer).Encode(v)
}

// String writes a formatted plain-text response with the given status code.
func (c *Context) String(code int, format string, args ...any) error {
	c.Writer.Header().Set("Content-Type", MIMEText)
	c.Writer.WriteHeader(code)
	var err error
	if len(args) > 0 {
		_, err = fmt.Fprintf(c.Writer, format, args...)
	} else {
		_, err = c.Writer.Write([]byte(format))
	}
	return err
}

// Blob writes raw bytes with the given status code and content type.
func (c *Context) Blob(code int, contentType string, data []byte) error {
	if contentType == "" {
		contentType = MIMEBytes
	}
	c.Writer.Header().Set("Content-Type", contentType)
	c.Writer.WriteHeader(code)
	_, err := c.Writer.Write(data)
	return err
}

// NoContent writes the status code with an empty body.
func (c *Context) NoContent(code int) error {
	c.Writer.WriteHeader(code)
	return nil
}

// Redirect replies with a redirect to location using the given 3xx status
// code.
func (c *Context) Redirect(code int, location string) error {
	if code < http.StatusMultipleChoices || code > http.StatusPermanentRedirect {
		return fmt.Errorf("goxpress: invalid redirect status code %d", code)
	}
	http.Redirect(c.Writer, c.Request, location, code)
	return nil
}

// ---------------------------------------------------------------------------
// Value store
// ---------------------------------------------------------------------------

// Set stores a request-scoped value under key, lazily allocating the store.
func (c *Context) Set(key string, value any) {
	if c.store == nil {
		c.store = make(map[string]any)
	}
	c.store[key] = value
}

// Get returns the value stored under key and whether it exists.
func (c *Context) Get(key string) (any, bool) {
	v, ok := c.store[key]
	return v, ok
}

// MustGet returns the value stored under key, panicking if it is absent.
func (c *Context) MustGet(key string) any {
	if v, ok := c.store[key]; ok {
		return v
	}
	panic("goxpress: no value stored for key '" + key + "'")
}

// GetString returns the value stored under key as a string, or "" if absent or
// not a string.
func (c *Context) GetString(key string) string {
	v, _ := c.store[key].(string)
	return v
}
