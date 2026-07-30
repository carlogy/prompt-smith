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
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/carlogy/prompt-smith/internal/prompt"
	"github.com/carlogy/prompt-smith/internal/registry"
)

func TestHandleIndex_RendersForm(t *testing.T) {
	app := testApp()
	req := newLocalRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	app.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}

	body := rec.Body.String()
	mustContain := []string{
		`<form id="prompt-form"`,
		`hx-post="/preview"`, // the live-preview wiring, proving the form is htmx-driven
		`name="target"`,
		`value="generic"`,  // a known target from the fixture registry
		`value="diagnose"`, // a known skill id from the fixture registry
		`Hard bugs.`,       // diagnose's WhenToUse, in the picker
		`<textarea id="goal"`,
		`<label for="examples"`, // Examples has a proper label/for association like every other field
		`<textarea id="examples" name="examples"`,
		`Worked examples of the output you want`, // Examples' fielddesc hint, proving it isn't hardcoded
		`navigator.clipboard`,                    // the copy button's implementation
		`select-caret`,                           // the custom dropdown chevron
		`The persona the model should adopt.`,    // a field hint, proving hints render
		`promptsmith:refresh`,                    // the custom trigger Clear fires to rebuild the preview
		`id="preview-indicator"`,                 // the htmx loading indicator
		`id="download-button"`,
		`id="clear-button"`,
		`data-skill-row`,       // the target-filtering hook on each skill row
		`flex-wrap`,            // action buttons wrap instead of overflowing on narrow viewports
		`htmx:beforeRequest`,   // aria-busy wiring around the live-preview request
		`href="#main-content"`, // skip-to-content link
		`Skip to content`,
		`id="main-content"`,   // skip link's target landmark
		`id="preview-status"`, // concise SR status region (replaces aria-live on #preview)
		`aria-labelledby="preview-heading"`,
		`id="preview-heading"`,
		`data-skill-unavailable`, // per-row SR reason for a disabled skill
		`htmx:afterSettle`,       // concise-status wiring
		`[role="alert"]`,         // afterSettle keys its announce-or-stay-silent decision off this selector
		`placeholder="e.g. fix the flaky checkout test"`,
		`placeholder="e.g. a senior Go engineer"`,
		`placeholder="e.g. checkout_test.go:42 fails ~1 in 5 in CI"`,
		`placeholder="e.g. no new dependencies; keep the public API"`,
		`placeholder="e.g. a worked input/output pair; separate multiple examples with a line containing only ---"`,
	}
	for _, want := range mustContain {
		if !strings.Contains(body, want) {
			t.Errorf("page missing %q, got:\n%s", want, body)
		}
	}

	// #preview must no longer be an aria-live region: the concise
	// #preview-status (role=status) region replaced it, so a screen
	// reader no longer re-reads the whole rebuilt prompt on every
	// keystroke. aria-live should now appear nowhere on the page.
	if strings.Contains(body, "aria-live") {
		t.Errorf("page still contains aria-live; #preview should rely on the concise #preview-status region instead")
	}

	// The old error class (formerly on the error <p> in preview.html)
	// must not creep back into the template or the afterSettle
	// selector: role="alert" is the single source of truth for a
	// preview error now, so nothing should reintroduce that class as
	// a second, driftable hook. Built by concatenation rather than as
	// a literal so this regression guard doesn't itself reintroduce
	// the retired token into the source tree.
	removedErrorClassToken := "preview" + "-error"
	if strings.Contains(body, removedErrorClassToken) {
		t.Errorf("page contains the removed error class token; role=\"alert\" should be the only error hook, not a class name")
	}

	// The picker shows each skill's WhenToUse (why to pick it), never
	// its Body (the generic-target methodology text itself) - that
	// only belongs in a built prompt, not the selection UI.
	if strings.Contains(body, "Build a feedback loop") {
		t.Error("page rendered diagnose's Body - only WhenToUse belongs in the picker")
	}
}

// TestHandleIndex_ShowsAGPLSourceNotice proves the served page carries
// an AGPL §13 offer-of-source notice: a network user must be able to
// get the program's source, so the footer must link to the repo and
// mention the AGPL. The footer is static template content (not
// registry-dependent), so this uses the synthetic testApp() fixture
// rather than loading the real registry.
func TestHandleIndex_ShowsAGPLSourceNotice(t *testing.T) {
	app := testApp()
	req := newLocalRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	app.routes().ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "https://github.com/carlogy/prompt-smith") {
		t.Errorf("page missing link to source repository, got:\n%s", body)
	}
	if !strings.Contains(body, "AGPL") {
		t.Errorf("page missing AGPL notice, got:\n%s", body)
	}
}

