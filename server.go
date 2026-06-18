package goxpress

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// DefaultShutdownTimeout is the drain period ListenAndServe allows for
// in-flight requests to complete after a shutdown signal before giving up.
const DefaultShutdownTimeout = 10 * time.Second

// Server returns an *http.Server configured to use this router as its handler
// and to listen on addr. Callers may tune its timeouts, TLS config and other
// fields before serving.
func (r *Router) Server(addr string) *http.Server {
	return &http.Server{Addr: addr, Handler: r}
}

// ListenAndServe starts an HTTP server on addr and blocks until the process
// receives SIGINT or SIGTERM. On signal it stops accepting new connections and
// gracefully shuts down, draining in-flight requests within ShutdownTimeout
// (or DefaultShutdownTimeout when unset). It returns nil on a clean shutdown.
func (r *Router) ListenAndServe(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return r.serve(ctx, r.Server(addr), ln)
}

// serve runs srv on ln until ctx is cancelled (a shutdown signal) or the server
// stops on its own, then drains in-flight requests within the configured
// timeout. It is the testable core shared by ListenAndServe.
func (r *Router) serve(ctx context.Context, srv *http.Server, ln net.Listener) error {
	r.printBanner(os.Stdout, ln.Addr().String())

	errc := make(chan error, 1)
	go func() {
		err := srv.Serve(ln)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		errc <- err
	}()

	select {
	case err := <-errc:
		// The server stopped before any shutdown signal (e.g. a serve error).
		return err
	case <-ctx.Done():
	}

	timeout := r.ShutdownTimeout
	if timeout <= 0 {
		timeout = DefaultShutdownTimeout
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return srv.Shutdown(shutdownCtx)
}
