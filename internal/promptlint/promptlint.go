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

// Package promptlint reports advisory prompt-quality findings for a
// prompt.Inputs value. Check is a pure function of its arguments - no
// LLM call, no I/O - the same "no LLM runs here" contract prompt.Build
// itself carries. Findings are advisory only: they never affect an
// exit code and never block generation. A caller is free to display
// them, log them, or ignore them entirely.
//
// The package name mirrors internal/prompthl: one small,
// single-purpose package per concern, named for what it does (lint)
// rather than what it touches.
package promptlint

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/carlogy/prompt-smith/internal/prompt"
	"github.com/carlogy/prompt-smith/internal/registry"
)

// RuleID identifies which lint rule produced a Finding.
//
// This is a string, not an int enum like prompthl.Kind. That
// divergence is deliberate: Kind is a closed, presentational
// classification that's never shown to a user, whereas RuleID is
// user-facing identity - it renders directly into the web UI's
// data-rule attribute (giving end-to-end tests a stable selector),
// reads legibly in test failure output, and would let a future
// --no-hints=<rule> flag parse it directly rather than needing a
// separate name<->int lookup table.
type RuleID string

const (
	RuleNegativeConstraints RuleID = "negative-constraints"
	RuleNoRole              RuleID = "no-role"
	RuleNoOutputFormat      RuleID = "no-output-format"
	// RuleNoExamples and RuleFewExamples are the two mutually-exclusive
	// outputs of one rule func (checkExamples) sharing one doc
	// citation: there are 7 RuleIDs here but only 6 rules. They stay
	// separate IDs, rather than being folded into one, because the CLI
	// renderer groups pure-absence findings ("no X given") into a
	// single line and needs to tell "zero examples" (groupable) apart
	// from "1-2 examples" (its own sentence) by ID identity alone - the
	// alternative, recounting examples in the CLI to reconstruct which
	// case fired, would duplicate this rule's logic in a second place
	// where it could drift.
	RuleNoExamples      RuleID = "no-examples"
	RuleFewExamples     RuleID = "few-examples"
	RuleShortGoal       RuleID = "short-goal"
	RuleOversizedPrompt RuleID = "oversized-prompt"
)

// Finding is one advisory lint result.
//
// It deliberately carries no severity field: every finding is one
// tier, advisory, full stop. A field with exactly one possible value
// would only invite the three rendering surfaces (CLI, TUI, web) to
// branch on something that never varies. If a second tier is ever
// needed, adding the field then is a one-line change with every call
// site already in-repo.
type Finding struct {
	Rule    RuleID
	Message string
}

// Check runs all six lint rules against in - and, for the
// oversized-prompt rule alone, against reg (see checkOversizedPrompt)
// - and returns every finding that fired.
//
// Findings are returned in fixed rule order, 1 through 6: negative
// constraints, no role, no output_format, examples (no/few), short
// goal, then oversized prompt. This order is a contract, not an
// implementation detail: the CLI, TUI, and web renderers all rely on
// it to render deterministically, and are tested against it.
//
// reg is used by the oversized-prompt rule alone; rules 1-5 are pure
// functions of in and need no registry at all, which is what keeps
// them independently testable.
func Check(reg *registry.Registry, in prompt.Inputs) []Finding {
	var findings []Finding

	if f, ok := checkNegativeConstraints(in); ok {
		findings = append(findings, f)
	}
	if f, ok := checkNoRole(in); ok {
		findings = append(findings, f)
	}
	if f, ok := checkNoOutputFormat(in); ok {
		findings = append(findings, f)
	}
	if f, ok := checkExamples(in); ok {
		findings = append(findings, f)
	}
	if f, ok := checkShortGoal(in); ok {
		findings = append(findings, f)
	}
	if f, ok := checkOversizedPrompt(reg, in); ok {
		findings = append(findings, f)
	}

	return findings
}

// bulletLeadRe matches a leading bullet marker: "-", "*", "•", or a
// numeric ordinal like "1." or "2.", plus any following whitespace.
var bulletLeadRe = regexp.MustCompile(`^(?:[-*\x{2022}]|\d+\.)\s*`)

// negativeConstraintMarkers is the small, closed list of clause-initial
// markers checkNegativeConstraints treats as a negatively-phrased
// constraint, compared case-folded. See checkNegativeConstraints for
// why this must match at the START of a clause and nowhere else.
var negativeConstraintMarkers = []string{
	"don't", "dont", "do not", "never", "avoid", "no ", "not ",
	"without", "must not", "mustn't", "should not", "shouldn't",
	"cannot", "can't", "won't", "neither",
}

