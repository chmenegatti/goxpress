package goxpress

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strconv"
	"strings"
)

// DefaultOpenAPIPath is where OpenAPI serves the generated document.
const DefaultOpenAPIPath = "/openapi.json"

// Route holds metadata for a single registered route. The route-registration
// methods (Get, Post, Handle, …) return a *Route so applications can attach
// OpenAPI metadata with a fluent chain:
//
//	app.Post("/users", createUser).
//		Summary("Create user").
//		Tags("Users").
//		Body(CreateUser{}).
//		Produces(201, User{}).
//		Response(400, "Invalid payload")
//
// All metadata is optional; routes work identically whether or not it is set.
// Metadata is consumed only when building the OpenAPI document, never on the
// request hot path.
type Route struct {
	Method string
	Path   string

	summary     string
	description string
	tags        []string
	pathParams  []string
	bodyType    reflect.Type
	queryType   reflect.Type
	responses   map[int]*responseMeta
	hidden      bool
}

// responseMeta describes a single documented response.
type responseMeta struct {
	description string
	typ         reflect.Type
}

// newRoute creates route metadata, extracting the path-parameter names from the
// route pattern (":name" and "*name" segments).
func newRoute(method, path string) *Route {
	rt := &Route{Method: method, Path: path}
	for seg := range strings.SplitSeq(path, "/") {
		if len(seg) > 1 && (seg[0] == ':' || seg[0] == '*') {
			rt.pathParams = append(rt.pathParams, paramName(seg[1:]))
		}
	}
	return rt
}

// Summary sets the route's short summary.
func (rt *Route) Summary(s string) *Route { rt.summary = s; return rt }

// Description sets the route's longer description.
func (rt *Route) Description(s string) *Route { rt.description = s; return rt }

// Tags adds OpenAPI tags to the route.
func (rt *Route) Tags(tags ...string) *Route { rt.tags = append(rt.tags, tags...); return rt }

// Body documents the JSON request body using the type of sample (typically a
// zero value such as CreateUser{}). The generic Body[T] helper is an
// instance-free alternative.
func (rt *Route) Body(sample any) *Route { rt.bodyType = reflect.TypeOf(sample); return rt }

// Query documents the URL query parameters from the "query"-tagged fields of
// sample's type. The generic Query[T] helper is an instance-free alternative.
func (rt *Route) Query(sample any) *Route { rt.queryType = reflect.TypeOf(sample); return rt }

// Response documents a response status code with a description.
func (rt *Route) Response(code int, description string) *Route {
	rt.resp(code).description = description
	return rt
}

// Produces documents a JSON response body of sample's type for the status code,
// defaulting the description to the standard status text. The generic
// Produces[T] helper is an instance-free alternative.
func (rt *Route) Produces(code int, sample any) *Route {
	m := rt.resp(code)
	m.typ = reflect.TypeOf(sample)
	if m.description == "" {
		m.description = http.StatusText(code)
	}
	return rt
}

// Hide excludes the route from the generated OpenAPI document.
func (rt *Route) Hide() *Route { rt.hidden = true; return rt }

// resp returns the (lazily created) response metadata for a status code.
func (rt *Route) resp(code int) *responseMeta {
	if rt.responses == nil {
		rt.responses = map[int]*responseMeta{}
	}
	m := rt.responses[code]
	if m == nil {
		m = &responseMeta{}
		rt.responses[code] = m
	}
	return m
}

// Body sets the request body type to T without needing an instance:
//
//	goxpress.Body[CreateUser](app.Post("/users", createUser))
func Body[T any](rt *Route) *Route { rt.bodyType = reflect.TypeFor[T](); return rt }

// Query sets the query-parameter type to T without needing an instance.
func Query[T any](rt *Route) *Route { rt.queryType = reflect.TypeFor[T](); return rt }

// Produces documents a JSON response of type T for the status code without
// needing an instance.
func Produces[T any](rt *Route, code int) *Route {
	m := rt.resp(code)
	m.typ = reflect.TypeFor[T]()
	if m.description == "" {
		m.description = http.StatusText(code)
	}
	return rt
}

