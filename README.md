# goXpress

[![CI](https://github.com/chmenegatti/goxpress/actions/workflows/ci.yml/badge.svg)](https://github.com/chmenegatti/goxpress/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/chmenegatti/goxpress.svg)](https://pkg.go.dev/github.com/chmenegatti/goxpress)
[![Go Report Card](https://goreportcard.com/badge/github.com/chmenegatti/goxpress)](https://goreportcard.com/report/github.com/chmenegatti/goxpress)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

> A fast, ergonomic HTTP router for Go — inspired by Express.js, powered by the
> best ideas from chi, gin, fiber and echo.

goXpress combines an Express-like developer experience with a high-performance
radix-tree engine, while staying **100% compatible with the standard
`net/http`** and **zero dependencies in the core**.

## Why goXpress

| From | What we borrowed |
| ---- | ---------------- |
| **chi** | `net/http` compatibility, sub-router mounting, idiomatic middleware |
| **gin** | Radix-tree performance, a rich request `Context`, route groups |
| **fiber / Express** | Ergonomic API (`app.Get`, `:param`, `*wildcard`) |
| **echo** | `func(c *Context) error` handlers, centralized error handling |
| **httprouter** | Priority-ordered radix tree |

## Status

🚧 **Under active development.** APIs may change until `v1.0.0`.

Roadmap:

- [x] Project scaffold & CI
- [x] Radix-tree routing engine
- [ ] Request `Context` & response helpers
- [ ] Middleware chain (+ `net/http` adapter)
- [ ] Route groups & sub-router mounting
- [ ] Centralized error handling & recovery
- [ ] Request binding & rendering
- [ ] Built-in middleware suite
- [ ] Docs & examples

## Install

```sh
go get github.com/chmenegatti/goxpress
```

Requires Go 1.26+.

## Quick start

```go
package main

import "github.com/chmenegatti/goxpress"

func main() {
	app := goxpress.New()

	app.Get("/hello/:name", func(c *goxpress.Context) error {
		return c.JSON(200, map[string]string{"hello": c.Param("name")})
	})

	app.Listen(":3000")
}
```

## License

[MIT](LICENSE) © Cesar Menegatti
