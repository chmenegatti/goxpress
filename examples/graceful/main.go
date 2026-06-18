// Command graceful demonstrates goXpress's graceful-shutdown helper:
// ListenAndServe traps SIGINT/SIGTERM and drains in-flight requests before
// exiting. Run it, hit /slow in another terminal, then press Ctrl-C and watch
// the in-flight request finish before the process stops.
package main

import (
	"log"
	"net/http"
	"time"

	"github.com/chmenegatti/goxpress"
	"github.com/chmenegatti/goxpress/middleware"
)

func main() {
	app := goxpress.New()

	// Log one line per request to stdout (method, status, latency, path).
	app.Use(middleware.Logger())

	// Allow up to 15s for in-flight requests to drain on shutdown.
	app.ShutdownTimeout = 15 * time.Second

	app.Get("/", func(c *goxpress.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	app.Get("/slow", func(c *goxpress.Context) error {
		// Simulate slow work; this request keeps running across a shutdown
		// signal and is allowed to complete before the server exits.
		time.Sleep(5 * time.Second)
		return c.String(http.StatusOK, "slow done")
	})

	log.Println("listening on :3200 (Ctrl-C to shut down gracefully)")
	if err := app.ListenAndServe(":3200"); err != nil {
		log.Fatal(err)
	}
	log.Println("server stopped cleanly")
}
