package web

import (
	"embed"
	"fmt"
	"html/template"
	"io"

	"uzeltok/internal/buildinfo"
)

//go:embed template/*.gohtml
var templateFS embed.FS

// Provider is an HTML template provider.
type Provider struct {
	templates map[string]*template.Template
}

// NewProvider creates a new template provider and preloads all gohtml templates.
func NewProvider() (*Provider, error) {
	p := &Provider{
		templates: make(map[string]*template.Template),
	}

	// Parse layout first
	layout, err := template.New("layout.gohtml").Funcs(template.FuncMap{
		"buildInfo": buildinfo.Get,
	}).ParseFS(templateFS, "template/layout.gohtml")
	if err != nil {
		return nil, fmt.Errorf("failed to parse layout: %w", err)
	}

	// For each page, clone the layout and parse the specific page
	pages := []string{"index.gohtml", "share.gohtml", "drop.gohtml", "404.gohtml", "401.gohtml", "admin.gohtml", "admin_link.gohtml"}
	for _, page := range pages {
		clone, err := layout.Clone()
		if err != nil {
			return nil, fmt.Errorf("failed to clone layout for %s: %w", page, err)
		}
		_, err = clone.ParseFS(templateFS, "template/"+page)
		if err != nil {
			return nil, fmt.Errorf("failed to parse template %s: %w", page, err)
		}
		p.templates[page] = clone
	}

	return p, nil
}

// Render executes the named template with the given data and writes it to w.
func (p *Provider) Render(w io.Writer, name string, data interface{}) error {
	t, ok := p.templates[name]
	if !ok {
		return fmt.Errorf("template not found: %s", name)
	}
	return t.ExecuteTemplate(w, name, data)
}
