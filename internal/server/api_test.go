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
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/carlogy/prompt-smith/internal/prompt"
	"github.com/carlogy/prompt-smith/internal/registry"
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

// TestHandleRegistry_RealRegistryIncludesCodingLeanCode guards the
// real, embedded registry (not the synthetic fixture, which never
// contains this skill): the "coding" category and its "lean-code"
// skill must surface in the /api/registry JSON exactly as they do for
// every other built-in category/skill, since this handler renders
// entirely off app.reg with nothing hardcoded. If this ever fails,
// either the category was dropped from registry.Categories, the skill
// lost its id/category, or handleRegistry stopped being fully
// dynamic - all regressions this test exists to catch.
func TestHandleRegistry_RealRegistryIncludesCodingLeanCode(t *testing.T) {
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

	if !slices.Contains(resp.Categories, "coding") {
		t.Errorf("Categories = %v, want it to contain %q", resp.Categories, "coding")
	}

	var found bool
	for _, sk := range resp.Skills {
		if sk.ID == "lean-code" {
			found = true
			if sk.Category != "coding" {
				t.Errorf(`lean-code.Category = %q, want "coding"`, sk.Category)
			}
			break
		}
	}
	if !found {
		t.Errorf("Skills = %v, want a skill with ID %q", resp.Skills, "lean-code")
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
