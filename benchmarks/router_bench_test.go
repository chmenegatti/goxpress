// Package benchmarks compares goXpress routing throughput against gin, chi and
// echo across static, parameterized and many-route scenarios.
//
// Run with:
//
//	go test -bench=. -benchmem ./...
package benchmarks

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chmenegatti/goxpress"
	"github.com/gin-gonic/gin"
	"github.com/go-chi/chi/v5"
	"github.com/labstack/echo/v4"
)

func init() {
	gin.SetMode(gin.ReleaseMode)
}

// run drives n-free, allocation-counted requests for path against h.
func run(b *testing.B, h http.Handler, target string) {
	b.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	w := httptest.NewRecorder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h.ServeHTTP(w, req)
	}
}

// --- builders -------------------------------------------------------------

func goxpressRouter(routes []string, param bool) http.Handler {
	r := goxpress.New()
	r.Banner = false
	h := func(c *goxpress.Context) error { return c.String(http.StatusOK, "ok") }
	for _, p := range routes {
		r.Get(p, h)
	}
	if param {
		r.Get("/users/:id", func(c *goxpress.Context) error {
			return c.String(http.StatusOK, "%s", c.Param("id"))
		})
	}
	return r
}

func ginRouter(routes []string, param bool) http.Handler {
	r := gin.New()
	h := func(c *gin.Context) { c.String(http.StatusOK, "ok") }
	for _, p := range routes {
		r.GET(p, h)
	}
	if param {
		r.GET("/users/:id", func(c *gin.Context) { c.String(http.StatusOK, c.Param("id")) })
	}
	return r
}

func chiRouter(routes []string, param bool) http.Handler {
	r := chi.NewRouter()
	h := func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) }
	for _, p := range routes {
		r.Get(p, h)
	}
	if param {
		r.Get("/users/{id}", func(w http.ResponseWriter, req *http.Request) {
			_, _ = w.Write([]byte(chi.URLParam(req, "id")))
		})
	}
	return r
}

func echoRouter(routes []string, param bool) http.Handler {
	e := echo.New()
	h := func(c echo.Context) error { return c.String(http.StatusOK, "ok") }
	for _, p := range routes {
		e.GET(p, h)
	}
	if param {
		e.GET("/users/:id", func(c echo.Context) error { return c.String(http.StatusOK, c.Param("id")) })
	}
	return e
}

// manyRoutes returns n distinct static routes plus the one hit during the
// benchmark as the last entry.
func manyRoutes(n int) []string {
	routes := make([]string, 0, n)
	for i := 0; i < n; i++ {
		routes = append(routes, fmt.Sprintf("/route%d/sub%d/leaf", i, i))
	}
	return routes
}

// --- benchmarks -----------------------------------------------------------

func BenchmarkStatic(b *testing.B) {
	const path = "/ping"
	b.Run("goxpress", func(b *testing.B) { run(b, goxpressRouter([]string{path}, false), path) })
	b.Run("gin", func(b *testing.B) { run(b, ginRouter([]string{path}, false), path) })
	b.Run("chi", func(b *testing.B) { run(b, chiRouter([]string{path}, false), path) })
	b.Run("echo", func(b *testing.B) { run(b, echoRouter([]string{path}, false), path) })
}

func BenchmarkParam(b *testing.B) {
	const target = "/users/42"
	b.Run("goxpress", func(b *testing.B) { run(b, goxpressRouter(nil, true), target) })
	b.Run("gin", func(b *testing.B) { run(b, ginRouter(nil, true), target) })
	b.Run("chi", func(b *testing.B) { run(b, chiRouter(nil, true), target) })
	b.Run("echo", func(b *testing.B) { run(b, echoRouter(nil, true), target) })
}

func BenchmarkManyRoutes(b *testing.B) {
	routes := manyRoutes(200)
	target := routes[len(routes)-1]
	b.Run("goxpress", func(b *testing.B) { run(b, goxpressRouter(routes, false), target) })
	b.Run("gin", func(b *testing.B) { run(b, ginRouter(routes, false), target) })
	b.Run("chi", func(b *testing.B) { run(b, chiRouter(routes, false), target) })
	b.Run("echo", func(b *testing.B) { run(b, echoRouter(routes, false), target) })
}
