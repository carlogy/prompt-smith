package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/carlogy/prompt-smith/internal/prompt"
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
		`navigator.clipboard`,                 // the copy button's implementation
		`select-caret`,                        // the custom dropdown chevron
		`The persona the model should adopt.`, // a field hint, proving hints render
		`promptsmith:refresh`,                 // the custom trigger Clear fires to rebuild the preview
		`id="preview-indicator"`,              // the htmx loading indicator
		`id="download-button"`,
		`id="clear-button"`,
		`data-skill-row`, // the target-filtering hook on each skill row
	}
	for _, want := range mustContain {
		if !strings.Contains(body, want) {
			t.Errorf("page missing %q, got:\n%s", want, body)
		}
	}

	// The picker shows each skill's WhenToUse (why to pick it), never
	// its Body (the generic-target methodology text itself) - that
	// only belongs in a built prompt, not the selection UI.
	if strings.Contains(body, "Build a feedback loop") {
		t.Error("page rendered diagnose's Body - only WhenToUse belongs in the picker")
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
