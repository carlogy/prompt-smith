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

package prompt_test

import (
	"strings"
	"testing"

	"github.com/carlogy/prompt-smith/internal/prompt"
	"github.com/carlogy/prompt-smith/internal/registry"
)

// TestBuild_WithRealRegistry proves the engine and the real shipped
// content work together end-to-end for every target, not just against
// the fixture registry used by the rest of this package's tests.
//
// PROMPTSMITH_SKILLS_DIR is pinned to an empty temp directory so this
// stays hermetic regardless of the developer machine's real user
// skills directory.
func TestBuild_WithRealRegistry(t *testing.T) {
	t.Setenv("PROMPTSMITH_SKILLS_DIR", t.TempDir())

	reg, warnings, err := registry.Load()
	if err != nil {
		t.Fatalf("registry.Load() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("registry.Load() warnings = %v, want none", warnings)
	}

	tests := []struct {
		target string
		skill  string
		want   string // substring expected somewhere in the output
	}{
		{"generic", "diagnose", "pass/fail"},                        // inlined methodology
		{"opencode", "diagnose", "Load the `diagnose` skill"},       // reference mode
		{"claude-code", "verify", "Load the `verify-checks` skill"}, // renamed ref
		{"gemini-cli", "diagnose", "Load the `diagnose` skill"},     // reference mode
		{"codex", "diagnose", "Load the `diagnose` skill"},          // reference mode, no tools
	}

	for _, tt := range tests {
		t.Run(tt.target, func(t *testing.T) {
			got, err := prompt.Build(reg, prompt.Inputs{
				Target: tt.target,
				Skills: []string{tt.skill},
				Goal:   "Fix the flaky checkout test.",
			})
			if err != nil {
				t.Fatalf("Build() error = %v", err)
			}
			if !strings.Contains(got, tt.want) {
				t.Errorf("Build() output missing %q:\n%s", tt.want, got)
			}
		})
	}
}
