// Package registry defines the skill/target data model and loads it from
// the embedded registry data (see Load).
package registry

import "sort"

// Registry is the fully-loaded set of skills, categories, and target
// configurations that prompt.Build assembles from.
type Registry struct {
	// Categories is the canonical category order (e.g. "debugging" before
	// "testing"). Skills are rendered in this order before any per-skill
	// weight is considered.
	Categories []string
	Skills     []Skill
	// Targets is keyed by target id ("generic", "opencode", "claude-code").
	Targets map[string]TargetConfig
}

// Skill is one methodology entry in the registry.
type Skill struct {
	ID        string
	Name      string
	Category  string
	Order     int
	WhenToUse string
	// Body is the generic inline methodology text. Empty means the skill
	// is not supported on the "generic" (inline) target.
	Body string
	// Refs optionally overrides the reference name used when this skill is
	// rendered for a "reference" mode target, keyed by target id. A target
	// not present here falls back to ID (e.g. "verify" -> "verify-checks"
	// for claude-code).
	Refs map[string]string
}

// TargetConfig describes how prompts are rendered for one target harness.
type TargetConfig struct {
	ID string
	// Name is a human-friendly display label (e.g. "Claude Code",
	// "Generic / Chat") - see DisplayName. Optional; falls back to ID
	// when empty, which most targets rely on (their id already reads
	// fine as a label, e.g. "opencode").
	Name string
	// Delimiter is reserved for future non-XML rendering; "xml" today.
	Delimiter string
	// SkillMode is "inline" (render each skill's Body directly) or
	// "reference" (derive a short "load this skill" pointer).
	SkillMode string
	// Reasoning and Action are reserved for future per-target prompt
	// coaching deltas. Unrendered in the current builder.
	Reasoning bool
	Action    string
	// Tools maps a generic capability name (search/read/find) to this
	// target's real tool name, rendered in a <tools> section for
	// "reference" mode targets.
	Tools map[string]string
}

// DisplayName returns t.Name if set, else t.ID - the single place that
// decides how a target is labeled for a human (the web UI's <select>;
// potentially CLI/TUI output later), so that fallback lives in one
// spot rather than being reimplemented at each call site.
func (t TargetConfig) DisplayName() string {
	if t.Name != "" {
		return t.Name
	}
	return t.ID
}

// SkillByID returns the skill with the given id, if present.
func (r *Registry) SkillByID(id string) (Skill, bool) {
	for _, s := range r.Skills {
		if s.ID == id {
			return s, true
		}
	}
	return Skill{}, false
}

// CategoryIndex returns each category's position in the canonical order,
// for sorting skills by category.
func (r *Registry) CategoryIndex() map[string]int {
	idx := make(map[string]int, len(r.Categories))
	for i, c := range r.Categories {
		idx[c] = i
	}
	return idx
}

// SortSkills sorts skills in place by canonical order: each skill's
// category position (per Categories), then its Order weight, then its id
// as a final, stable tiebreak. This is the single canonical ordering used
// both when rendering a selected subset (see prompt.Build) and when
// listing the full registry.
func (r *Registry) SortSkills(skills []Skill) {
	catIndex := r.CategoryIndex()
	sort.SliceStable(skills, func(i, j int) bool {
		a, b := skills[i], skills[j]
		if ai, bi := catIndex[a.Category], catIndex[b.Category]; ai != bi {
			return ai < bi
		}
		if a.Order != b.Order {
			return a.Order < b.Order
		}
		return a.ID < b.ID
	})
}

// SupportsTarget reports whether sk can render on the given target: the
// "inline" mode target requires a non-empty Body; "reference" mode
// targets are always supported (a reference snippet is derived from
// metadata regardless). An unknown target is never supported.
func (r *Registry) SupportsTarget(sk Skill, targetID string) bool {
	target, ok := r.Targets[targetID]
	if !ok {
		return false
	}
	if target.SkillMode == "reference" {
		return true
	}
	return sk.Body != ""
}
