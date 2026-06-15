package middleware

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/chmenegatti/goxpress"
)

// LoggerConfig configures the Logger middleware.
type LoggerConfig struct {
	// Output is where log lines are written. Defaults to os.Stdout.
	Output io.Writer
	// Now returns the current time; overridable for tests. Defaults to
	// time.Now.
	Now func() time.Time
}

// Logger returns request-logging middleware with the default configuration,
// writing one line per request to standard output.
func Logger() goxpress.HandlerFunc {
	return LoggerWithConfig(LoggerConfig{})
}

// LoggerWithConfig returns request-logging middleware using cfg, applying
// defaults for any zero-valued field.
func LoggerWithConfig(cfg LoggerConfig) goxpress.HandlerFunc {
	if cfg.Output == nil {
		cfg.Output = os.Stdout
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}

	return func(c *goxpress.Context) error {
		start := cfg.Now()
		method := c.Method()
		path := c.Path()

		err := c.Next()

		latency := cfg.Now().Sub(start)
		status := c.Writer.Status()
		if status == 0 {
			status = 200
		}

		_, _ = fmt.Fprintf(cfg.Output, "[goxpress] %s | %3d | %12s | %-7s %s\n",
			start.Format("2006/01/02 - 15:04:05"),
			status,
			latency,
			method,
			path,
		)
		return err
	}
}
