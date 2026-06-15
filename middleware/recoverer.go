package middleware

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime/debug"

	"github.com/chmenegatti/goxpress"
)

// RecovererConfig configures the Recoverer middleware.
type RecovererConfig struct {
	// Output is where the panic and stack trace are logged. Defaults to
	// os.Stderr.
	Output io.Writer
	// StackTrace controls whether the stack trace is logged. Defaults to true.
	StackTrace bool
}

// Recoverer returns panic-recovery middleware with the default configuration.
//
// goXpress already recovers panics at the router level; Recoverer is for
// applications that want recovery and logging close to their handlers, with a
// configurable destination and stack-trace toggle.
func Recoverer() goxpress.HandlerFunc {
	return RecovererWithConfig(RecovererConfig{StackTrace: true})
}

// RecovererWithConfig returns panic-recovery middleware using cfg.
func RecovererWithConfig(cfg RecovererConfig) goxpress.HandlerFunc {
	if cfg.Output == nil {
		cfg.Output = os.Stderr
	}

	return func(c *goxpress.Context) (err error) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			if rec == http.ErrAbortHandler {
				panic(rec)
			}

			_, _ = fmt.Fprintf(cfg.Output, "[goxpress] panic recovered: %v\n", rec)
			if cfg.StackTrace {
				_, _ = cfg.Output.Write(debug.Stack())
			}

			if !c.Writer.Written() {
				c.Writer.WriteHeader(http.StatusInternalServerError)
			}
			c.Abort()
			err = nil
		}()

		return c.Next()
	}
}
