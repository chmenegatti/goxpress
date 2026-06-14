// Package middleware provides a suite of ready-to-use middleware for goXpress.
//
// Every middleware in this package is built exclusively on the Go standard
// library, keeping the dependency footprint at zero. Each constructor returns a
// goxpress.HandlerFunc suitable for Router.Use or per-route registration:
//
//	app := goxpress.New()
//	app.Use(
//		middleware.RequestID(),
//		middleware.RealIP(),
//		middleware.Logger(),
//		middleware.Recoverer(),
//	)
package middleware
