package goxpress

import (
	"reflect"
	"strings"
	"time"
)

// schema is a minimal OpenAPI 3.1 / JSON Schema object. Struct types are
// emitted to components.schemas and referenced via $ref; everything else is
// inlined.
type schema struct {
	Ref                  string             `json:"$ref,omitempty"`
	Type                 string             `json:"type,omitempty"`
	Format               string             `json:"format,omitempty"`
	Properties           map[string]*schema `json:"properties,omitempty"`
	Required             []string           `json:"required,omitempty"`
	Items                *schema            `json:"items,omitempty"`
	AdditionalProperties *schema            `json:"additionalProperties,omitempty"`
}

// timeType matches time.Time, which is rendered as an RFC 3339 date-time string
// rather than an object.
var timeType = reflect.TypeFor[time.Time]()

// schemaGen builds schemas while collecting reusable named struct schemas in
// components, so each struct type is described once and referenced thereafter.
type schemaGen struct {
	components map[string]*schema
}

// schemaFor returns the schema for t, registering named struct types in
// components and returning a $ref to them.
func (g *schemaGen) schemaFor(t reflect.Type) *schema {
	t = deref(t)

	switch t.Kind() {
	case reflect.String:
		return &schema{Type: "string"}
	case reflect.Bool:
		return &schema{Type: "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return &schema{Type: "integer"}
	case reflect.Float32, reflect.Float64:
		return &schema{Type: "number"}
	case reflect.Slice, reflect.Array:
		return &schema{Type: "array", Items: g.schemaFor(t.Elem())}
	case reflect.Map:
		return &schema{Type: "object", AdditionalProperties: g.schemaFor(t.Elem())}
	case reflect.Struct:
		if t == timeType {
			return &schema{Type: "string", Format: "date-time"}
		}
		name := t.Name()
		if name == "" { // anonymous struct: inline it
			return g.structSchema(t)
		}
		if _, ok := g.components[name]; !ok {
			// Reserve the name first so recursive types terminate.
			g.components[name] = &schema{}
			g.components[name] = g.structSchema(t)
		}
		return &schema{Ref: "#/components/schemas/" + name}
	default:
		return &schema{}
	}
}

// structSchema builds an object schema from a struct type's exported fields,
// honoring json tags (name, "-", omitempty).
func (g *schemaGen) structSchema(t reflect.Type) *schema {
	s := &schema{Type: "object", Properties: map[string]*schema{}}

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		name, omitempty := jsonField(f)
		if name == "-" {
			continue
		}
		s.Properties[name] = g.schemaFor(f.Type)
		if !omitempty && f.Type.Kind() != reflect.Pointer {
			s.Required = append(s.Required, name)
		}
	}

	if len(s.Properties) == 0 {
		s.Properties = nil
	}
	return s
}

// queryParams builds query parameters from the "query"-tagged exported fields
// of a struct type.
func (g *schemaGen) queryParams(t reflect.Type) []parameter {
	t = deref(t)
	if t.Kind() != reflect.Struct {
		return nil
	}

	var params []parameter
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		name := f.Tag.Get("query")
		if name == "-" {
			continue
		}
		if name == "" {
			name = f.Name
		}
		params = append(params, parameter{
			Name:   name,
			In:     "query",
			Schema: g.schemaFor(f.Type),
		})
	}
	return params
}

// jsonField returns the JSON property name and omitempty flag for a struct
// field, falling back to the field name when no json tag is present.
func jsonField(f reflect.StructField) (name string, omitempty bool) {
	tag := f.Tag.Get("json")
	if tag == "" {
		return f.Name, false
	}
	parts := strings.Split(tag, ",")
	name = parts[0]
	if name == "" {
		name = f.Name
	}
	for _, opt := range parts[1:] {
		if opt == "omitempty" {
			omitempty = true
		}
	}
	return name, omitempty
}

// deref unwraps pointer types to their element type.
func deref(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t
}
