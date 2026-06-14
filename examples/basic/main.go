// Command basic is a minimal goXpress server demonstrating routing, URL
// parameters and JSON responses.
package main

import (
	"log"
	"net/http"

	"github.com/chmenegatti/goxpress"
)

func main() {
	app := goxpress.New()

	app.Get("/", func(c *goxpress.Context) error {
		return c.String(http.StatusOK, "Welcome to goXpress!")
	})

	app.Get("/hello/:name", func(c *goxpress.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"hello": c.Param("name"),
		})
	})

	log.Println("listening on :3000")
	if err := app.Listen(":3000"); err != nil {
		log.Fatal(err)
	}
}
