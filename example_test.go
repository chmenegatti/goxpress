package goxpress_test

import (
	"encoding/json"
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

// ExampleRouter_Param shows a constrained route parameter: only integer ids
// match, other values fall through to 404.
func ExampleRouter_Param() {
	app := goxpress.New()
	app.Get("/users/:id|int", func(c *goxpress.Context) error {
		return c.String(http.StatusOK, "user %s", c.Param("id"))
	})

	for _, target := range []string{"/users/42", "/users/abc"} {
		w := httptest.NewRecorder()
		app.ServeHTTP(w, httptest.NewRequest(http.MethodGet, target, nil))
		fmt.Println(target, "->", w.Code)
	}
	// Output:
	// /users/42 -> 200
	// /users/abc -> 404
}

// ExampleRouter_OpenAPI documents routes with fluent metadata and serves the
// generated OpenAPI 3.1 spec at /openapi.json.
func ExampleRouter_OpenAPI() {
	type CreateUser struct {
		Name string `json:"name"`
	}
	type User struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	app := goxpress.New()
	app.Post("/users", func(c *goxpress.Context) error { return nil }).
		Summary("Create user").
		Tags("Users").
		Body(CreateUser{}).
		Produces(201, User{})
	app.OpenAPI()

	w := httptest.NewRecorder()
	app.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))

	var doc struct {
		OpenAPI string `json:"openapi"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &doc)
	fmt.Println(doc.OpenAPI)
	// Output:
	// 3.1.0
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
