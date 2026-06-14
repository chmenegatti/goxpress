package goxpress

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBindJSON(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	c, _ := newContext(http.MethodPost, "/", `{"name":"go","age":15}`)
	c.Request.Header.Set("Content-Type", "application/json")

	var p payload
	if err := c.Bind(&p); err != nil {
		t.Fatalf("Bind returned error: %v", err)
	}
	if p.Name != "go" || p.Age != 15 {
		t.Errorf("payload = %+v, want {go 15}", p)
	}
}

func TestBindJSONInvalid(t *testing.T) {
	c, _ := newContext(http.MethodPost, "/", `{not json`)
	c.Request.Header.Set("Content-Type", "application/json")

	var p struct{ Name string }
	err := c.Bind(&p)
	he, ok := err.(*HTTPError)
	if !ok || he.Code != http.StatusBadRequest {
		t.Fatalf("err = %v, want 400 HTTPError", err)
	}
}

func TestBindQuery(t *testing.T) {
	type filter struct {
		Q    string   `query:"q"`
		Page int      `query:"page"`
		Tags []string `query:"tag"`
		Live bool     `query:"live"`
	}
	c, _ := newContext(http.MethodGet, "/search?q=go&page=3&tag=a&tag=b&live=true", "")

	var f filter
	if err := c.BindQuery(&f); err != nil {
		t.Fatalf("BindQuery returned error: %v", err)
	}
	if f.Q != "go" || f.Page != 3 || f.Live != true {
		t.Errorf("filter = %+v", f)
	}
	if len(f.Tags) != 2 || f.Tags[0] != "a" || f.Tags[1] != "b" {
		t.Errorf("tags = %v, want [a b]", f.Tags)
	}
}

func TestBindForm(t *testing.T) {
	type login struct {
		User string `form:"user"`
		Pin  uint   `form:"pin"`
	}
	c, _ := newContext(http.MethodPost, "/login", "user=alice&pin=4321")
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var l login
	if err := c.Bind(&l); err != nil {
		t.Fatalf("Bind returned error: %v", err)
	}
	if l.User != "alice" || l.Pin != 4321 {
		t.Errorf("login = %+v, want {alice 4321}", l)
	}
}

func TestBindQueryInvalidInt(t *testing.T) {
	c, _ := newContext(http.MethodGet, "/?page=notanumber", "")
	var f struct {
		Page int `query:"page"`
	}
	err := c.BindQuery(&f)
	if he, ok := err.(*HTTPError); !ok || he.Code != http.StatusBadRequest {
		t.Fatalf("err = %v, want 400 HTTPError", err)
	}
}

func TestBindNonPointer(t *testing.T) {
	c, _ := newContext(http.MethodGet, "/?x=1", "")
	var s struct{ X int }
	if err := c.BindQuery(s); err == nil {
		t.Error("expected error binding into non-pointer")
	}
}

func TestBindRoundTripViaRouter(t *testing.T) {
	r := New()
	r.Post("/echo", func(c *Context) error {
		var in struct {
			Msg string `json:"msg"`
		}
		if err := c.Bind(&in); err != nil {
			return err
		}
		return c.JSON(http.StatusOK, in)
	})

	req := httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader(`{"msg":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"msg":"hi"`) {
		t.Errorf("code=%d body=%q", w.Code, w.Body.String())
	}
}

func TestContextXML(t *testing.T) {
	type note struct {
		XMLName struct{} `xml:"note"`
		Body    string   `xml:"body"`
	}
	c, w := newContext(http.MethodGet, "/", "")
	if err := c.XML(http.StatusOK, note{Body: "hello"}); err != nil {
		t.Fatalf("XML returned error: %v", err)
	}
	if w.Header().Get("Content-Type") != MIMEXML {
		t.Errorf("Content-Type = %q, want %q", w.Header().Get("Content-Type"), MIMEXML)
	}
	if !strings.Contains(w.Body.String(), "<body>hello</body>") {
		t.Errorf("body = %q", w.Body.String())
	}
}
