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
		`<form id="prompt-form">`,
		`name="target"`,
		`value="generic"`,  // a known target from the fixture registry
		`value="diagnose"`, // a known skill id from the fixture registry
		`Hard bugs.`,       // diagnose's WhenToUse, in the picker
		`<textarea id="goal"`,
		`fetch("/api/build"`,  // the live-preview script, proving it's wired to the real API
		`navigator.clipboard`, // the copy button's implementation
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
