package goxpress_test

import (
	"fmt"
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/chmenegatti/goxpress"
)

// Example shows the smallest useful goXpress application.
func Example() {
	app := goxpress.New()
	app.Get("/hello/:name", func(c *goxpress.Context) error {
		return c.String(http.StatusOK, "Hello, %s!", c.Param("name"))
	})

	// Drive one request through the router (instead of app.Listen).
	req := httptest.NewRequest(http.MethodGet, "/hello/world", nil)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	fmt.Println(w.Code)
	fmt.Println(w.Body.String())
	// Output:
	// 200
	// Hello, world!
}

// ExampleRouter_Group demonstrates a route group with shared middleware.
func ExampleRouter_Group() {
	app := goxpress.New()

	api := app.Group("/api", func(c *goxpress.Context) error {
		c.SetHeader("X-API", "v1")
		return c.Next()
	})
	api.Get("/ping", func(c *goxpress.Context) error {
		return c.String(http.StatusOK, "pong")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/ping", nil)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	fmt.Println(w.Body.String())
	fmt.Println(w.Header().Get("X-API"))
	// Output:
	// pong
	// v1
}

// ExampleNewHTTPError shows returning a typed error that the centralized error
// handler renders as JSON.
func ExampleNewHTTPError() {
	app := goxpress.New()
	app.Get("/secret", func(c *goxpress.Context) error {
		return goxpress.NewHTTPError(http.StatusForbidden, "access denied")
	})

	req := httptest.NewRequest(http.MethodGet, "/secret", nil)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	fmt.Println(w.Code)
	fmt.Println(w.Body.String())
	// Output:
	// 403
	// {"error":"access denied"}
}

// ExampleBindJSON shows the generics-based binding API decoding a JSON body
// into a typed value without a pre-declared variable.
func ExampleBindJSON() {
	type CreateUser struct {
		Name string `json:"name"`
	}

	app := goxpress.New()
	app.Post("/users", func(c *goxpress.Context) error {
		req, err := goxpress.BindJSON[CreateUser](c)
		if err != nil {
			return err
		}
		return c.JSON(http.StatusCreated, req)
	})

	body := strings.NewReader(`{"name":"ada"}`)
	r := httptest.NewRequest(http.MethodPost, "/users", body)
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	app.ServeHTTP(w, r)

	fmt.Println(w.Code)
	fmt.Println(w.Body.String())
	// Output:
	// 201
	// {"name":"ada"}
}

// ExampleContext_HTML renders a named html/template through a Renderer set on
// the Router.
func ExampleContext_HTML() {
	tmpl := template.Must(template.New("hello").Parse(`<h1>Hello, {{.Name}}!</h1>`))

	app := goxpress.New()
	app.Renderer = goxpress.NewTemplateRenderer(tmpl)
	app.Get("/", func(c *goxpress.Context) error {
		return c.HTML(http.StatusOK, "hello", map[string]string{"Name": "world"})
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	fmt.Println(w.Code)
	fmt.Println(w.Header().Get("Content-Type"))
	fmt.Println(w.Body.String())
	// Output:
	// 200
	// text/html; charset=utf-8
	// <h1>Hello, world!</h1>
}
