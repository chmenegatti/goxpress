package goxpress

import "net/http"

// WrapH adapts a standard http.Handler into a goXpress HandlerFunc, so existing
// handlers drop into a goXpress route unchanged.
func WrapH(h http.Handler) HandlerFunc {
	return func(c *Context) error {
		h.ServeHTTP(c.Writer, c.Request)
		return nil
	}
}

// WrapF adapts a standard http.HandlerFunc into a goXpress HandlerFunc.
func WrapF(f http.HandlerFunc) HandlerFunc {
	return func(c *Context) error {
		f(c.Writer, c.Request)
		return nil
	}
}

// FromStd adapts standard net/http middleware (a func(http.Handler) http.Handler)
// into goXpress middleware.
//
// The wrapped middleware runs as usual; when it invokes the next handler, the
// goXpress chain resumes via Context.Next. Any ResponseWriter or Request the
// middleware substitutes (for example a gzip writer) is propagated to the
// downstream handlers.
func FromStd(mw func(http.Handler) http.Handler) HandlerFunc {
	return func(c *Context) error {
		var chainErr error

		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Adopt any writer/request the middleware swapped in, keeping the
			// status/size accounting of the goXpress ResponseWriter.
			if rw, ok := w.(ResponseWriter); ok {
				c.Writer = rw
			} else {
				c.writer.reset(w)
				c.Writer = &c.writer
			}
			c.Request = r
			chainErr = c.Next()
		})

		mw(next).ServeHTTP(c.Writer, c.Request)
		return chainErr
	}
}