// checkNegativeConstraints implements the negative-constraints rule.
// Anthropic's "be clear and direct" guidance - echoed elsewhere in
// prompt-engineering literature - is to tell the model what to do,
// not what not to do. A constraints block written entirely as
// prohibitions is the anti-pattern this rule measures.
//
// It operates on in.Constraints alone, nothing else, split into
// clauses on newlines, '.', and ';' (see splitClauses - a '.'
// directly after a digit does not split, so a "1."/"2." ordinal
// bullet survives intact as a clause-leading token instead of being
// cut in half at its own period); each clause is trimmed and empty
// clauses are dropped. Before a clause is tested, a leading bullet
// marker ("-", "*", "•", or a numeric ordinal like "1.") is stripped,
// then a leading conjunction ("and"/"but") is stripped - so a
// bulleted list of prohibitions, or a "Don't X; and don't Y" clause,
// still reads as a prohibition rather than being masked by its own
// punctuation.
//
// A clause counts as negative only if, after that stripping, it
// BEGINS with a marker from negativeConstraintMarkers, compared
// case-folded. The rule fires only when there are two or more clauses
// AND every one of them is negative. Two traps this design
// specifically exists to avoid:
//
//   - Matching a negation keyword ANYWHERE in a clause would flag
//     "add no new dependencies" - a positively-framed clause that
//     merely contains the word "no". Clause-initial matching is what
//     prevents that; this exact string is this repo's own
//     internal/prompt/testdata/all_optional_fields_present.golden
//     constraints value.
//   - Firing on a single negative clause would flag "Don't break the
//     build.", a perfectly legitimate constraint on its own. The >= 2
//     gate, combined with requiring every clause be negative, is what
//     makes the rule measure the documented anti-pattern - a
//     constraints block written ENTIRELY as prohibitions - rather
//     than any individual, reasonable prohibition.
//
// One deliberate under-detection follows from the same design:
// "don't do x and don't do y" written on a single unpunctuated line
// reads as one clause and stays silent. Quiet-by-default is the
// intended bias here; splitting on "and"/"or" too would start
// guessing at clause boundaries English doesn't reliably mark.
func checkNegativeConstraints(in prompt.Inputs) (Finding, bool) {
	clauses := splitClauses(in.Constraints)
	if len(clauses) < 2 {
		return Finding{}, false
	}
	for _, c := range clauses {
		if !isNegativeClause(c) {
			return Finding{}, false
		}
	}
	return Finding{
		Rule:    RuleNegativeConstraints,
		Message: "Every constraint is phrased as a prohibition; stating what the model should do works better than what it shouldn't.",
	}, true
}

// splitClauses splits s into trimmed, non-empty clauses on newlines,
// '.', and ';'. A '.' immediately preceded by a digit does not split:
// that's what lets a "1." / "2." ordinal bullet survive intact as a
// clause-leading token for stripLeadingBullet to strip below, rather
// than being cut in half at its own period before it can be
// recognized as a bullet marker at all.
func splitClauses(s string) []string {
	runes := []rune(s)
	raw := make([]string, 0, 4)
	var cur strings.Builder
	for i, r := range runes {
		switch {
		case r == '\n' || r == ';':
			raw = append(raw, cur.String())
			cur.Reset()
		case r == '.' && !(i > 0 && unicode.IsDigit(runes[i-1])):
			raw = append(raw, cur.String())
			cur.Reset()
		default:
			cur.WriteRune(r)
		}
	}
	raw = append(raw, cur.String())

	out := make([]string, 0, len(raw))
	for _, c := range raw {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		out = append(out, c)
	}
	return out
}

// isNegativeClause reports whether clause, after stripping a leading
// bullet marker and then a leading conjunction, begins (case-folded)
// with one of negativeConstraintMarkers.
func isNegativeClause(clause string) bool {
	clause = stripLeadingConjunction(stripLeadingBullet(clause))
	lower := strings.ToLower(clause)
	for _, marker := range negativeConstraintMarkers {
		if strings.HasPrefix(lower, marker) {
			return true
		}
	}
	return false
}

// stripLeadingBullet removes a single leading bullet marker ("-",
// "*", "•", or a numeric ordinal like "1.") and any whitespace after
// it, so a bulleted or numbered list of prohibitions reads the same
// as an unadorned one.
func stripLeadingBullet(clause string) string {
	return strings.TrimSpace(bulletLeadRe.ReplaceAllString(clause, ""))
}

// stripLeadingConjunction removes a single leading "and" or "but"
// (case-folded, whole word only - "android" must not match "and")
// and any whitespace after it.
func stripLeadingConjunction(clause string) string {
	lower := strings.ToLower(clause)
	for _, conj := range []string{"and", "but"} {
		if !strings.HasPrefix(lower, conj) {
			continue
		}
		rest := clause[len(conj):]
		if rest == "" || rest[0] == ' ' || rest[0] == '\t' {
			return strings.TrimSpace(rest)
		}
	}
	return clause
}

// checkNoRole implements the no-role rule: a role focuses the model
// on the kind of expertise wanted, per the common "assign a persona"
// prompt-engineering guidance.
func checkNoRole(in prompt.Inputs) (Finding, bool) {
	if strings.TrimSpace(in.Role) != "" {
		return Finding{}, false
	}
	return Finding{
		Rule:    RuleNoRole,
		Message: "No role given; a role focuses the model on the kind of expertise you want.",
	}, true
}

