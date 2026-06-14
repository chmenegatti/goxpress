// Package goxpress is a fast, ergonomic HTTP router for Go.
//
// goXpress is inspired by Express.js and combines ideas from the most popular
// Go routers — chi, gin, fiber and echo — while staying fully compatible with
// the standard net/http package and keeping zero dependencies in its core.
//
// The router is built on a priority-ordered radix tree for fast matching,
// exposes an ergonomic API (app.Get, ":param", "*wildcard"), and uses
// error-returning handlers for clean, centralized error handling:
//
//	app := goxpress.New()
//	app.Get("/hello/:name", func(c *goxpress.Context) error {
//		return c.JSON(200, map[string]string{"hello": c.Param("name")})
//	})
//	app.Listen(":3000")
//
// This package is under active development; APIs may change until v1.0.0.
package goxpress
