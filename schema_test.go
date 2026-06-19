package goxpress

import (
	"reflect"
	"testing"
	"time"
)

func newGen() *schemaGen { return &schemaGen{components: map[string]*schema{}} }

func TestSchemaForScalars(t *testing.T) {
	g := newGen()
	cases := []struct {
		val      any
		wantType string
		wantFmt  string
	}{
		{"", "string", ""},
		{true, "boolean", ""},
		{int32(0), "integer", ""},
		{uint(0), "integer", ""},
		{3.14, "number", ""},
		{time.Time{}, "string", "date-time"},
	}
	for _, tc := range cases {
		s := g.schemaFor(reflect.TypeOf(tc.val))
		if s.Type != tc.wantType || s.Format != tc.wantFmt {
			t.Errorf("%T -> {type:%q fmt:%q}, want {%q %q}", tc.val, s.Type, s.Format, tc.wantType, tc.wantFmt)
		}
	}
}

func TestSchemaForSliceAndMap(t *testing.T) {
	g := newGen()

	sl := g.schemaFor(reflect.TypeOf([]string{}))
	if sl.Type != "array" || sl.Items == nil || sl.Items.Type != "string" {
		t.Errorf("slice schema = %+v", sl)
	}

	m := g.schemaFor(reflect.TypeOf(map[string]int{}))
	if m.Type != "object" || m.AdditionalProperties == nil || m.AdditionalProperties.Type != "integer" {
		t.Errorf("map schema = %+v", m)
	}
}

func TestSchemaForStructRegistersComponent(t *testing.T) {
	type Inner struct {
		V string `json:"v"`
	}
	type Outer struct {
		Name   string `json:"name"`
		Maybe  string `json:"maybe,omitempty"`
		Ignore string `json:"-"`
		hidden string //nolint:unused // unexported, must be skipped
		Inner  Inner  `json:"inner"`
	}

	g := newGen()
	s := g.schemaFor(reflect.TypeOf(Outer{}))

	if s.Ref != "#/components/schemas/Outer" {
		t.Fatalf("ref = %q", s.Ref)
	}

	outer := g.components["Outer"]
	if outer == nil {
		t.Fatal("Outer not registered")
	}
	if _, ok := outer.Properties["name"]; !ok {
		t.Error("missing name property")
	}
	if _, ok := outer.Properties["Ignore"]; ok {
		t.Error("json:\"-\" field should be skipped")
	}
	if _, ok := outer.Properties["hidden"]; ok {
		t.Error("unexported field should be skipped")
	}
	// Nested struct registered separately and referenced.
	if outer.Properties["inner"].Ref != "#/components/schemas/Inner" {
		t.Errorf("inner ref = %q", outer.Properties["inner"].Ref)
	}
	if g.components["Inner"] == nil {
		t.Error("Inner component not registered")
	}

	// required excludes the omitempty field.
	req := map[string]bool{}
	for _, r := range outer.Required {
		req[r] = true
	}
	if !req["name"] || req["maybe"] {
		t.Errorf("required = %v", outer.Required)
	}
}

func TestSchemaForPointerDeref(t *testing.T) {
	type T struct {
		N int `json:"n"`
	}
	g := newGen()
	s := g.schemaFor(reflect.TypeOf(&T{}))
	if s.Ref != "#/components/schemas/T" {
		t.Errorf("pointer struct ref = %q", s.Ref)
	}
}

func TestSchemaForRecursive(t *testing.T) {
	type Node struct {
		Next *Node `json:"next,omitempty"`
	}
	g := newGen()
	// Must terminate (placeholder reservation breaks the cycle).
	s := g.schemaFor(reflect.TypeOf(Node{}))
	if s.Ref != "#/components/schemas/Node" {
		t.Errorf("ref = %q", s.Ref)
	}
	if g.components["Node"] == nil {
		t.Error("Node not registered")
	}
}

func TestQueryParams(t *testing.T) {
	type Filter struct {
		Page int    `query:"page"`
		Name string `query:"name"`
		Skip string `query:"-"`
		Bare string // no tag -> field name
	}
	g := newGen()
	params := g.queryParams(reflect.TypeOf(Filter{}))

	got := map[string]bool{}
	for _, p := range params {
		got[p.Name] = true
		if p.In != "query" {
			t.Errorf("param %q in = %q", p.Name, p.In)
		}
	}
	if !got["page"] || !got["name"] || !got["Bare"] {
		t.Errorf("params = %v", got)
	}
	if got["-"] {
		t.Error("query:\"-\" field should be skipped")
	}
}

func TestJSONField(t *testing.T) {
	type S struct {
		A string `json:"a,omitempty"`
		B string `json:"b"`
		C string
		D string `json:"-"`
	}
	rt := reflect.TypeOf(S{})

	name, omit := jsonField(rt.Field(0))
	if name != "a" || !omit {
		t.Errorf("A -> %q %v", name, omit)
	}
	name, omit = jsonField(rt.Field(1))
	if name != "b" || omit {
		t.Errorf("B -> %q %v", name, omit)
	}
	name, _ = jsonField(rt.Field(2))
	if name != "C" {
		t.Errorf("C (no tag) -> %q, want C", name)
	}
	name, _ = jsonField(rt.Field(3))
	if name != "-" {
		t.Errorf("D -> %q, want -", name)
	}
}
