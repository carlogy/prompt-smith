package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

// testApp and newLocalRequest live in testhelpers_test.go, shared with
// security_test.go.

func TestHandleRegistry(t *testing.T) {
	app := testApp()
	req := newLocalRequest(http.MethodGet, "/api/registry", nil)
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
	req := newLocalRequest(http.MethodPost, "/api/build", strings.NewReader(body))
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
	req := newLocalRequest(http.MethodPost, "/api/build", strings.NewReader(body))
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
			req := newLocalRequest(http.MethodPost, "/api/build", strings.NewReader(tc.body))
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
		{http.MethodPost, "/"},
	}

	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			app := testApp()
			req := newLocalRequest(tc.method, tc.path, nil)
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
	req := newLocalRequest(http.MethodPost, "/api/build", bytes.NewBufferString(`{"target":"generic","goal":"x"}`))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()

	app.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

// TestRoutes_UnmatchedPathReturns404 guards the "/{$}" pattern used
// for the index page (see app.routes): a plain "/" pattern would match
// as a subtree (per net/http's ServeMux docs, any pattern ending in
// "/" matches everything under it), silently serving the full index
// page for any unrelated, undefined path. "/{$}" is the Go 1.22+
// exact-match escape hatch for exactly this case.
func TestRoutes_UnmatchedPathReturns404(t *testing.T) {
	app := testApp()
	req := newLocalRequest(http.MethodGet, "/no-such-path", nil)
	rec := httptest.NewRecorder()

	app.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d, body = %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}
