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

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/carlogy/prompt-smith/internal/prompt"
)

// TestFocus_RightArrowOnTargetAdvancesAndRefiltersItems drives a fresh
// model to focusTarget (one Shift+Tab from the default focusSkills,
// since focusTarget sits immediately before focusSkills in the cycle),
// then presses Right and checks two things at once: m.target actually
// advanced to the next id in sortedTargetIDs order, and the item set
// was re-evaluated against the new target's SupportsTarget rules -
// "generic" (SkillMode: inline) disables the Body-less "agent-only"
// skill (greyed out, not hidden), but "opencode" (SkillMode:
// reference) supports every skill regardless of Body, so switching
// from "generic" to "opencode" must re-enable "agent-only".
func TestFocus_RightArrowOnTargetAdvancesAndRefiltersItems(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g"})

	if !hasItem(m.items, "agent-only") {
		t.Fatal("expected agent-only to still be present (greyed out, not hidden) on the initial target (generic)")
	}
	if !itemByID(m.items, "agent-only").disabled {
		t.Fatal("expected agent-only to be disabled on the initial target (generic)")
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab}) // skills -> target (wraps back)
	cur := updated.(model)
	if cur.focus != focusTarget {
		t.Fatalf("focus after one Shift+Tab = %v, want focusTarget", cur.focus)
	}

	updated2, _ := cur.Update(tea.KeyMsg{Type: tea.KeyRight})
	cur2 := updated2.(model)

	if cur2.target != "opencode" {
		t.Errorf("target after Right = %q, want %q", cur2.target, "opencode")
	}
	if !hasItem(cur2.items, "agent-only") {
		t.Error("expected agent-only to still be present once the target switched to opencode")
	}
	if itemByID(cur2.items, "agent-only").disabled {
		t.Error("expected agent-only to become enabled once the target switched to opencode (reference mode supports every skill)")
	}
	if cur2.items[cur2.cursor].isHeader {
		t.Error("expected the cursor to land on a selectable item after the target change")
	}
}

// TestFocus_TargetSwitchAutoUnchecksNowUnsupportedSkill is the mirror
// of TestFocus_RightArrowOnTargetAdvancesAndRefiltersItems: it starts
// on a target that supports a skill and has it selected, then switches
// to a target that doesn't - proving buildItems' force-unselect (see
// its doc comment) actually reaches a skill that WAS selected, not
// just one that never was, and that recomputePreview (called right
// after buildItems in updateTargetField) picks up the change so the
// now-unselected skill can't linger in the built prompt.
func TestFocus_TargetSwitchAutoUnchecksNowUnsupportedSkill(t *testing.T) {
	reg := fixtureRegistry()
	// opencode is reference-mode, so it supports agent-only (no Body)
	// despite generic not supporting it.
	m := newModel(reg, prompt.Inputs{Target: "opencode", Goal: "g", Skills: []string{"agent-only"}})

	if itemByID(m.items, "agent-only").disabled {
		t.Fatal("expected agent-only to be enabled on opencode")
	}
	if !itemByID(m.items, "agent-only").selected {
		t.Fatal("expected agent-only to start selected on opencode")
	}
	if !strings.Contains(m.preview, "agent-only") {
		t.Fatalf("expected the initial preview to reference agent-only, got:\n%s", m.preview)
	}

	// Shift+Tab to focusTarget (see the sibling test above for why one
	// Shift+Tab gets there), then Left cycles alphabetically backward:
	// opencode -> generic.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	cur := updated.(model)
	updated2, _ := cur.Update(tea.KeyMsg{Type: tea.KeyLeft})
	cur2 := updated2.(model)

	if cur2.target != "generic" {
		t.Fatalf("target after Left = %q, want %q", cur2.target, "generic")
	}
	if !itemByID(cur2.items, "agent-only").disabled {
		t.Error("expected agent-only to become disabled once the target switched to generic")
	}
	if itemByID(cur2.items, "agent-only").selected {
		t.Error("expected agent-only to be auto-unchecked once the target switched to generic")
	}
	if strings.Contains(cur2.preview, "agent-only") {
		t.Errorf("expected the recomputed preview to no longer reference agent-only, got:\n%s", cur2.preview)
	}
}

func hasItem(items []item, id string) bool {
	for _, it := range items {
		if !it.isHeader && it.skill.ID == id {
			return true
		}
	}
	return false
}

// itemByID returns the item with the given skill id, or the zero item
// if not found (callers that care check hasItem first).
func itemByID(items []item, id string) item {
	for _, it := range items {
		if !it.isHeader && it.skill.ID == id {
			return it
		}
	}
	return item{}
}
