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

package cli

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/carlogy/prompt-smith/internal/prompt"
)

// TestGenerate_BareGoalCollapsesAllThreeAbsenceFindings pins the exact
// sentence a first-time user sees on the single simplest command:
// `promptsmith "goal"`, with --quick so it never reaches for the
// picker. role, output_format, and examples are all missing, and this
// asserts they render as exactly one collapsed line, not three.
func TestGenerate_BareGoalCollapsesAllThreeAbsenceFindings(t *testing.T) {
	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"-t", "generic", "-s", "diagnose", "-q", "fix the flaky checkout test"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
	}

	want := `promptsmith: hint: no role, output_format, or examples given; adding them measurably improves output (see the "Hints" section of README.md)`
	if !strings.Contains(stderr.String(), want) {
		t.Errorf("stderr missing collapsed hint line %q, got:\n%s", want, stderr.String())
	}
	if n := strings.Count(stderr.String(), "promptsmith: hint:"); n != 1 {
		t.Errorf("expected exactly 1 hint line on a bare goal-only command, got %d in:\n%s", n, stderr.String())
	}
}

// TestGenerate_TwoMissingFieldsCollapsedPhrasing supplies role and
// output_format, leaving only examples missing, then supplies
// role+examples, leaving only output_format missing, to pin the
// one-missing and two-missing collapsed phrasings.
func TestGenerate_TwoMissingFieldsCollapsedPhrasing(t *testing.T) {
	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{
		"-t", "generic", "-s", "diagnose", "-q",
		"-r", "a senior engineer",
		"-f", "a unified diff",
		"fix the flaky checkout test",
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
	}

	want := `promptsmith: hint: no examples given; adding it measurably improves output (see the "Hints" section of README.md)`
	if !strings.Contains(stderr.String(), want) {
		t.Errorf("stderr missing one-missing hint line %q, got:\n%s", want, stderr.String())
	}
}

func TestGenerate_OneMissingFieldCollapsedPhrasing(t *testing.T) {
	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{
		"-t", "generic", "-s", "diagnose", "-q",
		"-r", "a senior engineer",
		"fix the flaky checkout test",
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
	}

	want := `promptsmith: hint: no output_format or examples given; adding them measurably improves output (see the "Hints" section of README.md)`
	if !strings.Contains(stderr.String(), want) {
		t.Errorf("stderr missing two-missing hint line %q, got:\n%s", want, stderr.String())
	}
}

// TestGenerate_NonCollapsibleFindingGetsItsOwnLineAlongsideCollapsed
// pins that a distinct-judgment finding (short-goal) prints its own
// line, in Check's documented order relative to the collapsed
// absence-findings line (short-goal comes after the three collapsed
// rules).
func TestGenerate_NonCollapsibleFindingGetsItsOwnLineAlongsideCollapsed(t *testing.T) {
	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	// "fix it" is under minGoalChars (15) and short-goal is exempt for
	// empty goals only, so this fires short-goal alongside all three
	// absence findings.
	root.SetArgs([]string{"-t", "generic", "-s", "diagnose", "-q", "fix it"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
	}

	collapsed := `promptsmith: hint: no role, output_format, or examples given; adding them measurably improves output (see the "Hints" section of README.md)`
	shortGoal := "promptsmith: hint: the goal is only 6 characters; a more specific goal produces better output."

	collapsedIdx := strings.Index(stderr.String(), collapsed)
	shortGoalIdx := strings.Index(stderr.String(), shortGoal)
	if collapsedIdx == -1 {
		t.Fatalf("stderr missing collapsed hint line, got:\n%s", stderr.String())
	}
	if shortGoalIdx == -1 {
		t.Fatalf("stderr missing short-goal hint line, got:\n%s", stderr.String())
	}
	if collapsedIdx >= shortGoalIdx {
		t.Errorf("expected the collapsed absence line before the short-goal line (Check's order), got:\n%s", stderr.String())
	}
}

// TestGenerate_NoHintsSuppressesAllHintsButKeepsNoSkillsNote pins that
// --no-hints suppresses every lint-derived hint line while leaving the
// prompt on stdout unchanged, and leaving the existing "no --skills
// given" note alone: that note is a runGenerate-owned fallback
// message, not a lint finding, and must keep printing regardless of
// --no-hints.
func TestGenerate_NoHintsSuppressesAllHintsButKeepsNoSkillsNote(t *testing.T) {
	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"-t", "generic", "-q", "--no-hints", "fix the flaky checkout test"}) // no -s

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
	}

	want := "<task>\nfix the flaky checkout test\n</task>"
	if !strings.Contains(stdout.String(), want) {
		t.Errorf("stdout missing %q, got:\n%s", want, stdout.String())
	}
	if strings.Contains(stderr.String(), "promptsmith: hint:") {
		t.Errorf("expected --no-hints to suppress all hint lines, got:\n%s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "--skills") {
		t.Errorf("expected the no-skills note to still print with --no-hints, got:\n%s", stderr.String())
	}
}

// TestGenerate_HintsNeverAppearOnStdout uses captureRealStdio, exactly
// like TestGenerate_DefaultDeliveryGoesToRealStdoutNotStderr in
// generate_test.go, since cobra's SetOut/SetErr can conflate the two
// streams and that is precisely the routing bug this repo has hit
// before (see captureRealStdio's own comment in cli_test.go).
func TestGenerate_HintsNeverAppearOnStdout(t *testing.T) {
	reg := testRegistry(t)
	root := newRootCmd(reg)
	root.SetArgs([]string{"-t", "generic", "-s", "diagnose", "-q", "fix the flaky checkout test"})

	var execErr error
	stdout, stderr := captureRealStdio(t, func() {
		execErr = root.Execute()
	})

	if execErr != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", execErr, stderr)
	}
	if !strings.Contains(stderr, "promptsmith: hint:") {
		t.Fatalf("expected a hint line on real stderr, got:\n%s", stderr)
	}
	if strings.Contains(stdout, "promptsmith: hint:") {
		t.Errorf("a hint line leaked onto real stdout, got:\n%s", stdout)
	}
}

