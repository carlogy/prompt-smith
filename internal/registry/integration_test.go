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

package registry_test

import (
	"testing"

	"github.com/carlogy/prompt-smith/internal/registry"
)

// TestLoad_RealRegistryIsValid guards the actual shipped, embedded data:
// it must parse and pass Validate(), and must contain what prompt.Build
// depends on for each target's rendering mode. This is what the
// `validate` CLI command runs before a rebuild ships.
//
// PROMPTSMITH_SKILLS_DIR is pinned to an empty temp directory so this
// stays hermetic regardless of the developer machine's real user skills
// directory - this test guards the embedded data specifically, not a
// merge (see userskills_test.go and cli/integration_test.go for that).
func TestLoad_RealRegistryIsValid(t *testing.T) {
	t.Setenv("PROMPTSMITH_SKILLS_DIR", t.TempDir())

	reg, warnings, err := registry.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("Load() warnings = %v, want none", warnings)
	}
	if err := reg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	if len(reg.Skills) != 14 {
		t.Errorf("len(Skills) = %d, want 14", len(reg.Skills))
	}

	for _, target := range []string{"generic", "opencode", "claude-code", "gemini-cli", "codex"} {
		if _, ok := reg.Targets[target]; !ok {
			t.Errorf("expected target %q to be defined", target)
		}
	}

	// Every shipped skill must have a non-empty generic body: this
	// registry has no agent-only skills yet, so every skill must render
	// on the "generic" (inline) target.
	for _, sk := range reg.Skills {
		if sk.Body == "" {
			t.Errorf("skill %q has no generic body", sk.ID)
		}
	}

	// "verify" carries the claude-code rename (verify -> verify-checks)
	// this design exists to exercise; guard it explicitly.
	verify, ok := reg.SkillByID("verify")
	if !ok {
		t.Fatal(`expected skill "verify" to be loaded`)
	}
	if verify.Refs["claude-code"] != "verify-checks" {
		t.Errorf(`verify.Refs["claude-code"] = %q, want "verify-checks"`, verify.Refs["claude-code"])
	}

	// "lean-code" is the new coding-category skill; guard its category,
	// body, and that it carries no target-specific refs.
	leanCode, ok := reg.SkillByID("lean-code")
	if !ok {
		t.Fatal(`expected skill "lean-code" to be loaded`)
	}
	if leanCode.Category != "coding" {
		t.Errorf(`lean-code.Category = %q, want "coding"`, leanCode.Category)
	}
	if leanCode.Body == "" {
		t.Error("lean-code.Body is empty, want non-empty")
	}
	if len(leanCode.Refs) != 0 {
		t.Errorf("lean-code.Refs = %v, want none", leanCode.Refs)
	}
}

// TestLoad_RealRegistry_ResearchAndSafetyCategories guards the "research"
// and "safety" categories (and their skills) added alongside "planning"
// and "git" respectively: both must be declared, and each of their
// skills must resolve with the right category, a non-empty body, and no
// target-specific refs.
func TestLoad_RealRegistry_ResearchAndSafetyCategories(t *testing.T) {
	t.Setenv("PROMPTSMITH_SKILLS_DIR", t.TempDir())

	reg, _, err := registry.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	wantCategories := map[string]bool{"research": false, "safety": false}
	for _, c := range reg.Categories {
		if _, ok := wantCategories[c]; ok {
			wantCategories[c] = true
		}
	}
	for c, found := range wantCategories {
		if !found {
			t.Errorf("expected category %q to be declared, got categories %v", c, reg.Categories)
		}
	}

	cases := []struct {
		id       string
		category string
	}{
		{"quote-grounding", "research"},
		{"generalize-not-hardcode", "testing"},
		{"safe-actions", "safety"},
	}
	for _, tc := range cases {
		sk, ok := reg.SkillByID(tc.id)
		if !ok {
			t.Errorf("expected skill %q to be loaded", tc.id)
			continue
		}
		if sk.Category != tc.category {
			t.Errorf("%s.Category = %q, want %q", tc.id, sk.Category, tc.category)
		}
		if sk.Body == "" {
			t.Errorf("%s.Body is empty, want non-empty", tc.id)
		}
		if len(sk.Refs) != 0 {
			t.Errorf("%s.Refs = %v, want none", tc.id, sk.Refs)
		}
	}
}
