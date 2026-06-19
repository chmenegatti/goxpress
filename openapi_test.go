package goxpress_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chmenegatti/goxpress"
)

type apiUser struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
}

type apiCreateUser struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type apiFilter struct {
	Page int    `query:"page"`
	Name string `query:"name"`
}

// fetchSpec registers OpenAPI on app, requests /openapi.json and returns the
// decoded document.
func fetchSpec(t *testing.T, app *goxpress.Router) map[string]any {
	t.Helper()
	app.OpenAPI()

	w := httptest.NewRecorder()
	app.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("/openapi.json code = %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != goxpress.MIMEJSON {
		t.Errorf("content-type = %q, want %q", ct, goxpress.MIMEJSON)
	}

	var doc map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &doc); err != nil {
		t.Fatalf("decode spec: %v\n%s", err, w.Body.String())
	}
	return doc
}

// at walks a nested map/slice document by keys (string) and indices (int).
func at(t *testing.T, v any, path ...any) any {
	t.Helper()
	for _, p := range path {
		switch key := p.(type) {
		case string:
			m, ok := v.(map[string]any)
			if !ok {
				t.Fatalf("expected object at %v, got %T", p, v)
			}
			v, ok = m[key]
			if !ok {
				t.Fatalf("missing key %q", key)
			}
		default:
			t.Fatalf("unsupported path element %v", p)
		}
	}
	return v
}

func TestOpenAPITopLevel(t *testing.T) {
	app := goxpress.New()
	app.Get("/users", func(c *goxpress.Context) error { return nil })
	app.Post("/users", func(c *goxpress.Context) error { return nil })

	doc := fetchSpec(t, app)
	if doc["openapi"] != "3.1.0" {
		t.Errorf("openapi = %v, want 3.1.0", doc["openapi"])
	}
	// Both methods present under /users.
	users := at(t, doc, "paths", "/users").(map[string]any)
	if _, ok := users["get"]; !ok {
		t.Error("missing GET /users")
	}
	if _, ok := users["post"]; !ok {
		t.Error("missing POST /users")
	}
	// The /openapi.json route itself must be hidden.
	if _, ok := at(t, doc, "paths").(map[string]any)["/openapi.json"]; ok {
		t.Error("/openapi.json leaked into the spec")
	}
}

func TestOpenAPIMetadata(t *testing.T) {
	app := goxpress.New()
	app.Post("/users", func(c *goxpress.Context) error { return nil }).
		Summary("Create user").
		Description("Creates a new user").
		Tags("Users").
		Body(apiCreateUser{}).
		Produces(201, apiUser{}).
		Response(400, "Invalid payload")

	doc := fetchSpec(t, app)
	op := at(t, doc, "paths", "/users", "post").(map[string]any)

	if op["summary"] != "Create user" {
		t.Errorf("summary = %v", op["summary"])
	}
	if op["description"] != "Creates a new user" {
		t.Errorf("description = %v", op["description"])
	}
	tags := op["tags"].([]any)
	if len(tags) != 1 || tags[0] != "Users" {
		t.Errorf("tags = %v", tags)
	}

	// Request body -> $ref to component.
	ref := at(t, op, "requestBody", "content", "application/json", "schema", "$ref")
	if ref != "#/components/schemas/apiCreateUser" {
		t.Errorf("body ref = %v", ref)
	}

	// Responses: 201 with schema, 400 description.
	if d := at(t, op, "responses", "400", "description"); d != "Invalid payload" {
		t.Errorf("400 description = %v", d)
	}
	if d := at(t, op, "responses", "201", "description"); d != "Created" {
		t.Errorf("201 default description = %v", d)
	}
	if r := at(t, op, "responses", "201", "content", "application/json", "schema", "$ref"); r != "#/components/schemas/apiUser" {
		t.Errorf("201 schema ref = %v", r)
	}
}

