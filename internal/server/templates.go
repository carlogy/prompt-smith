package server

import (
	_ "embed"
	"fmt"
	"html/template"
)

// indexHTMLSource is the page template rendered at GET / (see
// page.go). html/template, not text/template: several fields render
// user-supplied text (a goal typed at the CLI, or a user skill's
// WhenToUse), and html/template auto-escapes it - text/template
// wouldn't, which would be an XSS hole for anything reflected into
// the page.
//
//go:embed assets/index.html
var indexHTMLSource string

// parseTemplates parses the embedded page template. Its only possible
// failure mode is a malformed template committed to the repo - never
// anything at runtime, since the source is embedded, not read from
// disk - so a parse error here is always a build-time bug that every
// test in this package would already fail on.
func parseTemplates() (*template.Template, error) {
	tmpl, err := template.New("index.html").Parse(indexHTMLSource)
	if err != nil {
		return nil, fmt.Errorf("server: parse embedded index.html: %w", err)
	}
	return tmpl, nil
}
