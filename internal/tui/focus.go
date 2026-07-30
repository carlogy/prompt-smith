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

package tui

// focusZone is which region of the TUI currently receives key input:
// the skill picker, one of the editable fields, or the preview.
type focusZone int

const (
	focusSkills focusZone = iota
	focusGoal
	focusContext
	focusConstraints
	focusRole
	focusOutputFormat
	focusExamples
	focusPreview
	focusTarget
)

// focusCycle is the canonical Tab order. focusTarget sits immediately
// before focusSkills (i.e. right after focusPreview on the wrap) rather
// than at the front, so that the default starting zone (focusSkills,
// the zero value) is unaffected and every existing "N tabs from skills
// to <zone>" distance - notably "6 tabs to preview" (now 7, with
// focusExamples inserted ahead of it) - stays derivable from this one
// list rather than needing to change independently of it. focusExamples
// sits right after focusOutputFormat, matching where the Examples
// field renders visually (last of the six fields, directly above the
// preview pane it sits next to in Tab order) - following visual order
// is the same reasoning every other field's position in this list
// already relies on.
var focusCycle = []focusZone{
	focusTarget, focusSkills, focusGoal, focusContext, focusConstraints,
	focusRole, focusOutputFormat, focusExamples, focusPreview,
}

// nextFocus/prevFocus advance the cycle with wraparound. An unrecognized
// zone (shouldn't happen) falls back to focusSkills.
func nextFocus(f focusZone) focusZone {
	for i, z := range focusCycle {
		if z == f {
			return focusCycle[(i+1)%len(focusCycle)]
		}
	}
	return focusSkills
}

func prevFocus(f focusZone) focusZone {
	for i, z := range focusCycle {
		if z == f {
			return focusCycle[(i-1+len(focusCycle))%len(focusCycle)]
		}
	}
	return focusSkills
}
