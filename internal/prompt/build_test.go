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
	"reflect"
	"strings"
	"testing"

	"github.com/carlogy/prompt-smith/internal/prompt"
	"github.com/carlogy/prompt-smith/internal/registry"
)

// fixtureRegistry returns a minimal registry for testing prompt.Build's
// rendering behavior in isolation from real registry content. Real skill
// content is authored separately and covered by internal/registry's own
// tests plus one integration test that loads the shipped data.
func fixtureRegistry() *registry.Registry {
	return &registry.Registry{
		Categories: []string{"debugging", "testing"},
		Skills: []registry.Skill{
			{
				ID:        "diagnose",
				Name:      "Diagnose",
				Category:  "debugging",
				Order:     10,
				WhenToUse: "Hard bugs or failing tests that need a disciplined debugging loop.",
				Body:      "Build a fast, deterministic pass/fail signal before anything else. Reproduce, generate 3-5 falsifiable hypotheses, instrument one variable at a time, fix with a regression test, then remove the instrumentation.",
			},
			{
				ID:       "verify",
				Name:     "Verify",
				Category: "testing",
				Order:    20,
				Body:     "Before done: run build, lint, and tests.",
				Refs:     map[string]string{"claude-code": "verify-checks"},
			},
			{
				ID:       "audit",
				Name:     "Audit",
				Category: "testing",
				Order:    10,
				Body:     "Review the diff for security and correctness issues.",
			},
			{
				ID:       "check",
				Name:     "Check",
				Category: "testing",
				Order:    10,
				Body:     "Double-check edge cases against the spec.",
			},
			{
				ID:        "agent-only",
				Name:      "Agent Only",
				Category:  "testing",
				Order:     30,
				WhenToUse: "Only meaningful inside an agent harness.",
				// No Body: not supported on the "generic" (inline) target.
			},
		},
		Targets: map[string]registry.TargetConfig{
			"generic": {ID: "generic", Delimiter: "xml", SkillMode: "inline"},
			"opencode": {
				ID:        "opencode",
				Delimiter: "xml",
				SkillMode: "reference",
				Tools:     map[string]string{"search": "grep", "read": "read", "find": "glob"},
			},
			"claude-code": {
				ID:        "claude-code",
				Delimiter: "xml",
				SkillMode: "reference",
				Tools:     map[string]string{"search": "Grep", "read": "Read", "find": "Glob"},
			},
			"gemini-cli": {
				ID:        "gemini-cli",
				Delimiter: "xml",
				SkillMode: "reference",
				Tools:     map[string]string{"search": "grep_search", "read": "read_file", "find": "glob"},
			},
			"codex": {
				ID:        "codex",
				Delimiter: "xml",
				SkillMode: "reference",
			},
		},
	}
}

// bodyIndex returns the position of needle in got, failing the test if
// it's not found — used to assert relative ordering between skill bodies.
func bodyIndex(t *testing.T, got, needle string) int {
	t.Helper()
	i := strings.Index(got, needle)
	if i < 0 {
		t.Fatalf("expected output to contain %q, got:\n%s", needle, got)
	}
	return i
}

