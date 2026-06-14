// Command rest-api demonstrates a small CRUD API built with goXpress: route
// groups, built-in middleware, request binding and centralized error handling.
package main

import (
	"log"
	"net/http"
	"sync"

	"github.com/chmenegatti/goxpress"
	"github.com/chmenegatti/goxpress/middleware"
)

// book is the resource managed by the API.
type book struct {
	ID     int    `json:"id"`
	Title  string `json:"title"`
	Author string `json:"author"`
}

// store is a tiny in-memory, concurrency-safe book repository.
type store struct {
	mu     sync.Mutex
	books  map[int]book
	nextID int
}

func newStore() *store {
	return &store{books: make(map[int]book), nextID: 1}
}

func (s *store) list() []book {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]book, 0, len(s.books))
	for _, b := range s.books {
		out = append(out, b)
	}
	return out
}

func (s *store) create(b book) book {
	s.mu.Lock()
	defer s.mu.Unlock()
	b.ID = s.nextID
	s.nextID++
	s.books[b.ID] = b
	return b
}

func (s *store) get(id int) (book, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.books[id]
	return b, ok
}

func main() {
	app := goxpress.New()
	app.Use(
		middleware.RequestID(),
		middleware.Logger(),
		middleware.Recoverer(),
		middleware.CORS(),
	)

	db := newStore()

	api := app.Group("/api/v1")

	api.Get("/books", func(c *goxpress.Context) error {
		return c.JSON(http.StatusOK, db.list())
	})

	api.Post("/books", func(c *goxpress.Context) error {
		var b book
		if err := c.Bind(&b); err != nil {
			return err // 400 with the binding message
		}
		if b.Title == "" {
			return goxpress.NewHTTPError(http.StatusUnprocessableEntity, "title is required")
		}
		return c.JSON(http.StatusCreated, db.create(b))
	})

	api.Get("/books/:id", func(c *goxpress.Context) error {
		id, err := parseID(c.Param("id"))
		if err != nil {
			return goxpress.NewHTTPError(http.StatusBadRequest, "invalid id")
		}
		b, ok := db.get(id)
		if !ok {
			return goxpress.NewHTTPError(http.StatusNotFound, "book not found")
		}
		return c.JSON(http.StatusOK, b)
	})

	log.Println("listening on :3000")
	if err := app.Listen(":3000"); err != nil {
		log.Fatal(err)
	}
}

// parseID parses a positive integer path parameter.
func parseID(s string) (int, error) {
	var id int
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, errInvalidID
		}
		id = id*10 + int(r-'0')
	}
	if s == "" {
		return 0, errInvalidID
	}
	return id, nil
}

var errInvalidID = &parseError{}

type parseError struct{}

func (*parseError) Error() string { return "invalid id" }
