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

// resolveSkills looks up each id (deduping repeats, preserving first
// occurrence), then sorts the result via the registry's canonical
// ordering (category position, then weight, then id).
func resolveSkills(reg *registry.Registry, ids []string) ([]registry.Skill, error) {
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
