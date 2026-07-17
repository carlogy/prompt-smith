package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
)

// testApp and newLocalRequest live in testhelpers_test.go, shared with
// security_test.go and preview_test.go.

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

func TestRoutes_WrongMethodReturns405(t *testing.T) {
	cases := []struct {
		method, path string
	}{
		{http.MethodPost, "/api/registry"},
		{http.MethodGet, "/preview"},
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
