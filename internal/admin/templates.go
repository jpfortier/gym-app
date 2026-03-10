package admin

import (
	"embed"
	"html/template"
)

//go:embed templates/*.html
var templateFS embed.FS

// LoadTemplates parses all admin templates.
func LoadTemplates() (*template.Template, error) {
	return template.New("").ParseFS(templateFS, "templates/*.html")
}
