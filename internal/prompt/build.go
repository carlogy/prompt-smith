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

// Package prompt assembles a complete, deterministic prompt from a
// registry and user inputs. No LLM runs here: Build is a pure function of
// its arguments.
package prompt

import (
	"fmt"
	"sort"
	"strings"

	"github.com/carlogy/prompt-smith/internal/registry"
)

// Inputs are the user-supplied values used to assemble a prompt. Only
// Target and Goal are required; the rest are optional and simply omitted
// from the output when empty.
type Inputs struct {
	Target       string
	Skills       []string
	Goal         string
	Role         string
	Context      string
	Constraints  string
	OutputFormat string
	Examples     []string
}

// Build assembles a complete prompt from the registry and the given
// inputs.
func Build(reg *registry.Registry, in Inputs) (string, error) {
	target, ok := reg.Targets[in.Target]
	if !ok {
		return "", fmt.Errorf("prompt: unknown target %q", in.Target)
	}

	approach, err := buildApproach(reg, target, in.Skills)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	section(&b, "role", in.Role)
	section(&b, "task", in.Goal)
	section(&b, "context", in.Context)
	section(&b, "approach", approach)
	section(&b, "tools", buildTools(target))
	section(&b, "constraints", in.Constraints)
	section(&b, "output_format", in.OutputFormat)
	// <examples> is last on purpose. Anthropic's long-context guidance
	// orders prompt content as longform data -> query -> instructions ->
	// examples, and independent of that, examples are most legible sitting
	// directly beneath the <output_format> they usually demonstrate rather
	// than ahead of it.
	examplesSection(&b, in.Examples)

	return strings.TrimRight(b.String(), "\n"), nil
}

// buildApproach resolves the selected skills (deduped, then sorted by
// canonical category order, then per-skill weight, then id) and renders
// each one for the given target: inlined verbatim for "inline" targets,
// or as a derived reference snippet for "reference" targets.
func buildApproach(reg *registry.Registry, target registry.TargetConfig, ids []string) (string, error) {
	skills, err := resolveSkills(reg, ids)
	if err != nil {
		return "", err
	}

	parts := make([]string, 0, len(skills))
	for _, sk := range skills {
		body, err := renderSkill(sk, target)
		if err != nil {
			return "", err
		}
		parts = append(parts, body)
	}
	return strings.Join(parts, "\n\n"), nil
}

// renderSkill renders one skill for the given target.
func renderSkill(sk registry.Skill, target registry.TargetConfig) (string, error) {
	if target.SkillMode == "reference" {
		return deriveReference(sk, target), nil
	}
	if sk.Body == "" {
		return "", fmt.Errorf("prompt: skill %q has no generic body (unsupported on target %q)", sk.ID, target.ID)
	}
	return strings.TrimSpace(sk.Body), nil
}

// deriveReference builds a short "load this skill" pointer for
// reference-mode targets, using the skill's per-target ref override (e.g.
// "verify" -> "verify-checks" for claude-code) when present, falling back
// to the skill id.
func deriveReference(sk registry.Skill, target registry.TargetConfig) string {
	ref := sk.ID
	if r, ok := sk.Refs[target.ID]; ok && r != "" {
		ref = r
	}
	if sk.WhenToUse == "" {
		return fmt.Sprintf("Load the `%s` skill.", ref)
	}
	return fmt.Sprintf("Load the `%s` skill: %s", ref, sk.WhenToUse)
}

// buildTools renders a target's tool-name mapping (search/read/find ->
// the real tool name for that harness) as deterministic, sorted lines.
// Targets with no tools (e.g. generic) render nothing.
func buildTools(target registry.TargetConfig) string {
	if len(target.Tools) == 0 {
		return ""
	}
	keys := make([]string, 0, len(target.Tools))
	for k := range target.Tools {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	lines := make([]string, 0, len(keys))
	for _, k := range keys {
		lines = append(lines, fmt.Sprintf("%s: %s", k, target.Tools[k]))
	}
	return strings.Join(lines, "\n")
}

// trimAndDropEmpty trims surrounding whitespace from each element and
// drops any that are empty after trimming, preserving input order and
// never deduping. It backs both NormalizeSkills and NormalizeExamples:
// the two normalize different fields with identical semantics, so this
// holds the one implementation while each keeps its own exported name
// and doc comment explaining its own callers - two names that can't
// drift apart because there's only one body.
func trimAndDropEmpty(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		out = append(out, s)
	}
	return out
}

// NormalizeSkills trims surrounding whitespace from each id and drops
// any that are empty after trimming, preserving input order. It exists
// because pflag's StringSlice flag (used for --skills) CSV-splits its
// argument, so "-s a, b" and "-s a," yield [" b"] and [""] respectively
// as literal elements - both of which would otherwise hard-error as
// unknown skills. It does not dedupe (see resolveSkills) and does not
// case-fold: skill matching stays case-sensitive.
func NormalizeSkills(ids []string) []string {
	return trimAndDropEmpty(ids)
}

// NormalizeExamples trims surrounding whitespace from each example and
// drops any that are empty after trimming, preserving input order and
// not deduping - two identical examples are a legitimate (if redundant)
// input, not something to silently collapse. It's called both by
// examplesSection below (for Examples set directly, e.g. via the CLI's
// repeated -e/--example flag) and by SplitExamples (for the TUI/web
// single multi-line field, after it's divided into pieces), so both
// paths into Inputs.Examples end up normalized the same way.
func NormalizeExamples(examples []string) []string {
	return trimAndDropEmpty(examples)
}