// TestHandleIndex_SkillRowsCarrySupportedTargets proves each skill row
// renders its own SupportedTargets (see page.go), which index.html's
// JS uses to grey out and disable a skill when the selected target
// doesn't support it - the same Registry.SupportsTarget check `list
// -t` and the TUI picker use, applied client-side here since a target
// change never round-trips to this handler.
func TestHandleIndex_SkillRowsCarrySupportedTargets(t *testing.T) {
	app := testApp()
	req := newLocalRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	app.routes().ServeHTTP(rec, req)
	body := rec.Body.String()

	// diagnose and verify both have a Body -> supported on generic
	// (inline) and opencode (reference, always supported).
	if !strings.Contains(body, `data-supported-targets="generic,opencode"`) {
		t.Errorf(`expected a skill row with data-supported-targets="generic,opencode", got:\n%s`, body)
	}
	// agent-only has no Body -> unsupported on generic (inline
	// requires one), but reference-mode opencode supports it anyway.
	if !strings.Contains(body, `data-supported-targets="opencode"`) {
		t.Errorf(`expected agent-only's row to have data-supported-targets="opencode" only, got:\n%s`, body)
	}
}

func TestHandleIndex_SeedsInitialValues(t *testing.T) {
	app := testAppWithInitial(prompt.Inputs{
		Target:       "opencode",
		Skills:       []string{"verify"},
		Goal:         "my seeded goal",
		Role:         "a seeded role",
		Context:      "seeded context",
		Constraints:  "seeded constraints",
		OutputFormat: "seeded output format",
		Examples:     []string{"first seeded example", "second seeded example"},
	})
	req := newLocalRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	app.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()

	if !strings.Contains(body, `value="opencode" selected`) {
		t.Errorf(`expected the opencode <option> to be selected, got:\n%s`, body)
	}
	if strings.Contains(body, `value="generic" selected`) {
		t.Error("expected generic NOT to be selected when opencode was seeded")
	}
	if !strings.Contains(body, `value="verify" checked`) {
		t.Errorf(`expected the verify checkbox to be checked, got:\n%s`, body)
	}
	if strings.Contains(body, `value="diagnose" checked`) {
		t.Error("expected diagnose NOT to be checked when only verify was seeded")
	}

	wantSeeded := []string{"my seeded goal", "a seeded role", "seeded context", "seeded constraints", "seeded output format"}
	for _, want := range wantSeeded {
		if !strings.Contains(body, want) {
			t.Errorf("page missing seeded value %q, got:\n%s", want, body)
		}
	}

	// Examples is seeded as prompt.JoinExamples's joined string, not
	// the raw []string - the two examples must appear separated by
	// the "---" line the textarea (and SplitExamples on the next
	// submit) both expect, not as Go's "[a b]" slice formatting.
	if !strings.Contains(body, "first seeded example\n---\nsecond seeded example") {
		t.Errorf("expected Examples seeded as one \"---\"-joined string, got:\n%s", body)
	}
}

// advancedDetailsOpenTag is the exact rendered opening tag of the
// optional-fields <details> when AdvancedOpen is true - see
// index.html. Matched as a whole to avoid false positives from any
// other "open" substring elsewhere on the page.
const advancedDetailsOpenTag = `<details class="border-t border-slate-200 pt-6 dark:border-slate-700" open>`

func TestHandleIndex_AdvancedClosedByDefault(t *testing.T) {
	app := testApp() // no seeded optional fields
	req := newLocalRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	app.routes().ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "<details") {
		t.Fatalf("page missing the optional-fields <details>, got:\n%s", body)
	}
	if strings.Contains(body, advancedDetailsOpenTag) {
		t.Errorf("optional fields rendered open with nothing seeded, got:\n%s", body)
	}
}

func TestHandleIndex_AdvancedOpenWhenSeeded(t *testing.T) {
	app := testAppWithInitial(prompt.Inputs{Role: "a seeded role"})
	req := newLocalRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	app.routes().ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, advancedDetailsOpenTag) {
		t.Errorf("expected the optional fields to render open when Role was seeded, got:\n%s", body)
	}
}