func TestBuild_GoalOnlyWithOneInlineSkill(t *testing.T) {
	reg := fixtureRegistry()

	got, err := prompt.Build(reg, prompt.Inputs{
		Target: "generic",
		Skills: []string{"diagnose"},
		Goal:   "Fix the flaky checkout test.",
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	assertGolden(t, "goal_only_one_inline_skill", got)
}

func TestBuild_UnknownTargetErrors(t *testing.T) {
	reg := fixtureRegistry()

	_, err := prompt.Build(reg, prompt.Inputs{
		Target: "does-not-exist",
		Goal:   "Fix the flaky checkout test.",
	})
	if err == nil {
		t.Fatal("Build() error = nil, want an error for an unknown target")
	}
}

func TestBuild_UnknownSkillErrors(t *testing.T) {
	reg := fixtureRegistry()

	tests := []struct {
		name    string
		skills  []string
		wantErr string // exact error message, or "" to just check err != nil
	}{
		{
			name:   "unknown id",
			skills: []string{"does-not-exist"},
		},
		{
			name:    "whitespace-padded id reports the trimmed id",
			skills:  []string{" nope "},
			wantErr: `prompt: unknown skill "nope"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := prompt.Build(reg, prompt.Inputs{
				Target: "generic",
				Skills: tt.skills,
				Goal:   "Fix the flaky checkout test.",
			})
			if err == nil {
				t.Fatal("Build() error = nil, want an error for an unknown skill")
			}
			if tt.wantErr != "" && err.Error() != tt.wantErr {
				t.Errorf("Build() error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestBuild_AllOptionalFieldsPresent(t *testing.T) {
	reg := fixtureRegistry()

	got, err := prompt.Build(reg, prompt.Inputs{
		Target:       "generic",
		Skills:       []string{"diagnose"},
		Goal:         "Fix the flaky checkout test.",
		Role:         "You are a senior Go engineer meticulous about concurrency.",
		Context:      "checkout_test.go:42 fails ~1 in 5 in CI; passes locally; suspected race.",
		Constraints:  "Don't change assertions; add no new dependencies.",
		OutputFormat: "Return the root cause and the fix as a unified diff.",
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	assertGolden(t, "all_optional_fields_present", got)
}

func TestBuild_OrdersSkillsByCategoryThenWeightThenID(t *testing.T) {
	reg := fixtureRegistry()

	t.Run("cross-category: category order wins over selection order", func(t *testing.T) {
		got, err := prompt.Build(reg, prompt.Inputs{
			Target: "generic",
			Skills: []string{"verify", "diagnose"}, // selected testing-then-debugging
			Goal:   "goal",
		})
		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}
		// "debugging" is categories[0], "testing" is categories[1]: diagnose
		// must render first regardless of selection order.
		if bodyIndex(t, got, "pass/fail signal") > bodyIndex(t, got, "run build, lint") {
			t.Errorf("expected diagnose (debugging) before verify (testing):\n%s", got)
		}
	})

	t.Run("same category: order weight wins over selection order", func(t *testing.T) {
		got, err := prompt.Build(reg, prompt.Inputs{
			Target: "generic",
			Skills: []string{"verify", "audit"}, // verify=20, audit=10
			Goal:   "goal",
		})
		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}
		if bodyIndex(t, got, "security and correctness") > bodyIndex(t, got, "run build, lint") {
			t.Errorf("expected audit (order 10) before verify (order 20):\n%s", got)
		}
	})

	t.Run("same category and order: id tiebreak wins over selection order", func(t *testing.T) {
		got, err := prompt.Build(reg, prompt.Inputs{
			Target: "generic",
			Skills: []string{"check", "audit"}, // both order=10; "audit" < "check"
			Goal:   "goal",
		})
		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}
		if bodyIndex(t, got, "security and correctness") > bodyIndex(t, got, "edge cases") {
			t.Errorf("expected audit before check (id tiebreak):\n%s", got)
		}
	})
}

func TestBuild_DedupesRepeatedSkills(t *testing.T) {
	reg := fixtureRegistry()

	got, err := prompt.Build(reg, prompt.Inputs{
		Target: "generic",
		Skills: []string{"diagnose", "diagnose"},
		Goal:   "goal",
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if n := strings.Count(got, "pass/fail signal"); n != 1 {
		t.Errorf("expected diagnose's body to appear once, appeared %d times:\n%s", n, got)
	}
}

func TestBuild_ReferenceModeTargetDerivesSnippetAndTools(t *testing.T) {
	reg := fixtureRegistry()

	got, err := prompt.Build(reg, prompt.Inputs{
		Target: "opencode",
		Skills: []string{"diagnose"},
		Goal:   "Fix the flaky checkout test.",
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	assertGolden(t, "opencode_reference_and_tools", got)
}

func TestBuild_GeminiCLIDerivesSnippetAndTools(t *testing.T) {
	reg := fixtureRegistry()

	got, err := prompt.Build(reg, prompt.Inputs{
		Target: "gemini-cli",
		Skills: []string{"diagnose"},
		Goal:   "Fix the flaky checkout test.",
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	assertGolden(t, "gemini_cli_reference_and_tools", got)
}

func TestBuild_CodexDerivesReferenceNoTools(t *testing.T) {
	reg := fixtureRegistry()

	got, err := prompt.Build(reg, prompt.Inputs{
		Target: "codex",
		Skills: []string{"diagnose"},
		Goal:   "Fix the flaky checkout test.",
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	// Codex CLI is shell-centric (no discrete search/read/find tools),
	// so unlike the other reference targets it renders NO <tools>
	// section - reference mode still applies to the skills themselves.
	if strings.Contains(got, "<tools>") {
		t.Errorf("codex should emit no <tools> section (shell-centric), got:\n%s", got)
	}

	assertGolden(t, "codex_reference_no_tools", got)
}

func TestBuild_ClaudeCodeUsesRefOverride(t *testing.T) {
	reg := fixtureRegistry()

	got, err := prompt.Build(reg, prompt.Inputs{
		Target: "claude-code",
		Skills: []string{"verify"},
		Goal:   "goal",
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if !strings.Contains(got, "`verify-checks`") {
		t.Errorf("expected claude-code's ref override %q in output, got:\n%s", "verify-checks", got)
	}
	if strings.Contains(got, "`verify`") {
		t.Errorf("expected the unrenamed id NOT to appear, got:\n%s", got)
	}
}

func TestBuild_SkillUnsupportedOnInlineTargetErrors(t *testing.T) {
	reg := fixtureRegistry()

	_, err := prompt.Build(reg, prompt.Inputs{
		Target: "generic",
		Skills: []string{"agent-only"}, // has no Body
		Goal:   "goal",
	})
	if err == nil {
		t.Fatal("Build() error = nil, want an error for a skill with no generic body")
	}
}

func TestNormalizeSkills(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{"leading whitespace trimmed", []string{" diagnose"}, []string{"diagnose"}},
		{"trailing whitespace trimmed", []string{"diagnose "}, []string{"diagnose"}},
		{"surrounding whitespace trimmed", []string{"  diagnose  "}, []string{"diagnose"}},
		{"trailing empty entry dropped", []string{"diagnose", ""}, []string{"diagnose"}},
		{"single empty entry dropped", []string{""}, []string{}},
		{"multiple empty entries dropped", []string{"", ""}, []string{}},
		{"whitespace-only entry dropped", []string{"  "}, []string{}},
		{"nil input", nil, []string{}},
		{"order preserved across mixed entries", []string{" c", "a ", "  b  "}, []string{"c", "a", "b"}},
		{"does not dedupe", []string{"a", "a"}, []string{"a", "a"}},
		{"does not case-fold", []string{"Diagnose"}, []string{"Diagnose"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := prompt.NormalizeSkills(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NormalizeSkills(%#v) = %#v, want %#v", tt.in, got, tt.want)
			}
		})
	}
}

func TestBuild_NormalizesSkillIDs(t *testing.T) {
	reg := fixtureRegistry()

	got, err := prompt.Build(reg, prompt.Inputs{
		Target: "generic",
		Skills: []string{" diagnose ", "", "diagnose"},
		Goal:   "goal",
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if n := strings.Count(got, "pass/fail signal"); n != 1 {
		t.Errorf("expected diagnose's body to appear once, appeared %d times:\n%s", n, got)
	}
}

func TestBuild_AllSkillsEmptyRendersNoApproachSection(t *testing.T) {
	reg := fixtureRegistry()

	got, err := prompt.Build(reg, prompt.Inputs{
		Target: "generic",
		Skills: []string{"", "  "},
		Goal:   "goal",
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if strings.Contains(got, "<approach>") {
		t.Errorf("expected no <approach> section when all skills are empty, got:\n%s", got)
	}
}
