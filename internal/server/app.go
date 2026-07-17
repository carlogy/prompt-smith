// Package server implements promptsmith's local web UI: a loopback-only
// HTTP server exposing the same registry and prompt-building logic the
// CLI and TUI already use, over a small JSON API (see api.go) and a
// server-rendered page (see page.go, assets/templates/index.html).
package server

import (
	"html/template"
	"log/slog"
	"net/http"
	"sort"

	"github.com/carlogy/prompt-smith/internal/prompt"
	"github.com/carlogy/prompt-smith/internal/registry"
)

// application holds this server's dependencies, threaded into every
// handler as a method receiver rather than closed-over package
// globals - keeps handlers testable against a fresh application value
// per test, with no shared state between tests.
type application struct {
	reg     *registry.Registry
	logger  *slog.Logger
	tmpl    *template.Template
	static  http.Handler
	initial prompt.Inputs // seeds the page's form - see --ui's flag seeding in cli
}

// newApplication builds an application. A nil logger defaults to
// slog.Default(), so callers that don't care about logging (most
// tests) don't need to construct one. The only failure modes are a
// malformed embedded template or a broken embedded static-asset path -
// see parseTemplates/newStaticHandler - both build-time-only bugs.
func newApplication(reg *registry.Registry, logger *slog.Logger, initial prompt.Inputs) (*application, error) {
	if logger == nil {
		logger = slog.Default()
	}
	tmpl, err := parseTemplates()
	if err != nil {
		return nil, err
	}
	static, err := newStaticHandler()
	if err != nil {
		return nil, err
	}
	return &application{reg: reg, logger: logger, tmpl: tmpl, static: static, initial: initial}, nil
}

// routes builds this server's handler: the served page, its static
// assets, and the JSON API, wrapped in enforceLocalOnly (see
// security.go) - every route this server has needs that protection, so
// it's applied once here rather than per-registration. Separated from
// Serve (server.go) so tests - including a future browser-driven
// end-to-end suite - can exercise it via httptest without binding a
// real port.
func (app *application) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", app.handleIndex) // {$}: exact "/" only, not every unmatched path as a subtree
	mux.Handle("GET /static/", http.StripPrefix("/static/", app.static))
	mux.HandleFunc("GET /api/registry", app.handleRegistry)
	mux.HandleFunc("POST /api/build", app.handleBuild)
	return enforceLocalOnly(mux)
}

// sortedTargetIDs returns the registry's target ids, alphabetically:
// Targets has no canonical order (unlike Categories, which is an
// explicit ordered slice) - it's a map, so alphabetical is the
// simplest deterministic choice. Shared by handleRegistry and
// handleIndex.
func sortedTargetIDs(reg *registry.Registry) []string {
	ids := make([]string, 0, len(reg.Targets))
	for id := range reg.Targets {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// sortedSkills returns a defensive copy of reg.Skills in the same
// canonical order (category position, then weight, then id) every
// other surface uses - list, the TUI picker. A copy because SortSkills
// sorts in place, and reg.Skills is shared across every request.
// Shared by handleRegistry and handleIndex.
func sortedSkills(reg *registry.Registry) []registry.Skill {
	skills := append([]registry.Skill(nil), reg.Skills...)
	reg.SortSkills(skills)
	return skills
}
