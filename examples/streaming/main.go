// Command streaming demonstrates goXpress streaming responses: Server-Sent
// Events via Context.SSEvent and a WebSocket echo endpoint via the ws
// subpackage.
//
// Try it:
//
//	go run ./examples/streaming
//	curl -N localhost:3000/events            # streams an SSE tick every second
//	# open ws://localhost:3000/ws in a WebSocket client and send a message
package main

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/chmenegatti/goxpress"
	"github.com/chmenegatti/goxpress/ws"
)

func main() {
	app := goxpress.New()

	// Server-Sent Events: stream a tick once per second until the client leaves.
	app.Get("/events", func(c *goxpress.Context) error {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		done := c.Request.Context().Done()
		for n := 1; ; n++ {
			select {
			case <-done:
				return nil
			case <-ticker.C:
				if err := c.SSEvent("tick", strconv.Itoa(n)); err != nil {
					return err
				}
			}
		}
	})

	// WebSocket echo.
	app.Get("/ws", func(c *goxpress.Context) error {
		conn, err := ws.Upgrade(c)
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		for {
			mt, msg, err := conn.ReadMessage()
			if err != nil {
				return nil // client disconnected
			}
			if err := conn.WriteMessage(mt, msg); err != nil {
				return nil
			}
		}
	})

	app.Get("/", func(c *goxpress.Context) error {
		return c.String(http.StatusOK, "GET /events for SSE, ws://.../ws for WebSocket echo")
	})

	log.Println("listening on :3000")
	if err := app.Listen(":3000"); err != nil {
		log.Fatal(err)
	}
}
