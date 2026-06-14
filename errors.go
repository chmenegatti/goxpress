package goxpress

import (
	"fmt"
	"net/http"
)

// HTTPError represents an error with an associated HTTP status code. Returning
// one from a handler lets the centralized error handler render an appropriate
// response without the handler writing it directly.
type HTTPError struct {
	// Code is the HTTP status code to send.
	Code int
	// Message is the client-facing message. When empty it defaults to the
	// standard text for Code.
	Message string
	// Internal is an optional underlying error, kept for logging and unwrapping
	// but never exposed to the client.
	Internal error
}

// NewHTTPError creates an HTTPError with the given status code and an optional
// message. When no message is supplied, the standard status text is used.
func NewHTTPError(code int, message ...string) *HTTPError {
	e := &HTTPError{Code: code, Message: http.StatusText(code)}
	if len(message) > 0 {
		e.Message = message[0]
	}
	return e
}

// Error implements the error interface.
func (e *HTTPError) Error() string {
	if e.Internal != nil {
		return fmt.Sprintf("goxpress: %d %s: %v", e.Code, e.Message, e.Internal)
	}
	return fmt.Sprintf("goxpress: %d %s", e.Code, e.Message)
}

// Unwrap returns the internal error, enabling errors.Is/errors.As traversal.
func (e *HTTPError) Unwrap() error { return e.Internal }

// WithInternal returns a copy of the error annotated with an internal cause.
func (e *HTTPError) WithInternal(err error) *HTTPError {
	cp := *e
	cp.Internal = err
	return &cp
}

// PanicError wraps a value recovered from a panic together with the stack trace
// captured at recovery time. The router's recovery converts panics into a
// PanicError and routes it through the error handler.
type PanicError struct {
	// Value is the value passed to panic.
	Value any
	// Stack is the stack trace captured at recovery time.
	Stack []byte
}

// Error implements the error interface.
func (e *PanicError) Error() string {
	return fmt.Sprintf("goxpress: recovered from panic: %v", e.Value)
}

// ErrorHandler renders a response for an error returned by (or a panic
// recovered from) the handler chain. Implementations should check
// c.Writer.Written before writing, since part of the response may already have
// been sent.
type ErrorHandler func(c *Context, err error)

// DefaultErrorHandler is the error handler installed by New. It renders an
// HTTPError with its status code and message as JSON, and any other error as a
// generic 500. It writes nothing if the response has already started.
func DefaultErrorHandler(c *Context, err error) {
	if c.Writer.Written() {
		return
	}

	if he, ok := err.(*HTTPError); ok {
		msg := he.Message
		if msg == "" {
			msg = http.StatusText(he.Code)
		}
		_ = c.JSON(he.Code, map[string]string{"error": msg})
		return
	}

	_ = c.JSON(http.StatusInternalServerError,
		map[string]string{"error": http.StatusText(http.StatusInternalServerError)})
}
