package registry

import "fmt"

// Validate checks semantic integrity beyond what LoadFS's parsing already
// guarantees: every skill's category must be declared, skill ids must be
// unique, and every ref target must be a known target. This is what the
// "validate" CLI command runs before a registry ships.
func (r *Registry) Validate() error {
	categories := make(map[string]bool, len(r.Categories))
	for _, c := range r.Categories {
		categories[c] = true
	}

	seen := make(map[string]bool, len(r.Skills))
	for _, sk := range r.Skills {
		if seen[sk.ID] {
			return fmt.Errorf("registry: duplicate skill id %q", sk.ID)
		}
		seen[sk.ID] = true

		if !categories[sk.Category] {
			return fmt.Errorf("registry: skill %q: unknown category %q", sk.ID, sk.Category)
		}

		for targetID := range sk.Refs {
			if _, ok := r.Targets[targetID]; !ok {
				return fmt.Errorf("registry: skill %q: ref for unknown target %q", sk.ID, targetID)
			}
		}
	}

	return nil
}
