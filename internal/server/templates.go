package server

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"mime"
	"net/http"
)

// templateFiles holds the page templates (see page.go); html/template,
// not text/template: several fields render user-supplied text (a goal
// typed at the CLI, or a user skill's WhenToUse), and html/template
// auto-escapes it - text/template wouldn't, which would be an XSS hole
// for anything reflected into the page.
//
//go:embed assets/templates
var templateFiles embed.FS

// staticFiles holds vendored third-party assets (htmx.min.js) and, from
// a later commit, the built Tailwind CSS - served as-is at /static/,
// never templated. Vendored rather than CDN-loaded per htmx's own
// installation docs' recommendation, and to keep this a single,
// self-contained, offline-capable binary with no calls out.
//
//go:embed assets/static
var staticFiles embed.FS

// parseTemplates parses the embedded page templates. Its only possible
// failure mode is a malformed template committed to the repo - never
// anything at runtime, since the source is embedded, not read from
// disk - so a parse error here is always a build-time bug that every
// test in this package would already fail on.
func parseTemplates() (*template.Template, error) {
	sub, err := fs.Sub(templateFiles, "assets/templates")
	if err != nil {
		return nil, fmt.Errorf("server: sub embedded templates: %w", err)
	}
	tmpl, err := template.ParseFS(sub, "*.html")
	if err != nil {
		return nil, fmt.Errorf("server: parse embedded templates: %w", err)
	}
	return tmpl, nil
}

// newStaticHandler serves the embedded static assets, rooted so URLs
// are clean (/static/htmx.min.js, not /static/assets/static/htmx.min.js).
func newStaticHandler() (http.Handler, error) {
	// Force .js to the RFC 9239 standard text/javascript on every OS.
	// http.FileServer derives Content-Type from mime.TypeByExtension,
	// which on Windows is overridden by the HKEY_CLASSES_ROOT registry
	// value - commonly application/javascript there - and Go only
	// hard-codes around the .js -> text/plain registry bug (issue
	// #32350), not this one. AddExtensionType runs after that OS init
	// and overrides it, so the vendored htmx is served identically
	// regardless of host OS (see TestStaticHandler_ForcesJavaScriptMIME).
	if err := mime.AddExtensionType(".js", "text/javascript"); err != nil {
		return nil, fmt.Errorf("server: register js mime type: %w", err)
	}

	sub, err := fs.Sub(staticFiles, "assets/static")
	if err != nil {
		return nil, fmt.Errorf("server: sub embedded static assets: %w", err)
	}
	return http.FileServer(http.FS(sub)), nil
}
