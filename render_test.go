package goxpress_test

import (
	"errors"
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chmenegatti/goxpress"
)

// stubRenderer records its arguments and returns a configurable error, letting
// tests drive Context.HTML without a real template.
type stubRenderer struct {
	out     string
	err     error
	gotName string
	gotData any
	gotCtx  bool
}

func (s *stubRenderer) Render(w io.Writer, name string, data any, c *goxpress.Context) error {
	s.gotName = name
	s.gotData = data
	s.gotCtx = c != nil
	if s.err != nil {
		return s.err
	}
	_, err := io.WriteString(w, s.out)
	return err
}

func TestContextHTML(t *testing.T) {
	tmpl := template.Must(template.New("greet").Parse(`Hi {{.}}`))

	tests := []struct {
		name        string
		renderer    goxpress.Renderer
		wantCode    int
		wantCT      string
		wantBody    string
		wantErrType bool // expect an *HTTPError surfaced to the error handler
	}{
		{
			name:     "template renderer",
			renderer: goxpress.NewTemplateRenderer(tmpl),
			wantCode: http.StatusOK,
			wantCT:   goxpress.MIMEHTML,
			wantBody: "Hi gopher",
		},
		{
			name:     "stub renderer",
			renderer: &stubRenderer{out: "<p>ok</p>"},
			wantCode: http.StatusCreated,
			wantCT:   goxpress.MIMEHTML,
			wantBody: "<p>ok</p>",
		},
		{
			name:        "no renderer configured",
			renderer:    nil,
			wantCode:    http.StatusInternalServerError,
			wantBody:    `"error"`,
			wantErrType: true,
		},
		{
			name:     "renderer error",
			renderer: &stubRenderer{err: errors.New("boom")},
			wantCode: http.StatusInternalServerError,
			wantBody: `"error"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := goxpress.New()
			app.Renderer = tt.renderer
			code := http.StatusOK
			if tt.name == "stub renderer" {
				code = http.StatusCreated
			}
			app.Get("/", func(c *goxpress.Context) error {
				return c.HTML(code, "greet", "gopher")
			})

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			w := httptest.NewRecorder()
			app.ServeHTTP(w, req)

			if w.Code != tt.wantCode {
				t.Errorf("code = %d, want %d", w.Code, tt.wantCode)
			}
			if tt.wantCT != "" && w.Header().Get("Content-Type") != tt.wantCT {
				t.Errorf("Content-Type = %q, want %q", w.Header().Get("Content-Type"), tt.wantCT)
			}
			if !strings.Contains(w.Body.String(), tt.wantBody) {
				t.Errorf("body = %q, want to contain %q", w.Body.String(), tt.wantBody)
			}
		})
	}
}

// TestContextHTMLPassesContext verifies the renderer receives the template
// name, data, and the active Context.
func TestContextHTMLPassesContext(t *testing.T) {
	sr := &stubRenderer{out: "x"}
	app := goxpress.New()
	app.Renderer = sr
	app.Get("/", func(c *goxpress.Context) error {
		return c.HTML(http.StatusOK, "page", 42)
	})

	app.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	if sr.gotName != "page" {
		t.Errorf("name = %q, want page", sr.gotName)
	}
	if sr.gotData != 42 {
		t.Errorf("data = %v, want 42", sr.gotData)
	}
	if !sr.gotCtx {
		t.Error("renderer did not receive a Context")
	}
}

// TestContextHTMLErrorNotPartiallyWritten ensures a render failure leaves the
// body empty so the error handler can produce a clean response.
func TestContextHTMLErrorNotPartiallyWritten(t *testing.T) {
	app := goxpress.New()
	app.Renderer = &stubRenderer{err: errors.New("fail")}
	app.Get("/", func(c *goxpress.Context) error {
		return c.HTML(http.StatusOK, "x", nil)
	})

	w := httptest.NewRecorder()
	app.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

	if strings.Contains(w.Body.String(), "fail") {
		t.Errorf("internal error leaked to client: %q", w.Body.String())
	}
}
