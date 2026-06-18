# goXpress

[![CI](https://github.com/chmenegatti/goxpress/actions/workflows/ci.yml/badge.svg)](https://github.com/chmenegatti/goxpress/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/chmenegatti/goxpress.svg)](https://pkg.go.dev/github.com/chmenegatti/goxpress)
[![Go Report Card](https://goreportcard.com/badge/github.com/chmenegatti/goxpress)](https://goreportcard.com/report/github.com/chmenegatti/goxpress)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

> A fast, ergonomic HTTP router for Go — inspired by Express.js, powered by the
> best ideas from chi, gin, fiber and echo.

goXpress combines an Express-like developer experience with a high-performance
radix-tree engine, while staying **100% compatible with the standard
`net/http`** and keeping **zero dependencies in the core**.

## Why goXpress

| From | What we borrowed |
| ---- | ---------------- |
| **chi** | `net/http` compatibility, sub-router mounting, idiomatic middleware |
| **gin** | Radix-tree performance, a rich request `Context`, route groups |
| **fiber / Express** | Ergonomic API (`app.Get`, `:param`, `*wildcard`) |
| **echo** | `func(c *Context) error` handlers, centralized error handling |
| **httprouter** | Priority-ordered radix tree |

## Features

- ⚡ **Fast** — priority radix-tree routing, **0 allocations** on the hot path
- 🧩 **`net/http` native** — the router is an `http.Handler`; wrap and reuse the
  whole standard ecosystem
- 🎯 **Ergonomic handlers** — `func(c *Context) error` with a clean,
  centralized error pipeline
- 🧱 **Middleware** — onion-model chain with `Next`/`Abort`, plus adapters for
  standard `func(http.Handler) http.Handler` middleware
- 🌳 **Groups & mounting** — shared prefixes/middleware and chi-style `Mount`
- 📦 **Binding & rendering** — JSON/query/form binding and JSON/XML/text/blob
  responses
- 🛠 **Batteries included** — `middleware/` package: Logger, Recoverer,
  RequestID, RealIP, CORS, Timeout, Compress — all standard-library only
- 🪶 **Zero core dependencies**

## Install

```sh
go get github.com/chmenegatti/goxpress
```

Requires Go 1.26+.

## Quick start

```go
package main

import (
	"net/http"

	"github.com/chmenegatti/goxpress"
)

func main() {
	app := goxpress.New()

	app.Get("/hello/:name", func(c *goxpress.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"hello": c.Param("name")})
	})

	app.Listen(":3000")
}
```

## Routing

```go
app.Get("/users", listUsers)
app.Post("/users", createUser)
app.Put("/users/:id", replaceUser)
app.Patch("/users/:id", updateUser)
app.Delete("/users/:id", deleteUser)
```

Path patterns support three segment kinds:

| Pattern | Matches | Access |
| ------- | ------- | ------ |
| `/users` | exactly `/users` | — |
| `/users/:id` | one segment | `c.Param("id")` |
| `/files/*path` | the rest of the path | `c.Param("path")` |

Unmatched paths yield `404`; a path that exists for other methods yields `405`
with an `Allow` header. Trailing-slash mismatches redirect automatically (set
`app.RedirectTrailingSlash = false` to disable).

`HEAD` requests without an explicit `HEAD` route are answered by the matching
`GET` handler with the body discarded, and `OPTIONS` requests are answered with
a `204` and an `Allow` header listing the registered methods. Disable either
with `app.HandleHEAD = false` / `app.HandleOPTIONS = false`.

## Context

```go
func handler(c *goxpress.Context) error {
	id := c.Param("id")                 // URL parameter
	q := c.Query("q")                   // ?q=...
	page := c.QueryDefault("page", "1") // with default
	auth := c.GetHeader("Authorization")

	c.Set("key", value)                 // request-scoped store
	v, ok := c.Get("key")

	return c.JSON(http.StatusOK, payload) // JSON / XML / String / Blob / NoContent / Redirect
}
```

## Static files

```go
app.Static("/assets", "./public")          // serve a directory tree
app.File("/favicon.ico", "./public/favicon.ico") // serve a single file

app.Get("/logo", func(c *goxpress.Context) error {
	return c.File("./public/logo.png")       // or c.Attachment(path, "logo.png")
})
```

`Static` maps a directory under a URL prefix via a catch-all route; path
traversal is blocked and content types are detected automatically. `StaticFS`
takes any `fs.FS` (e.g. an `embed.FS`), and `c.FileFromFS` serves a single file
from one.

## Groups & mounting

```go
api := app.Group("/api/v1", authMiddleware)
api.Get("/books", listBooks)

admin := api.Group("/admin", requireAdmin) // nested: /api/v1/admin/...
admin.Delete("/books/:id", deleteBook)

app.Mount("/legacy", legacyRouter) // delegate a sub-tree to another handler
```

## Middleware

```go
app.Use(func(c *goxpress.Context) error {
	start := time.Now()
	err := c.Next()              // run downstream handlers
	log.Println(time.Since(start))
	return err
})
```

Reuse standard middleware with `FromStd`, and standard handlers with
`WrapH`/`WrapF`:

```go
app.Use(goxpress.FromStd(stdMiddleware))
app.Get("/legacy", goxpress.WrapH(legacyHandler))
```

### Built-in middleware

```go
import "github.com/chmenegatti/goxpress/middleware"

app.Use(
	middleware.RequestID(),
	middleware.RealIP(),
	middleware.Logger(),
	middleware.Recoverer(),
	middleware.CORS(),
	middleware.Compress(),
	middleware.Timeout(5*time.Second),
)
```

## Binding

```go
type CreateUser struct {
	Name  string `json:"name" form:"name"`
	Email string `json:"email" form:"email"`
}

func createUser(c *goxpress.Context) error {
	var in CreateUser
	if err := c.Bind(&in); err != nil { // 400 on failure
		return err
	}
	// ...
	return c.JSON(http.StatusCreated, in)
}
```

`Bind` negotiates by `Content-Type` (JSON or form); `BindJSON`, `BindForm` and
`BindQuery` (struct tags `query`/`form`) are available explicitly.

## Error handling

Return an `error` from any handler — the centralized handler renders it:

```go
func getBook(c *goxpress.Context) error {
	b, ok := db.Get(c.Param("id"))
	if !ok {
		return goxpress.NewHTTPError(http.StatusNotFound, "book not found")
	}
	return c.JSON(http.StatusOK, b)
}
```

`HTTPError` is rendered with its status and message; any other error becomes a
`500`. Override with `app.ErrorHandler`. Panics are recovered by default
(`app.Recovery`).

## Graceful shutdown

```go
app := goxpress.New()
app.ShutdownTimeout = 15 * time.Second // drain window (default 10s)

// Traps SIGINT/SIGTERM, then drains in-flight requests before returning.
if err := app.ListenAndServe(":3000"); err != nil {
	log.Fatal(err)
}
```

Need full control over the server? `app.Server(addr)` returns a configured
`*http.Server` you can tune (timeouts, TLS) and run yourself.

## Performance

```
BenchmarkRouterStatic-12    48203124    25.40 ns/op    0 B/op    0 allocs/op
BenchmarkRouterParam-12     35268422    37.35 ns/op    0 B/op    0 allocs/op
```

## Examples

Runnable programs live under [`examples/`](examples):

- [`examples/basic`](examples/basic) — routing, params and JSON
- [`examples/rest-api`](examples/rest-api) — CRUD with groups, middleware,
  binding and error handling
- [`examples/graceful`](examples/graceful) — graceful shutdown draining
  in-flight requests on SIGINT/SIGTERM

## Status

🚧 **Under active development.** APIs may change until `v1.0.0`.

- [x] Project scaffold & CI
- [x] Radix-tree routing engine
- [x] Request `Context` & response helpers
- [x] Middleware chain (+ `net/http` adapter)
- [x] Route groups & sub-router mounting
- [x] Centralized error handling & recovery
- [x] Request binding & rendering
- [x] Built-in middleware suite
- [x] Docs & examples

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Run `make check` before opening a PR.

## License

[MIT](LICENSE) © Cesar Menegatti
