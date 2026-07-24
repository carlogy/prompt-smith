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

package server

import "net/http"

// registryResponse is the JSON shape of GET /api/registry: everything
// the page needs to render the skill picker and target selector.
type registryResponse struct {
	Categories []string    `json:"categories"`
	Targets    []targetDTO `json:"targets"`
	Skills     []skillDTO  `json:"skills"`
}

type targetDTO struct {
	ID string `json:"id"`
}

// skillDTO mirrors registry.Skill's picker-relevant fields, plus
// SupportedTargets precomputed via Registry.SupportsTarget - the same
// check `list -t` uses - so the page doesn't need target-support logic
// duplicated in JavaScript.
type skillDTO struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Category         string   `json:"category"`
	WhenToUse        string   `json:"whenToUse"`
	SupportedTargets []string `json:"supportedTargets"`
}

// handleRegistry serves the loaded registry (embedded skills merged
// with any user skills - see registry.Load) as JSON, with skills in
// the same canonical order SortSkills defines for every other surface
// (list, the TUI picker).
func (app *application) handleRegistry(w http.ResponseWriter, r *http.Request) {
	targetIDs := sortedTargetIDs(app.reg)
	skills := sortedSkills(app.reg)

	skillDTOs := make([]skillDTO, 0, len(skills))
	for _, sk := range skills {
		supported := make([]string, 0, len(targetIDs))
		for _, tid := range targetIDs {
			if app.reg.SupportsTarget(sk, tid) {
				supported = append(supported, tid)
			}
		}
		skillDTOs = append(skillDTOs, skillDTO{
			ID:               sk.ID,
			Name:             sk.Name,
			Category:         sk.Category,
			WhenToUse:        sk.WhenToUse,
			SupportedTargets: supported,
		})
	}

	targetDTOs := make([]targetDTO, 0, len(targetIDs))
	for _, id := range targetIDs {
		targetDTOs = append(targetDTOs, targetDTO{ID: id})
	}

	resp := registryResponse{
		Categories: app.reg.Categories,
		Targets:    targetDTOs,
		Skills:     skillDTOs,
	}
	if err := writeJSON(w, http.StatusOK, resp); err != nil {
		app.serverError(w, r, err)
	}
}
