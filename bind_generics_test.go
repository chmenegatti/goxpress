package goxpress_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chmenegatti/goxpress"
)

type createUser struct {
	Name  string `json:"name" form:"name"`
	Email string `json:"email" form:"email"`
}

type filter struct {
	Page int    `query:"page"`
	Name string `query:"name"`
}

// ctx builds a Context wired to a request/recorder by driving it through a
// router, so the generic helpers run exactly as they would in a handler.
func bindThrough(t *testing.T, req *http.Request, h goxpress.HandlerFunc) {
	t.Helper()
	app := goxpress.New()
	app.Post("/", h)
	app.Get("/", h)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)
}

func TestGenericBindJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"ada","email":"a@b.c"}`))
	req.Header.Set("Content-Type", "application/json")

	bindThrough(t, req, func(c *goxpress.Context) error {
		got, err := goxpress.BindJSON[createUser](c)
		if err != nil {
			t.Fatalf("BindJSON: %v", err)
		}
		if got.Name != "ada" || got.Email != "a@b.c" {
			t.Errorf("got %+v", got)
		}
		return nil
	})
}

func TestGenericBindJSONInvalid(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{bad`))
	req.Header.Set("Content-Type", "application/json")

	bindThrough(t, req, func(c *goxpress.Context) error {
		_, err := goxpress.BindJSON[createUser](c)
		if err == nil {
			t.Error("want error for invalid JSON")
		}
		return nil
	})
}

func TestGenericBindEmptyStruct(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")

	bindThrough(t, req, func(c *goxpress.Context) error {
		got, err := goxpress.BindJSON[createUser](c)
		if err != nil {
			t.Fatalf("BindJSON empty: %v", err)
		}
		if got != (createUser{}) {
			t.Errorf("want zero value, got %+v", got)
		}
		return nil
	})
}

func TestGenericBindQuery(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?page=3&name=go", nil)

	bindThrough(t, req, func(c *goxpress.Context) error {
		got, err := goxpress.BindQuery[filter](c)
		if err != nil {
			t.Fatalf("BindQuery: %v", err)
		}
		if got.Page != 3 || got.Name != "go" {
			t.Errorf("got %+v", got)
		}
		return nil
	})
}

func TestGenericBindForm(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("name=grace&email=g@h.i"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	bindThrough(t, req, func(c *goxpress.Context) error {
		got, err := goxpress.BindForm[createUser](c)
		if err != nil {
			t.Fatalf("BindForm: %v", err)
		}
		if got.Name != "grace" || got.Email != "g@h.i" {
			t.Errorf("got %+v", got)
		}
		return nil
	})
}

func TestGenericBindNegotiates(t *testing.T) {
	// JSON via Bind[T] (content-type negotiated).
	jreq := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"x"}`))
	jreq.Header.Set("Content-Type", "application/json")
	bindThrough(t, jreq, func(c *goxpress.Context) error {
		got, err := goxpress.Bind[createUser](c)
		if err != nil || got.Name != "x" {
			t.Errorf("Bind JSON = %+v, %v", got, err)
		}
		return nil
	})

	// Query via Bind[T] (no body content type).
	qreq := httptest.NewRequest(http.MethodGet, "/?page=7", nil)
	bindThrough(t, qreq, func(c *goxpress.Context) error {
		got, err := goxpress.Bind[filter](c)
		if err != nil || got.Page != 7 {
			t.Errorf("Bind query = %+v, %v", got, err)
		}
		return nil
	})
}
