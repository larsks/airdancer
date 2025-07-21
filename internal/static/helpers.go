package static

import (
	"bytes"
	"html/template"
)

// TemplateData holds data for rendering HTML templates
type TemplateData struct {
	Title         string
	DefaultStatus string
	Content       template.HTML
	CSS           template.CSS
	JS            template.JS
	ExtraCSS      template.HTML
	ExtraJS       template.HTML
}

// RenderTemplate renders the base HTML template with the provided data
func RenderTemplate(data TemplateData) (string, error) {
	// Read the base template
	baseHTML, err := assets.ReadFile("base.html")
	if err != nil {
		return "", err
	}

	// Get common CSS and JS if not provided
	if data.CSS == "" {
		css, err := GetCSS()
		if err != nil {
			return "", err
		}
		data.CSS = template.CSS(css)
	}

	if data.JS == "" {
		js, err := GetJS()
		if err != nil {
			return "", err
		}
		data.JS = template.JS(js)
	}

	// Parse and execute template
	tmpl, err := template.New("base").Parse(string(baseHTML))
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// GetBaseHTML returns the base HTML template content
func GetBaseHTML() ([]byte, error) {
	return assets.ReadFile("base.html")
}
