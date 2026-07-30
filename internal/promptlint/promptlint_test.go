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

package promptlint_test

import (
	"fmt"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/carlogy/prompt-smith/internal/prompt"
	"github.com/carlogy/prompt-smith/internal/promptlint"
	"github.com/carlogy/prompt-smith/internal/registry"
)

// emptyReg is a valid, non-nil, empty registry. It's what rule 1-5
// tests pass to Check: checkOversizedPrompt looks up in.Target in
// reg.Targets, and a read on a nil map (Registry's zero-value Targets
// field) is safe and simply misses, so this keeps rules 1-5 - which
// need no registry at all - testable without hand-building one.
var emptyReg = &registry.Registry{}

// findRule returns the first finding with the given rule, if any.
func findRule(findings []promptlint.Finding, rule promptlint.RuleID) (promptlint.Finding, bool) {
	for _, f := range findings {
		if f.Rule == rule {
			return f, true
		}
	}
	return promptlint.Finding{}, false
}

// TestCheck_NegativeConstraints_FalsePositives is the dedicated rule-1
// false-positive table. The first four cases are mandated exactly as
// specified; the rest cover bullet lists, numeric ordinals, and
// leading-conjunction stripping.
func TestCheck_NegativeConstraints_FalsePositives(t *testing.T) {
	cases := []struct {
		name        string
		constraints string
		wantFinding bool
	}{
		{
			name:        "single clause never fires, even if negative",
			constraints: "Don't break the build.",
			wantFinding: false, // 1 clause
		},
		{
			name:        "two clauses, only one negative, does not fire",
			constraints: "Don't break the build; keep tests green.",
			wantFinding: false, // 2 clauses, 1 negative
		},
		{
			name:        "this repo's own golden constraints value does not fire",
			constraints: "Don't change assertions; add no new dependencies.",
			wantFinding: false, // 2 clauses; "no" isn't clause-initial in clause 2
		},
		{
			name:        "every clause negative fires",
			constraints: "Don't use markdown. Don't be verbose. Never add dependencies.",
			wantFinding: true, // 3 clauses, all negative
		},
		{
			name:        "bulleted list of prohibitions fires",
			constraints: "- Don't use tabs\n- Don't skip tests\n- Don't merge without review",
			wantFinding: true, // 3 clauses (newline-split), bullet stripped, all negative
		},
		{
			name:        "numbered ordinal list of prohibitions fires",
			constraints: "1. Don't use tabs\n2. Never skip tests",
			wantFinding: true, // 2 clauses, ordinal stripped, all negative
		},
		{
			name:        "leading 'and' conjunction stripped before matching",
			constraints: "Don't use tabs. And don't skip tests.",
			wantFinding: true, // without conjunction stripping, clause 2 starts with "and", not a marker
		},
		{
			name:        "leading 'but' conjunction stripped before matching",
			constraints: "Don't use tabs; but never skip tests.",
			wantFinding: true, // without conjunction stripping, clause 2 starts with "but", not a marker
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := prompt.Inputs{Constraints: tc.constraints}
			findings := promptlint.Check(emptyReg, in)
			_, got := findRule(findings, promptlint.RuleNegativeConstraints)
			if got != tc.wantFinding {
				t.Errorf("negative-constraints finding = %v, want %v (constraints %q)", got, tc.wantFinding, tc.constraints)
			}
		})
	}
}

