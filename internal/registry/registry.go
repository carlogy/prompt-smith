// Package registry defines the skill/target data model and loads it from
// the embedded registry data (see Load).
package registry

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
