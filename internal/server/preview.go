// promptsmith - generate portable, skill-aware prompts for any LLM or agent harness.
// Copyright (C) 2026 carlogy
// SPDX-License-Identifier: AGPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package server

import (
	"net/http"
	"strings"
	"time"

	"github.com/carlogy/prompt-smith/internal/naming"
	"github.com/carlogy/prompt-smith/internal/prompt"
	"github.com/carlogy/prompt-smith/internal/prompthl"
	"github.com/carlogy/prompt-smith/internal/promptlint"
)

// previewData is what the preview partial (assets/templates/preview.html)
// renders from.
type previewData struct {
	Lines    []previewLine
	Error    string
	Filename string // suggested Download filename - see naming.SuggestFilename
	// Findings holds every promptlint.Finding for the built prompt,
	// rendered as its own list item in preview.html. This deliberately
	// does NOT collapse the three pure-absence findings (no role, no
	// output_format, no examples) into one line the way the CLI's
	// warnLintFindings (internal/cli/generate.go) does: that collapse
	// exists because a terminal's stderr hint is space-constrained,
	// and this pane isn't - it has room to list every finding on its
	// own line.
	Findings []promptlint.Finding
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

	// Hoisted into a local rather than built inline for prompt.Build
	// and then separately again for promptlint.Check below: both calls
	// have to see the identical Inputs value, or the lint findings
	// could describe a prompt that isn't the one actually rendered -
	// the same reasoning behind the analogous "in" local in
	// internal/cli/generate.go's runGenerate.
	in := prompt.Inputs{
		Target:       r.FormValue("target"),
		Skills:       r.Form["skills"],
		Goal:         r.FormValue("goal"),
		Role:         r.FormValue("role"),
		Context:      r.FormValue("context"),
		Constraints:  r.FormValue("constraints"),
		OutputFormat: r.FormValue("outputFormat"),
		// r.FormValue, not r.Form["examples"] like skills' multi-value
		// checkbox handling: the examples textarea is ONE form field
		// holding every example, "---"-separated (see index.html and
		// fielddesc.Examples's hint text), so there's exactly one
		// "examples" key to read - SplitExamples does the dividing
		// prompt.Inputs.Examples ([]string) actually needs.
		Examples: prompt.SplitExamples(r.FormValue("examples")),
	}

	out, buildErr := prompt.Build(app.reg, in)

	data := previewData{Filename: naming.SuggestFilename(r.FormValue("goal"), time.Now())}
	if buildErr != nil {
		data.Error = buildErr.Error()
	} else {
		data.Lines = highlightPrompt(out)
		// Findings are populated only on a successful build and only
		// when hints aren't suppressed. On a build error, the template's
		// {{if .Error}} branch owns the whole pane (see preview.html) -
		// stacking advisory suggestions underneath a hard error would be
		// noise when the user simply hasn't picked valid values yet, not
		// useful context for fixing that error.
		if !app.noHints {
			data.Findings = promptlint.Check(app.reg, in)
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := app.tmpl.ExecuteTemplate(w, "preview.html", data); err != nil {
		app.serverError(w, r, err)
	}
}
