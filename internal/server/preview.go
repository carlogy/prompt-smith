package server

import (
	"net/http"
	"strings"

	"github.com/carlogy/prompt-smith/internal/prompt"
	"github.com/carlogy/prompt-smith/internal/prompthl"
)

// previewData is what the preview partial (assets/templates/preview.html)
// renders from.
type previewData struct {
	Lines []previewLine
	Error string
}

// previewLine is one line of a built prompt plus how the preview
// should style it. IsOpen/IsClose are value-receiver methods so the
// template can call them directly ({{if $l.IsOpen}}).
type previewLine struct {
	Text string
	Kind prompthl.Kind
}

func (l previewLine) IsOpen() bool  { return l.Kind == prompthl.OpenTag }
func (l previewLine) IsClose() bool { return l.Kind == prompthl.CloseTag }

// highlightPrompt splits a built prompt into lines for the preview's
// section-tag highlighting, classifying each via the shared
// internal/prompthl (also used by the TUI's live preview, so both
// always highlight identically). An empty (or whitespace-only) prompt
// returns nil, letting the template distinguish "nothing built yet"
// from "built to an empty string".
func highlightPrompt(text string) []previewLine {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	lines := strings.Split(text, "\n")
	out := make([]previewLine, len(lines))
	for i, line := range lines {
		out[i] = previewLine{Text: line, Kind: prompthl.Classify(line)}
	}
	return out
}

// handlePreview renders the live-preview fragment htmx swaps into
// #preview (see the form's hx-post wiring in index.html). It runs the
// same prompt.Build the flag-only CLI path and the TUI's live preview
// already call - this is that same call, reachable over HTTP,
// rendering an HTML partial instead of JSON (this replaced the JSON
// POST /api/build once the page moved to htmx - see api.go).
//
// A build-logic error (unknown target/skill) is a routine, expected
// outcome of live preview - the user just hasn't picked valid values
// yet - so it renders inline as part of a normal 200 response: htmx
// does not swap 4xx/5xx responses by default (see htmx's Response
// Handling docs), and an un-swapped error would leave the preview pane
// silently stuck on stale content instead of showing the problem.
//
// A malformed request (unparseable form body, oversized body) is a
// genuine request error and does 400 - reaching that path requires a
// hand-crafted request; htmx's own form serialization can't produce
// one from normal use of the page.
func (app *application) handlePreview(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	out, buildErr := prompt.Build(app.reg, prompt.Inputs{
		Target:       r.FormValue("target"),
		Skills:       r.Form["skills"],
		Goal:         r.FormValue("goal"),
		Role:         r.FormValue("role"),
		Context:      r.FormValue("context"),
		Constraints:  r.FormValue("constraints"),
		OutputFormat: r.FormValue("outputFormat"),
	})

	data := previewData{}
	if buildErr != nil {
		data.Error = buildErr.Error()
	} else {
		data.Lines = highlightPrompt(out)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := app.tmpl.ExecuteTemplate(w, "preview.html", data); err != nil {
		app.serverError(w, r, err)
	}
}