func TestCheck_NoRole(t *testing.T) {
	cases := []struct {
		name string
		role string
		want bool
	}{
		{"empty role fires", "", true},
		{"whitespace-only role fires", "   ", true},
		{"non-empty role is silent", "Senior Go engineer", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := promptlint.Check(emptyReg, prompt.Inputs{Role: tc.role})
			_, got := findRule(findings, promptlint.RuleNoRole)
			if got != tc.want {
				t.Errorf("no-role finding = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCheck_NoOutputFormat(t *testing.T) {
	cases := []struct {
		name         string
		outputFormat string
		want         bool
	}{
		{"empty output_format fires", "", true},
		{"whitespace-only output_format fires", "\t\n", true},
		{"non-empty output_format is silent", "Unified diff", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := promptlint.Check(emptyReg, prompt.Inputs{OutputFormat: tc.outputFormat})
			_, got := findRule(findings, promptlint.RuleNoOutputFormat)
			if got != tc.want {
				t.Errorf("no-output-format finding = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestCheck_Examples covers all three example bands: 0 (RuleNoExamples),
// 1 and 2 (RuleFewExamples, singular/plural), and 3+ (silent).
func TestCheck_Examples(t *testing.T) {
	cases := []struct {
		name        string
		examples    []string
		wantRule    promptlint.RuleID
		wantFinding bool
		wantMessage string
	}{
		{
			name:        "zero examples fires no-examples",
			examples:    nil,
			wantRule:    promptlint.RuleNoExamples,
			wantFinding: true,
			wantMessage: "No examples given; 3-5 worked examples measurably improve output.",
		},
		{
			name:        "one example fires few-examples, singular",
			examples:    []string{"one"},
			wantRule:    promptlint.RuleFewExamples,
			wantFinding: true,
			wantMessage: "Only 1 example given; 3-5 works best.",
		},
		{
			name:        "two examples fires few-examples, plural",
			examples:    []string{"one", "two"},
			wantRule:    promptlint.RuleFewExamples,
			wantFinding: true,
			wantMessage: "Only 2 examples given; 3-5 works best.",
		},
		{
			name:        "three examples is silent",
			examples:    []string{"one", "two", "three"},
			wantFinding: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := promptlint.Check(emptyReg, prompt.Inputs{Examples: tc.examples})
			f, gotNoExamples := findRule(findings, promptlint.RuleNoExamples)
			g, gotFewExamples := findRule(findings, promptlint.RuleFewExamples)
			got := gotNoExamples || gotFewExamples
			if got != tc.wantFinding {
				t.Fatalf("examples finding = %v, want %v", got, tc.wantFinding)
			}
			if !tc.wantFinding {
				return
			}
			var gotFinding promptlint.Finding
			if gotNoExamples {
				gotFinding = f
			} else {
				gotFinding = g
			}
			if gotFinding.Rule != tc.wantRule {
				t.Errorf("Rule = %q, want %q", gotFinding.Rule, tc.wantRule)
			}
			if gotFinding.Message != tc.wantMessage {
				t.Errorf("Message = %q, want %q", gotFinding.Message, tc.wantMessage)
			}
		})
	}
}

// TestCheck_ShortGoal pins the 14/15-character boundary exactly, plus
// the empty-goal exemption (an empty goal is a hard error elsewhere in
// the CLI, not a lint finding).
func TestCheck_ShortGoal(t *testing.T) {
	cases := []struct {
		name string
		goal string
		want bool
	}{
		{"empty goal does not fire", "", false},
		{"14 characters fires", strings.Repeat("a", 14), true},
		{"15 characters does not fire", strings.Repeat("a", 15), false},
		{"README example 'fix the bug' (11 chars) fires", "fix the bug", true},
		{"README example 'fix the flaky checkout test' (27 chars) is silent", "fix the flaky checkout test", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := promptlint.Check(emptyReg, prompt.Inputs{Goal: tc.goal})
			_, got := findRule(findings, promptlint.RuleShortGoal)
			if got != tc.want {
				t.Errorf("short-goal finding = %v, want %v (goal %q, len %d)", got, tc.want, tc.goal, len(tc.goal))
			}
		})
	}
}

func TestCheck_ShortGoal_MessageIncludesCharCount(t *testing.T) {
	findings := promptlint.Check(emptyReg, prompt.Inputs{Goal: "fix the bug"})
	f, ok := findRule(findings, promptlint.RuleShortGoal)
	if !ok {
		t.Fatal("expected short-goal finding")
	}
	want := "The goal is only 11 characters; a more specific goal produces better output."
	if f.Message != want {
		t.Errorf("Message = %q, want %q", f.Message, want)
	}
}

// rule6FS returns a synthetic, in-memory registry filesystem for
// testing the oversized-prompt rule, with skill bodies sized to
// straddle maxInlinePromptChars (8000).
//
// This deliberately does NOT use the real bundled registry
// (registry.Load): a future phase adding a 15th skill would silently
// change what this test measures (see checkOversizedPrompt's own doc
// comment on why prompt.Build is called rather than an estimate
// re-derived here). A synthetic fixture keeps this test's meaning
// fixed regardless of what the shipped registry grows into.
func rule6FS() fstest.MapFS {
	return fstest.MapFS{
		"skills.yaml": &fstest.MapFile{Data: []byte(`
categories:
  - test
skills:
  - id: big
    name: Big
    category: test
    order: 0
    body: bodies/big.md
  - id: small
    name: Small
    category: test
    order: 1
    body: bodies/small.md
`)},
		"targets.yaml": &fstest.MapFile{Data: []byte(`
targets:
  - id: inline-target
    delimiter: xml
    skill_mode: inline
  - id: ref-target
    delimiter: xml
    skill_mode: reference
`)},
		// 9000 chars, comfortably over the 8000 threshold once
		// wrapped in <approach>...</approach>.
		"bodies/big.md": &fstest.MapFile{Data: []byte(strings.Repeat("a", 9000))},
		// 300 chars, comfortably under the threshold.
		"bodies/small.md": &fstest.MapFile{Data: []byte(strings.Repeat("a", 300))},
	}
}

func TestCheck_OversizedPrompt(t *testing.T) {
	reg, err := registry.LoadFS(rule6FS())
	if err != nil {
		t.Fatalf("LoadFS() error = %v", err)
	}

	t.Run("inline target over threshold fires", func(t *testing.T) {
		in := prompt.Inputs{Target: "inline-target", Skills: []string{"big"}, Goal: "a sufficiently long goal"}
		built, err := prompt.Build(reg, in)
		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}
		wantLen := len(built)
		if wantLen <= 8000 {
			t.Fatalf("fixture built prompt is %d chars, want > 8000", wantLen)
		}

		findings := promptlint.Check(reg, in)
		f, ok := findRule(findings, promptlint.RuleOversizedPrompt)
		if !ok {
			t.Fatal("expected oversized-prompt finding")
		}
		wantMsg := fmt.Sprintf(
			"The prompt is %d characters because target %q inlines every skill body; consider selecting fewer skills, or a target that references skills instead.",
			wantLen, "inline-target",
		)
		if f.Message != wantMsg {
			t.Errorf("Message = %q, want %q", f.Message, wantMsg)
		}
	})

	t.Run("inline target under threshold is silent", func(t *testing.T) {
		in := prompt.Inputs{Target: "inline-target", Skills: []string{"small"}, Goal: "a sufficiently long goal"}
		findings := promptlint.Check(reg, in)
		if _, ok := findRule(findings, promptlint.RuleOversizedPrompt); ok {
			t.Error("expected no oversized-prompt finding")
		}
	})

	t.Run("reference-mode target with the same oversized skill is silent", func(t *testing.T) {
		in := prompt.Inputs{Target: "ref-target", Skills: []string{"big"}, Goal: "a sufficiently long goal"}
		findings := promptlint.Check(reg, in)
		if _, ok := findRule(findings, promptlint.RuleOversizedPrompt); ok {
			t.Error("expected no oversized-prompt finding on a reference-mode target")
		}
	})

	t.Run("unknown target is silent, other rules still report", func(t *testing.T) {
		in := prompt.Inputs{Target: "does-not-exist", Skills: []string{"big"}}
		findings := promptlint.Check(reg, in)
		if _, ok := findRule(findings, promptlint.RuleOversizedPrompt); ok {
			t.Error("expected no oversized-prompt finding for an unknown target")
		}
		// The other rules are pure functions of Inputs and don't
		// depend on the target resolving at all.
		if _, ok := findRule(findings, promptlint.RuleNoRole); !ok {
			t.Error("expected no-role finding to still report")
		}
		if _, ok := findRule(findings, promptlint.RuleNoOutputFormat); !ok {
			t.Error("expected no-output-format finding to still report")
		}
		if _, ok := findRule(findings, promptlint.RuleNoExamples); !ok {
			t.Error("expected no-examples finding to still report")
		}
	})
}

// TestCheck_OrderingContract pins Check's fixed rule-order contract: a
// single Inputs that trips all six rules must return them in the
// order constraints, role, output_format, examples, goal, oversized.
func TestCheck_OrderingContract(t *testing.T) {
	reg, err := registry.LoadFS(rule6FS())
	if err != nil {
		t.Fatalf("LoadFS() error = %v", err)
	}

	in := prompt.Inputs{
		Target:      "inline-target",
		Skills:      []string{"big"},
		Goal:        "fix bug", // 7 chars: fires short-goal
		Role:        "",
		Constraints: "Don't do X. Never do Y.",
		// OutputFormat and Examples left empty/nil.
	}

	findings := promptlint.Check(reg, in)

	wantOrder := []promptlint.RuleID{
		promptlint.RuleNegativeConstraints,
		promptlint.RuleNoRole,
		promptlint.RuleNoOutputFormat,
		promptlint.RuleNoExamples,
		promptlint.RuleShortGoal,
		promptlint.RuleOversizedPrompt,
	}
	if len(findings) != len(wantOrder) {
		t.Fatalf("len(findings) = %d, want %d; findings = %+v", len(findings), len(wantOrder), findings)
	}
	for i, want := range wantOrder {
		if findings[i].Rule != want {
			t.Errorf("findings[%d].Rule = %q, want %q", i, findings[i].Rule, want)
		}
	}
}

// TestRuleIDs_DistinctAndNonEmpty guards against a future edit
// accidentally colliding two RuleID values or leaving one blank -
// either would silently break identity-based lookups (findRule here,
// the web UI's data-rule selector, a future --no-hints=<rule> parse).
func TestRuleIDs_DistinctAndNonEmpty(t *testing.T) {
	ids := []promptlint.RuleID{
		promptlint.RuleNegativeConstraints,
		promptlint.RuleNoRole,
		promptlint.RuleNoOutputFormat,
		promptlint.RuleNoExamples,
		promptlint.RuleFewExamples,
		promptlint.RuleShortGoal,
		promptlint.RuleOversizedPrompt,
	}
	seen := make(map[promptlint.RuleID]bool, len(ids))
	for _, id := range ids {
		if id == "" {
			t.Error("found an empty RuleID")
		}
		if seen[id] {
			t.Errorf("RuleID %q is duplicated", id)
		}
		seen[id] = true
	}
}
