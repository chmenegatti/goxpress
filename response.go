package goxpress

import (
	"io"
	"net/http"
)

// noStatus is the sentinel used before any status code has been written.
const noStatus = 0

// ResponseWriter extends http.ResponseWriter with introspection that
// middleware commonly needs: the status code written, the number of body bytes
// sent, and whether the response has been started.
//
// It also implements Unwrap so the standard http.ResponseController can reach
// the underlying writer for optional capabilities such as Flush and Hijack.
type ResponseWriter interface {
	http.ResponseWriter

	// Status returns the HTTP status code that was written, or 0 if the header
	// has not been written yet.
	Status() int

	// Size returns the number of body bytes written so far.
	Size() int

	// Written reports whether the response header has been written.
	Written() bool

	// Unwrap returns the underlying http.ResponseWriter.
	Unwrap() http.ResponseWriter
}

// responseWriter is the default ResponseWriter implementation. It defers
// writing the status line until the first body write, so handlers may set the
// status via Context.Status before producing output.
type responseWriter struct {
	http.ResponseWriter
	status int
	size   int
	// suppressBody discards body writes while still recording their size,
	// used to serve HEAD requests from a GET handler without sending a body.
	suppressBody bool
}

// reset re-binds the writer to a new underlying http.ResponseWriter for reuse
// from the Context pool.
func (w *responseWriter) reset(rw http.ResponseWriter) {
	w.ResponseWriter = rw
	w.status = noStatus
	w.size = 0
	w.suppressBody = false
}

// WriteHeader records the status code and forwards it to the underlying writer
// exactly once.
func (w *responseWriter) WriteHeader(code int) {
	if w.status != noStatus {
		return
	}
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// Write writes the body, defaulting the status to 200 if none was set, and
// accumulates the byte count.
func (w *responseWriter) Write(b []byte) (int, error) {
	if w.status == noStatus {
		w.WriteHeader(http.StatusOK)
	}
	if w.suppressBody {
		w.size += len(b)
		return len(b), nil
	}
	n, err := w.ResponseWriter.Write(b)
	w.size += n
	return n, err
}

// WriteString writes a string body, mirroring Write. It satisfies the
// io.StringWriter fast path used by some helpers.
func (w *responseWriter) WriteString(s string) (int, error) {
	if w.status == noStatus {
		w.WriteHeader(http.StatusOK)
	}
	if w.suppressBody {
		w.size += len(s)
		return len(s), nil
	}
	n, err := io.WriteString(w.ResponseWriter, s)
	w.size += n
	return n, err
}

func (w *responseWriter) Status() int                 { return w.status }
func (w *responseWriter) Size() int                   { return w.size }
func (w *responseWriter) Written() bool               { return w.status != noStatus }
func (w *responseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
