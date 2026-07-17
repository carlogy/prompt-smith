package server

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/carlogy/prompt-smith/internal/registry"
)

// testApp builds an application against a small synthetic registry -
// same fixture style as internal/registry's own tests - plus a
// discard logger so tests never spam output for expected error paths
// (e.g. the 500 test).
func testApp() *application {
	reg := &registry.Registry{
		Categories: []string{"debugging", "testing"},
		Skills: []registry.Skill{
			{ID: "diagnose", Name: "Diagnose", Category: "debugging", Order: 10, WhenToUse: "Hard bugs.", Body: "Build a feedback loop first."},
			{ID: "verify", Name: "Verify", Category: "testing", Order: 10, WhenToUse: "Before done.", Body: "Run the checks."},
			{ID: "agent-only", Name: "Agent Only", Category: "testing", Order: 20, WhenToUse: "Agent harnesses only."}, // no Body
		},
		Targets: map[string]registry.TargetConfig{
			"generic":  {ID: "generic", SkillMode: "inline"},
			"opencode": {ID: "opencode", SkillMode: "reference"},
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return newApplication(reg, logger)
}

func TestHandleRegistry(t *testing.T) {
	app := testApp()
	req := httptest.NewRequest(http.MethodGet, "/api/registry", nil)
	rec := httptest.NewRecorder()

	app.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp registryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Unmarshal() error = %v, body = %s", err, rec.Body.String())
	}

	if !slices.Equal(resp.Categories, []string{"debugging", "testing"}) {
		t.Errorf("Categories = %v, want [debugging testing]", resp.Categories)
	}

	wantTargets := []string{"generic", "opencode"}
	var gotTargets []string
	for _, td := range resp.Targets {
		gotTargets = append(gotTargets, td.ID)
	}
	if !slices.Equal(gotTargets, wantTargets) {
		t.Errorf("Targets = %v, want %v (alphabetical)", gotTargets, wantTargets)
	}

	if len(resp.Skills) != 3 {
		t.Fatalf("len(Skills) = %d, want 3", len(resp.Skills))
	}
	// Canonical order: category position (debugging < testing), then
	// Order weight - same as SortSkills everywhere else.
	wantOrder := []string{"diagnose", "verify", "agent-only"}
	var gotOrder []string
	for _, sk := range resp.Skills {
		gotOrder = append(gotOrder, sk.ID)
	}
	if !slices.Equal(gotOrder, wantOrder) {
		t.Errorf("skill order = %v, want %v", gotOrder, wantOrder)
	}

	byID := make(map[string]skillDTO, len(resp.Skills))
	for _, sk := range resp.Skills {
		byID[sk.ID] = sk
	}
	if !slices.Equal(byID["diagnose"].SupportedTargets, []string{"generic", "opencode"}) {
		t.Errorf("diagnose.SupportedTargets = %v, want both (has a body)", byID["diagnose"].SupportedTargets)
	}
	if !slices.Equal(byID["agent-only"].SupportedTargets, []string{"opencode"}) {
		t.Errorf("agent-only.SupportedTargets = %v, want [opencode] only (no body -> unsupported on inline)", byID["agent-only"].SupportedTargets)
	}
}

func TestHandleBuild_Success(t *testing.T) {
	app := testApp()
	body := `{"target":"generic","skills":["diagnose"],"goal":"fix the flaky test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/build", strings.NewReader(body))
	rec := httptest.NewRecorder()

	app.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var resp buildResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if resp.Error != "" {
		t.Errorf("Error = %q, want empty", resp.Error)
	}
	if !strings.Contains(resp.Prompt, "fix the flaky test") || !strings.Contains(resp.Prompt, "Build a feedback loop first.") {
		t.Errorf("Prompt missing expected content, got:\n%s", resp.Prompt)
	}
}

func TestHandleBuild_UnknownSkillIsA200WithError(t *testing.T) {
	// A build-logic error (bad target/skill) is an expected, routine
	// outcome of live preview - not a malformed request - so it must
	// stay 200 with the error in the body, never a 4xx.
	app := testApp()
	body := `{"target":"generic","skills":["does-not-exist"],"goal":"x"}`
	req := httptest.NewRequest(http.MethodPost, "/api/build", strings.NewReader(body))
	rec := httptest.NewRecorder()

	app.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (build errors are not request errors), body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var resp buildResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if resp.Error == "" {
		t.Error("Error is empty, want a message about the unknown skill")
	}
}

func TestHandleBuild_RequestShapeErrors(t *testing.T) {
	cases := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{"malformed JSON", `{"target": `, http.StatusBadRequest},
		{"unknown field", `{"target":"generic","bogus":true}`, http.StatusBadRequest},
		{"empty body", ``, http.StatusBadRequest},
		{"trailing data", `{"target":"generic"}{}`, http.StatusBadRequest},
		{"oversized body", `{"goal":"` + strings.Repeat("x", maxRequestBody+1) + `"}`, http.StatusBadRequest},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := testApp()
			req := httptest.NewRequest(http.MethodPost, "/api/build", strings.NewReader(tc.body))
			rec := httptest.NewRecorder()

			app.routes().ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d, body = %s", rec.Code, tc.wantStatus, rec.Body.String())
			}
			var resp errorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("response body isn't valid JSON: %v, body = %s", err, rec.Body.String())
			}
			if resp.Error == "" {
				t.Error("Error is empty, want a message")
			}
		})
	}
}

func TestRoutes_WrongMethodReturns405(t *testing.T) {
	cases := []struct {
		method, path string
	}{
		{http.MethodPost, "/api/registry"},
		{http.MethodGet, "/api/build"},
		{http.MethodDelete, "/api/registry"},
	}

	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			app := testApp()
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rec := httptest.NewRecorder()

			app.routes().ServeHTTP(rec, req)

			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
			}
		})
	}
}

func TestHandleBuild_RejectsUnknownContentSilently(t *testing.T) {
	// readJSON doesn't require a Content-Type header - the body is
	// still valid JSON regardless of what the client claims it is.
	app := testApp()
	req := httptest.NewRequest(http.MethodPost, "/api/build", bytes.NewBufferString(`{"target":"generic","goal":"x"}`))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()

	app.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}
