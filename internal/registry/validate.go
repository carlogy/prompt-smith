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