// TestHandleIndex_AdvancedOpenWhenExamplesSeeded proves Examples
// counts as a seeded optional field for AdvancedOpen's purposes just
// like Role/Context/Constraints/OutputFormat already do - it lives in
// the same "Optional fields" <details> in index.html, so a seeded
// example must expand it too, not leave it collapsed.
func TestHandleIndex_AdvancedOpenWhenExamplesSeeded(t *testing.T) {
	app := testAppWithInitial(prompt.Inputs{Examples: []string{"a seeded example"}})
	req := newLocalRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	app.routes().ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, advancedDetailsOpenTag) {
		t.Errorf("expected the optional fields to render open when Examples was seeded, got:\n%s", body)
	}
}

func TestHandleIndex_EscapesUserSuppliedContent(t *testing.T) {
	// html/template auto-escapes by construction - this proves it
	// empirically for the field most plausibly reflecting
	// attacker/user-controlled text (a goal typed at the CLI), rather
	// than just trusting the package's default behavior.
	app := testAppWithInitial(prompt.Inputs{Goal: `<script>alert(1)</script>`})
	req := newLocalRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	app.routes().ServeHTTP(rec, req)

	body := rec.Body.String()
	if strings.Contains(body, `<script>alert(1)</script>`) {
		t.Errorf("goal was rendered unescaped - XSS risk, got:\n%s", body)
	}
	if !strings.Contains(body, `&lt;script&gt;alert(1)&lt;/script&gt;`) {
		t.Errorf("expected the goal to be HTML-escaped, got:\n%s", body)
	}
}

// TestHandleIndex_RealRegistryShowsCodingLeanCode guards the real,
// embedded registry (not the synthetic fixture, which never contains
// this skill): the "coding" category and its "lean-code" skill must
// surface in the rendered index page exactly as they do for every
// other built-in category/skill, since index.html renders entirely
// off app.reg with nothing hardcoded (see page.go's handleIndex). The
// asserted substrings are the exact forms index.html emits: a skill's
// checkbox is `value="{{.ID}}"` and a category's heading is
// `<h3 ...>{{.Name}}</h3>` with .Name being the raw category string
// (uppercase is CSS-only via the "uppercase" class, not the text
// content).
func TestHandleIndex_RealRegistryShowsCodingLeanCode(t *testing.T) {
	t.Setenv("PROMPTSMITH_SKILLS_DIR", t.TempDir()) // hermetic: ignore any real user skills dir

	reg, warnings, err := registry.Load()
	if err != nil {
		t.Fatalf("registry.Load() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("registry.Load() warnings = %v, want none", warnings)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	app, err := newApplication(reg, logger, prompt.Inputs{})
	if err != nil {
		t.Fatalf("newApplication() error = %v", err)
	}

	req := newLocalRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	app.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()

	if !strings.Contains(body, `value="lean-code"`) {
		t.Errorf(`expected the lean-code checkbox value="lean-code", got:\n%s`, body)
	}
	if !strings.Contains(body, `>coding</h3>`) {
		t.Errorf(`expected a category heading rendering "coding", got:\n%s`, body)
	}
}

// TestHandleIndex_TargetOptionsShowDisplayNameNotID proves the target
// <select> renders each option's human-friendly TargetConfig.Name (see
// registry.TargetConfig.DisplayName) as its label while still
// submitting the raw id as its value - the same id handlePreview reads
// via r.FormValue("target") and passes straight to prompt.Build.
// Exercises both branches: an explicit Name (claude-code) and the
// fallback-to-id path (generic, which sets none here).
func TestHandleIndex_TargetOptionsShowDisplayNameNotID(t *testing.T) {
	reg := &registry.Registry{
		Categories: []string{"debugging"},
		Skills:     []registry.Skill{{ID: "diagnose", Category: "debugging", Body: "Build a feedback loop first."}},
		Targets: map[string]registry.TargetConfig{
			"generic":     {ID: "generic", SkillMode: "inline"},
			"claude-code": {ID: "claude-code", Name: "Claude Code", SkillMode: "reference"},
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	app, err := newApplication(reg, logger, prompt.Inputs{})
	if err != nil {
		t.Fatalf("newApplication() error = %v", err)
	}

	req := newLocalRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	app.routes().ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, `value="claude-code"`) {
		t.Errorf(`expected an <option> submitting value="claude-code", got:\n%s`, body)
	}
	if !strings.Contains(body, `>Claude Code</option>`) {
		t.Errorf(`expected claude-code's <option> to display "Claude Code", got:\n%s`, body)
	}
	if strings.Contains(body, `>claude-code</option>`) {
		t.Error("claude-code's option displayed the raw id instead of its Name")
	}
	if !strings.Contains(body, `>generic</option>`) {
		t.Errorf(`expected generic's <option> to fall back to displaying its id (no Name set), got:\n%s`, body)
	}
}