func TestOpenAPIComponentSchema(t *testing.T) {
	app := goxpress.New()
	app.Post("/users", func(c *goxpress.Context) error { return nil }).Body(apiUser{})

	doc := fetchSpec(t, app)
	props := at(t, doc, "components", "schemas", "apiUser", "properties").(map[string]any)

	if at(t, props, "id", "type") != "integer" {
		t.Errorf("id type = %v", at(t, props, "id", "type"))
	}
	if at(t, props, "name", "type") != "string" {
		t.Errorf("name type = %v", at(t, props, "name", "type"))
	}

	// "email,omitempty" must not be in required; "id"/"name" must be.
	required := map[string]bool{}
	for _, r := range at(t, doc, "components", "schemas", "apiUser", "required").([]any) {
		required[r.(string)] = true
	}
	if !required["id"] || !required["name"] {
		t.Errorf("required = %v, want id+name", required)
	}
	if required["email"] {
		t.Error("omitempty field email should not be required")
	}
}

func TestOpenAPIComponentReuse(t *testing.T) {
	app := goxpress.New()
	app.Post("/users", func(c *goxpress.Context) error { return nil }).Body(apiUser{})
	app.Put("/users/:id", func(c *goxpress.Context) error { return nil }).Body(apiUser{})

	doc := fetchSpec(t, app)
	schemas := at(t, doc, "components", "schemas").(map[string]any)
	if _, ok := schemas["apiUser"]; !ok {
		t.Fatal("apiUser component missing")
	}
	// Defined once, referenced by both operations.
	if len(schemas) != 1 {
		t.Errorf("schemas = %v, want a single reused component", schemas)
	}
}

func TestOpenAPIPathParams(t *testing.T) {
	app := goxpress.New()
	app.Get("/users/:id", func(c *goxpress.Context) error { return nil })

	doc := fetchSpec(t, app)
	// Path template uses {id}.
	op := at(t, doc, "paths", "/users/{id}", "get").(map[string]any)
	params := op["parameters"].([]any)
	if len(params) != 1 {
		t.Fatalf("parameters = %v", params)
	}
	p := params[0].(map[string]any)
	if p["name"] != "id" || p["in"] != "path" || p["required"] != true {
		t.Errorf("path param = %v", p)
	}
}

func TestOpenAPIQueryParams(t *testing.T) {
	app := goxpress.New()
	app.Get("/users", func(c *goxpress.Context) error { return nil }).Query(apiFilter{})

	doc := fetchSpec(t, app)
	params := at(t, doc, "paths", "/users", "get").(map[string]any)["parameters"].([]any)

	got := map[string]string{}
	for _, raw := range params {
		p := raw.(map[string]any)
		got[p["name"].(string)] = p["in"].(string)
	}
	if got["page"] != "query" || got["name"] != "query" {
		t.Errorf("query params = %v", got)
	}
}

func TestOpenAPIGroups(t *testing.T) {
	app := goxpress.New()
	api := app.Group("/api")
	api.Get("/users", func(c *goxpress.Context) error { return nil })

	doc := fetchSpec(t, app)
	if _, ok := at(t, doc, "paths").(map[string]any)["/api/users"]; !ok {
		t.Error("group route /api/users missing from spec")
	}
}

func TestOpenAPIDefaultResponse(t *testing.T) {
	app := goxpress.New()
	app.Get("/ping", func(c *goxpress.Context) error { return nil })

	doc := fetchSpec(t, app)
	// No metadata: a default 200 keeps the operation valid.
	if d := at(t, doc, "paths", "/ping", "get", "responses", "200", "description"); d != "OK" {
		t.Errorf("default response = %v", d)
	}
}

func TestOpenAPIGenericHelpers(t *testing.T) {
	app := goxpress.New()
	goxpress.Produces[apiUser](
		goxpress.Query[apiFilter](
			goxpress.Body[apiCreateUser](app.Post("/users", func(c *goxpress.Context) error { return nil })),
		),
		201,
	)

	doc := fetchSpec(t, app)
	op := at(t, doc, "paths", "/users", "post").(map[string]any)
	if r := at(t, op, "requestBody", "content", "application/json", "schema", "$ref"); r != "#/components/schemas/apiCreateUser" {
		t.Errorf("generic Body ref = %v", r)
	}
	if r := at(t, op, "responses", "201", "content", "application/json", "schema", "$ref"); r != "#/components/schemas/apiUser" {
		t.Errorf("generic Produces ref = %v", r)
	}
	if len(op["parameters"].([]any)) != 2 {
		t.Errorf("generic Query params = %v", op["parameters"])
	}
}