// checkNoOutputFormat implements the no-output_format rule: stating
// the shape of the wanted response - a diff, JSON, bullets - makes
// the output easier to use without a second round-trip, per the
// "specify output format" prompt-engineering guidance.
func checkNoOutputFormat(in prompt.Inputs) (Finding, bool) {
	if strings.TrimSpace(in.OutputFormat) != "" {
		return Finding{}, false
	}
	return Finding{
		Rule:    RuleNoOutputFormat,
		Message: "No output_format given; describing the shape you want makes the response easier to use.",
	}, true
}

// checkExamples implements the examples rule: the one rule producing
// two mutually-exclusive RuleIDs (RuleNoExamples, RuleFewExamples)
// from a single citation. Anthropic's multishot-prompting guidance -
// echoed by this repo's own fielddesc.Examples sentence and README
// flags table - is that roughly 3-5 worked examples measurably
// improves output; fewer than that keeps firing (at 1 and 2 examples)
// and 3 or more is silent.
//
// Examples are normalized via prompt.NormalizeExamples rather than
// hand-rolled trimming, so this rule counts exactly what prompt.Build
// would render, not a slightly different count arrived at by
// re-implementing the same trim/drop-empty logic here.
func checkExamples(in prompt.Inputs) (Finding, bool) {
	examples := prompt.NormalizeExamples(in.Examples)
	n := len(examples)
	switch {
	case n == 0:
		return Finding{
			Rule:    RuleNoExamples,
			Message: "No examples given; 3-5 worked examples measurably improve output.",
		}, true
	case n <= 2:
		word := "examples"
		if n == 1 {
			word = "example"
		}
		return Finding{
			Rule:    RuleFewExamples,
			Message: fmt.Sprintf("Only %d %s given; 3-5 works best.", n, word),
		}, true
	default:
		return Finding{}, false
	}
}

// minGoalChars is the short-goal threshold: a trimmed goal strictly
// shorter than this is measurably under-specified. Chosen against
// this repo's own README example goals: "fix the bug" (11 characters)
// fires; "fix the flaky checkout test" (27) does not. Character count
// only - no token estimation anywhere in this package; token
// estimation was explicitly declined for this project.
const minGoalChars = 15

// checkShortGoal implements the short-goal rule: a very short goal
// under-specifies the task, per the general prompt-engineering
// guidance to be specific about what's wanted. An empty goal is
// deliberately exempt - it's a hard error enforced elsewhere in the
// CLI, not a lint finding - so this rule only ever fires on a short
// but non-empty goal.
func checkShortGoal(in prompt.Inputs) (Finding, bool) {
	goal := strings.TrimSpace(in.Goal)
	if goal == "" || len(goal) >= minGoalChars {
		return Finding{}, false
	}
	return Finding{
		Rule:    RuleShortGoal,
		Message: fmt.Sprintf("The goal is only %d characters; a more specific goal produces better output.", len(goal)),
	}, true
}

// maxInlinePromptChars is the oversized-prompt threshold, grounded in
// this registry's own data: the 14 bundled skill bodies total
// roughly 11,081 characters (~792 characters average per skill), so
// 8000 fires only when roughly 10 of the 14 are inlined at once - an
// indiscriminate, "just include everything" selection - while staying
// silent on a focused 3-5 skill prompt (~4,000 characters) and still
// catching a prompt made oversized by large user-supplied context or
// examples instead.
const maxInlinePromptChars = 8000

// checkOversizedPrompt implements the oversized-prompt rule: it fires
// only when the selected target renders skills inline (SkillMode ==
// "inline") and the fully assembled prompt exceeds
// maxInlinePromptChars. Reference-mode targets are exempt: they
// render a short pointer per skill rather than its full body, so
// skill count barely moves their size.
//
// This is the one rule that needs reg. It looks the target up in
// reg.Targets and, if the target is absent (an unrecognized
// in.Target) or not inline, skips silently rather than reporting
// anything. reg is used here alone, which is what keeps rules 1-5
// pure functions of in and testable with no registry at all.
//
// The prompt is measured by actually calling prompt.Build and taking
// len() of the result, rather than estimating a length from in plus
// the selected skills' Body lengths. An estimate would have to
// re-derive Build's own section layout (tags, blank lines, the
// <examples> wrapper, ...) in a second place, and would drift
// silently the moment that layout changes - the <examples> section
// added in an earlier phase would have broken exactly such an
// estimate with no test failure to catch it. The cost of calling
// Build here is negligible: it's pure string concatenation over at
// most roughly 12 KB of input.
//
// If Build returns an error (e.g. an unresolvable skill id), this
// rule is skipped silently; the other five rules are unaffected and
// still return their findings.
func checkOversizedPrompt(reg *registry.Registry, in prompt.Inputs) (Finding, bool) {
	target, ok := reg.Targets[in.Target]
	if !ok || target.SkillMode != "inline" {
		return Finding{}, false
	}

	built, err := prompt.Build(reg, in)
	if err != nil {
		return Finding{}, false
	}

	n := len(built)
	if n <= maxInlinePromptChars {
		return Finding{}, false
	}
	return Finding{
		Rule: RuleOversizedPrompt,
		Message: fmt.Sprintf(
			"The prompt is %d characters because target %q inlines every skill body; consider selecting fewer skills, or a target that references skills instead.",
			n, in.Target,
		),
	}, true
}
