package view

import (
	"html/template"
	"io/fs"
	"net/http"
	"panel/internal/i18n"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/gin-gonic/gin/render"
)

// Renderer renders html templates.
type Renderer struct {
	templates *template.Template
}

// New loads template files.
func New(patterns ...string) (*Renderer, error) {
	root := template.New("").Funcs(template.FuncMap{
		"safeHTML": func(s string) template.HTML { return template.HTML(s) },
		"safeCSS":  func(s string) template.CSS { return template.CSS(s) },
		"t":        func(lang, key string) string { return i18n.T(lang, key) },
		"initial": func(s string) string {
			trimmed := strings.TrimSpace(s)
			if trimmed == "" {
				return "?"
			}
			runes := []rune(trimmed)
			return string(unicode.ToUpper(runes[0]))
		},
	})

	var err error
	for _, pattern := range patterns {
		matches, matchErr := filepath.Glob(pattern)
		if matchErr != nil {
			return nil, matchErr
		}
		if len(matches) == 0 {
			continue
		}
		root, err = root.ParseFiles(matches...)
		if err != nil {
			return nil, err
		}
	}
	return &Renderer{templates: root}, nil
}

// NewFromFS loads template files from an fs.
func NewFromFS(files fs.FS, patterns ...string) (*Renderer, error) {
	root := template.New("").Funcs(template.FuncMap{
		"safeHTML": func(s string) template.HTML { return template.HTML(s) },
		"safeCSS":  func(s string) template.CSS { return template.CSS(s) },
		"t":        func(lang, key string) string { return i18n.T(lang, key) },
		"initial": func(s string) string {
			trimmed := strings.TrimSpace(s)
			if trimmed == "" {
				return "?"
			}
			runes := []rune(trimmed)
			return string(unicode.ToUpper(runes[0]))
		},
	})

	parsed, err := root.ParseFS(files, patterns...)
	if err != nil {
		return nil, err
	}
	return &Renderer{templates: parsed}, nil
}

// HTML renders a named template via a gin-like context.
func (r *Renderer) HTML(c interface{ HTML(int, string, any) }, status int, name string, data any) {
	c.HTML(status, name, data)
}

// Instance creates a gin render instance.
func (r *Renderer) Instance(name string, data any) render.Render {
	return &HTML{
		Template: r.templates,
		Name:     name,
		Data:     data,
	}
}

// HTML is a gin render implementation.
type HTML struct {
	Template *template.Template
	Name     string
	Data     any
}

// Render writes the html response.
func (h *HTML) Render(w http.ResponseWriter) error {
	return h.Template.ExecuteTemplate(w, h.Name, h.Data)
}

// WriteContentType writes headers.
func (h *HTML) WriteContentType(w http.ResponseWriter) {
	header := w.Header()
	if header.Get("Content-Type") == "" {
		header.Set("Content-Type", "text/html; charset=utf-8")
	}
}
