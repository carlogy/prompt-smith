// Package server implements promptsmith's local web UI: a loopback-only
// HTTP server exposing the same registry and prompt-building logic the
// CLI and TUI already use, over a small JSON API (see api.go) plus a
// server-rendered page (see assets/index.html, wired in a later phase).
package server

import (
	"log/slog"
	"net/http"

	"github.com/carlogy/prompt-smith/internal/registry"
)

// application holds this server's dependencies, threaded into every
// handler as a method receiver rather than closed-over package
// globals - keeps handlers testable against a fresh application value
// per test, with no shared state between tests.
type application struct {
	reg    *registry.Registry
	logger *slog.Logger
}

// newApplication builds an application. A nil logger defaults to
// slog.Default(), so callers that don't care about logging (most
// tests) don't need to construct one.
func newApplication(reg *registry.Registry, logger *slog.Logger) *application {
	if logger == nil {
		logger = slog.Default()
	}
	return &application{reg: reg, logger: logger}
}

// routes builds this server's handler: the JSON API today, and (in a
// later phase) the served page. Separated from Serve (server.go) so
// tests - including a future browser-driven end-to-end suite - can
// exercise it via httptest without binding a real port.
func (app *application) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/registry", app.handleRegistry)
	mux.HandleFunc("POST /api/build", app.handleBuild)
	return mux
}