// resolveSkills normalizes ids (trimming whitespace and dropping empty
// entries), looks up each remaining id (deduping repeats, preserving
// first occurrence), then sorts the result via the registry's canonical
// ordering (category position, then weight, then id).
func resolveSkills(reg *registry.Registry, ids []string) ([]registry.Skill, error) {
	ids = NormalizeSkills(ids)
	seen := make(map[string]bool, len(ids))
	skills := make([]registry.Skill, 0, len(ids))
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true

		sk, ok := reg.SkillByID(id)
		if !ok {
			return nil, fmt.Errorf("prompt: unknown skill %q", id)
		}
		skills = append(skills, sk)
	}

	reg.SortSkills(skills)
	return skills, nil
}

// section appends an XML-delimited block, separated from any prior
// section by a single blank line. Empty bodies are omitted entirely.
func section(b *strings.Builder, tag, body string) {
	if body == "" {
		return
	}
	if b.Len() > 0 {
		b.WriteString("\n")
	}
	fmt.Fprintf(b, "<%s>\n%s\n</%s>\n", tag, strings.TrimSpace(body), tag)
}

// examplesSection appends the <examples> section: a wrapper tag around
// one <example> child per entry in examples (after NormalizeExamples),
// with a single blank line between consecutive children. It's omitted
// entirely when examples is empty after normalization, mirroring
// section()'s empty-body handling. This can't be built with section()
// itself: section() only knows how to wrap one flat, already-plain-text
// body in a single tag, and has no notion of a tag that contains other
// tags, so <examples>/<example> nesting needs its own builder.
//
// Every tag here - <examples>, </examples>, <example>, </example> -
// sits alone on its own line. That's a hard contract, not a formatting
// preference: internal/prompthl.Classify matches lines against
// ^<[a-z_]+>$ / ^</[a-z_]+>$ to decide what to syntax-highlight in the
// TUI and web previews, and <example> already satisfies that pattern,
// so nested highlighting comes for free - but only for as long as no
// tag ever shares a line with anything else.
func examplesSection(b *strings.Builder, examples []string) {
	examples = NormalizeExamples(examples)
	if len(examples) == 0 {
		return
	}

	children := make([]string, 0, len(examples))
	for _, ex := range examples {
		children = append(children, fmt.Sprintf("<example>\n%s\n</example>", ex))
	}

	if b.Len() > 0 {
		b.WriteString("\n")
	}
	fmt.Fprintf(b, "<examples>\n%s\n</examples>\n", strings.Join(children, "\n\n"))
}

// SplitExamples divides one multi-line field into individual examples by
// splitting on any line whose content is exactly "---" after trimming
// surrounding whitespace, then runs the pieces through NormalizeExamples
// (trimming each and dropping empties). It exists because the TUI and
// web UI each expose Examples as a single free-form textarea rather than
// the CLI's repeated -e/--example flag, so both need one shared way to
// turn that one field into the []string prompt.Inputs.Examples expects.
//
// "---" was chosen over, say, a blank-line separator for two reasons:
// it matches the "---"-delimited frontmatter convention SKILL.md
// already uses elsewhere in this repo (see
// internal/registry/userskills.go's parseSkillMD), and unlike a
// blank-line separator, it survives examples that themselves contain
// internal blank lines - which multi-line input/output example pairs
// routinely do.
func SplitExamples(s string) []string {
	// Normalize CRLF and lone CR to LF first. This matters: HTML
	// <textarea> submissions arrive CRLF-encoded, so without this both
	// the separator-line detection below and the example bodies
	// themselves would silently corrupt on any browser-submitted input.
	// Mirrors parseSkillMD's normalization of the same problem.
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")

	lines := strings.Split(s, "\n")
	groups := make([]string, 0, 1)
	var current []string
	for _, line := range lines {
		if strings.TrimSpace(line) == "---" {
			groups = append(groups, strings.Join(current, "\n"))
			current = nil
			continue
		}
		current = append(current, line)
	}
	groups = append(groups, strings.Join(current, "\n"))

	return NormalizeExamples(groups)
}

// JoinExamples is SplitExamples's inverse: it runs examples through
// NormalizeExamples, then joins them back into one multi-line string
// with the same "\n---\n" separator line SplitExamples divides on.
// Nil/empty input (or input that normalizes away to nothing) returns
// "" rather than a lone "---", so an unseeded Examples field renders
// as an empty textarea, not stray separator punctuation.
//
// It exists for the opposite direction from SplitExamples: seeding the
// TUI/web UI's single Examples textarea from an already-assembled
// []string. Today that's the web UI (server/page.go's initialData,
// seeded from app.initial.Examples - itself sourced from --ui's
// flags/args); the TUI's own textarea field will need the same seeding
// once it lands. Round-trip contract both callers rely on:
// SplitExamples(JoinExamples(x)) == NormalizeExamples(x) for any x -
// so the value a user sees on page load is exactly what rebuilds back
// out of the textarea on the very next submit, with no drift from
// whitespace or ordering.
func JoinExamples(examples []string) string {
	examples = NormalizeExamples(examples)
	if len(examples) == 0 {
		return ""
	}
	return strings.Join(examples, "\n---\n")
}