// OpenAPI generates an OpenAPI 3.1 document for the routes registered so far and
// serves it as JSON at DefaultOpenAPIPath (GET /openapi.json). Call it after all
// routes are registered; the document is built once and cached, so reflection
// runs at setup time only, never per request.
func (r *Router) OpenAPI() *Route {
	doc := r.buildOpenAPI()
	data, _ := json.MarshalIndent(doc, "", "  ")
	return r.Get(DefaultOpenAPIPath, func(c *Context) error {
		return c.Blob(http.StatusOK, MIMEJSON, data)
	}).Hide()
}

// buildOpenAPI assembles the OpenAPI document from the recorded routes.
func (r *Router) buildOpenAPI() *openAPIDoc {
	gen := &schemaGen{components: map[string]*schema{}}
	paths := map[string]map[string]*operation{}

	for _, rt := range r.routes {
		if rt.hidden {
			continue
		}
		tmpl := rt.openAPIPath()
		item := paths[tmpl]
		if item == nil {
			item = map[string]*operation{}
			paths[tmpl] = item
		}
		item[strings.ToLower(rt.Method)] = rt.operation(gen)
	}

	doc := &openAPIDoc{
		OpenAPI: "3.1.0",
		Info:    openAPIInfo{Title: "goXpress API", Version: "0.1.0"},
		Paths:   paths,
	}
	if len(gen.components) > 0 {
		doc.Components = &components{Schemas: gen.components}
	}
	return doc
}

// openAPIPath converts the goXpress route pattern to an OpenAPI path template,
// turning ":id" and "*path" segments into "{id}" and "{path}".
func (rt *Route) openAPIPath() string {
	segs := strings.Split(rt.Path, "/")
	for i, s := range segs {
		if len(s) > 1 && (s[0] == ':' || s[0] == '*') {
			segs[i] = "{" + paramName(s[1:]) + "}"
		}
	}
	return strings.Join(segs, "/")
}

// operation builds the OpenAPI operation object for the route.
func (rt *Route) operation(gen *schemaGen) *operation {
	op := &operation{
		Summary:     rt.summary,
		Description: rt.description,
		Tags:        rt.tags,
	}

	for _, p := range rt.pathParams {
		op.Parameters = append(op.Parameters, parameter{
			Name:     p,
			In:       "path",
			Required: true,
			Schema:   &schema{Type: "string"},
		})
	}
	if rt.queryType != nil {
		op.Parameters = append(op.Parameters, gen.queryParams(rt.queryType)...)
	}

	if rt.bodyType != nil {
		op.RequestBody = &requestBody{
			Required: true,
			Content:  map[string]mediaType{"application/json": {Schema: gen.schemaFor(rt.bodyType)}},
		}
	}

	op.Responses = map[string]response{}
	for code, meta := range rt.responses {
		resp := response{Description: meta.description}
		if meta.typ != nil {
			resp.Content = map[string]mediaType{"application/json": {Schema: gen.schemaFor(meta.typ)}}
		}
		op.Responses[strconv.Itoa(code)] = resp
	}
	if len(op.Responses) == 0 {
		// OpenAPI requires at least one response per operation.
		op.Responses["200"] = response{Description: "OK"}
	}

	return op
}

// --- OpenAPI document object model ----------------------------------------

type openAPIDoc struct {
	OpenAPI    string                           `json:"openapi"`
	Info       openAPIInfo                      `json:"info"`
	Paths      map[string]map[string]*operation `json:"paths"`
	Components *components                      `json:"components,omitempty"`
}

type openAPIInfo struct {
	Title   string `json:"title"`
	Version string `json:"version"`
}

type operation struct {
	Summary     string              `json:"summary,omitempty"`
	Description string              `json:"description,omitempty"`
	Tags        []string            `json:"tags,omitempty"`
	Parameters  []parameter         `json:"parameters,omitempty"`
	RequestBody *requestBody        `json:"requestBody,omitempty"`
	Responses   map[string]response `json:"responses"`
}

type parameter struct {
	Name     string  `json:"name"`
	In       string  `json:"in"`
	Required bool    `json:"required,omitempty"`
	Schema   *schema `json:"schema,omitempty"`
}

type requestBody struct {
	Content  map[string]mediaType `json:"content"`
	Required bool                 `json:"required,omitempty"`
}

type response struct {
	Description string               `json:"description"`
	Content     map[string]mediaType `json:"content,omitempty"`
}

type mediaType struct {
	Schema *schema `json:"schema"`
}

type components struct {
	Schemas map[string]*schema `json:"schemas,omitempty"`
}