// TestGenerate_WellFormedPromptProducesNoHints proves the linter stays
// silent on a well-formed prompt: 3+ examples, a role, an
// output_format, and a goal well over minGoalChars.
func TestGenerate_WellFormedPromptProducesNoHints(t *testing.T) {
	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{
		"-t", "generic", "-s", "diagnose", "-q",
		"-r", "a senior Go engineer",
		"-f", "a unified diff",
		"-e", "input: 1 -> output: one",
		"-e", "input: 2 -> output: two",
		"-e", "input: 3 -> output: three",
		"fix the flaky checkout test with a clear repro",
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
	}
	if strings.Contains(stderr.String(), "promptsmith: hint:") {
		t.Errorf("expected no hint output for a well-formed prompt, got:\n%s", stderr.String())
	}
}

// TestWarnLintFindings_JoinAndPluralization unit-tests
// warnLintFindings directly against a bytes.Buffer, independent of
// cobra, covering the join/pluralization logic for all three
// collapsed-count cases plus the noHints short-circuit.
func TestWarnLintFindings_JoinAndPluralization(t *testing.T) {
	reg := testRegistry(t)

	cases := []struct {
		name string
		in   prompt.Inputs
		want string
	}{
		{
			name: "one missing: examples",
			in: prompt.Inputs{
				Target:       "generic",
				Goal:         "fix the flaky checkout test",
				Role:         "a senior engineer",
				OutputFormat: "a unified diff",
			},
			want: `promptsmith: hint: no examples given; adding it measurably improves output (see the "Hints" section of README.md)` + "\n",
		},
		{
			name: "two missing: output_format, examples",
			in: prompt.Inputs{
				Target: "generic",
				Goal:   "fix the flaky checkout test",
				Role:   "a senior engineer",
			},
			want: `promptsmith: hint: no output_format or examples given; adding them measurably improves output (see the "Hints" section of README.md)` + "\n",
		},
		{
			name: "three missing: role, output_format, examples",
			in: prompt.Inputs{
				Target: "generic",
				Goal:   "fix the flaky checkout test",
			},
			want: `promptsmith: hint: no role, output_format, or examples given; adding them measurably improves output (see the "Hints" section of README.md)` + "\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			warnLintFindings(&buf, reg, tc.in, false)
			if buf.String() != tc.want {
				t.Errorf("warnLintFindings() =\n%q\nwant:\n%q", buf.String(), tc.want)
			}
		})
	}
}

// TestWarnLintFindings_NoHintsSuppressesOutput unit-tests the
// noHints short-circuit directly, independent of cobra.
func TestWarnLintFindings_NoHintsSuppressesOutput(t *testing.T) {
	reg := testRegistry(t)
	var buf bytes.Buffer
	warnLintFindings(&buf, reg, prompt.Inputs{Target: "generic", Goal: "fix the flaky checkout test"}, true)
	if buf.Len() != 0 {
		t.Errorf("warnLintFindings() with noHints=true wrote output, got:\n%s", buf.String())
	}
}

// TestWarnLintFindings_LowercasesNonCollapsibleFindingButPreservesRestVerbatim
// pins the Task 6 cosmetic fix: a non-collapsible finding's message
// (promptlint.Finding.Message is a capitalized standalone sentence,
// correct for the web UI) renders with its first rune lowercased once
// inlined after the CLI's "promptsmith: hint: " prefix, while every
// other character - including a quoted target id and a run of digits
// elsewhere in the same sentence - survives untouched. Triggers
// RuleOversizedPrompt (the one rule whose message carries both a
// quoted target id AND a digit run) by inlining every embedded skill
// for the "generic" target, comfortably over maxInlinePromptChars; see
// promptlint.checkOversizedPrompt's own doc comment for why that
// pushes past the 8000-character threshold.
func TestWarnLintFindings_LowercasesNonCollapsibleFindingButPreservesRestVerbatim(t *testing.T) {
	reg := testRegistry(t)
	var buf bytes.Buffer
	warnLintFindings(&buf, reg, prompt.Inputs{
		Target: "generic",
		Skills: []string{
			"architect", "caveman-commit", "caveman-review", "caveman",
			"codebase-course", "convention", "diagnose", "generalize-not-hardcode",
			"grill-me", "lean-code", "quote-grounding", "safe-actions", "tdd", "verify",
		},
		Goal:         "fix the flaky checkout test",
		Role:         "a senior engineer",
		OutputFormat: "a unified diff",
		Examples:     []string{"a", "b", "c"},
	}, false)

	got := buf.String()
	if strings.Contains(got, "promptsmith: hint: The prompt is") {
		t.Errorf("expected the finding's leading rune to be lowercased, got:\n%s", got)
	}
	want := regexp.MustCompile(`promptsmith: hint: the prompt is \d+ characters because target "generic" inlines every skill body`)
	if !want.MatchString(got) {
		t.Errorf("expected lowercased oversized-prompt hint with the quoted target id and digit count preserved verbatim, got:\n%s", got)
	}
}
