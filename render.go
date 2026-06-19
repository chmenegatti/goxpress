package goxpress

import (
	"html/template"
	"io"
)

// Renderer renders the named template with data, writing the result to w. The
// active Context is supplied so implementations may read request-scoped state.
//
// Set a Renderer on Router.Renderer to enable Context.HTML.
type Renderer interface {
	Render(w io.Writer, name string, data any, c *Context) error
}

// TemplateRenderer adapts an html/template.Template to the Renderer interface,
// rendering a template by name via ExecuteTemplate. It suits templates parsed
// with ParseFiles or ParseGlob, and is safe for concurrent use once parsing is
// complete.
type TemplateRenderer struct {
	// Template is the parsed template set whose named members are rendered.
	Template *template.Template
}

// NewTemplateRenderer returns a TemplateRenderer backed by t.
func NewTemplateRenderer(t *template.Template) *TemplateRenderer {
	return &TemplateRenderer{Template: t}
}

// Render renders the named template from the underlying set.
func (tr *TemplateRenderer) Render(w io.Writer, name string, data any, _ *Context) error {
	return tr.Template.ExecuteTemplate(w, name, data)
}
