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
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/carlogy/prompt-smith/internal/prompt"
	"github.com/carlogy/prompt-smith/internal/registry"
)

// manySkillsRegistry returns a registry with n skills in a single
// category, all supported on "generic" - enough selectable items that
// a page (several rows) is a small fraction of the whole list, unlike
// fixtureRegistry's 2 selectable items. Used to exercise real paging
// (as opposed to the short-list clamp case, which fixtureRegistry
// already covers on its own).
func manySkillsRegistry(n int) *registry.Registry {
	skills := make([]registry.Skill, n)
	for i := range skills {
		skills[i] = registry.Skill{
			ID:       fmt.Sprintf("skill-%02d", i+1),
			Category: "cat",
			Order:    i,
			Body:     "body",
		}
	}
	return &registry.Registry{
		Categories: []string{"cat"},
		Skills:     skills,
		Targets: map[string]registry.TargetConfig{
			"generic": {ID: "generic", Delimiter: "xml", SkillMode: "inline"},
		},
	}
}

func TestSkills_PgDownPagesTheListWhenSkillsHasFocus(t *testing.T) {
	reg := manySkillsRegistry(20)
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g"})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	m2 := updated.(model) // focus defaults to focusSkills

	updated2, _ := m2.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	m3 := updated2.(model)

	if m3.items[m3.cursor].isHeader {
		t.Fatal("cursor landed on a header after PgDown")
	}

	// PgDown must move further than a single Down step - that's the
	// entire point of paging, not just re-testing Down under a
	// different key.
	updatedDown, _ := m2.Update(tea.KeyMsg{Type: tea.KeyDown})
	singleStep := updatedDown.(model)
	if m3.cursor <= singleStep.cursor {
		t.Errorf("PgDown cursor = %d, want further than a single Down step (%d)", m3.cursor, singleStep.cursor)
	}

	// PgDown while skills is focused must not touch the preview at all.
	if m3.previewVP.YOffset != 0 {
		t.Errorf("expected PgDown while skills is focused not to scroll the preview, got YOffset=%d", m3.previewVP.YOffset)
	}
}

func TestSkills_PgUpPagesTheListWhenSkillsHasFocus(t *testing.T) {
	reg := manySkillsRegistry(20)
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g"})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	m2 := updated.(model)

	// Move well into the list first so there's room to page back up.
	last := m2
	for i := 0; i < 30; i++ { // generously more than needed to reach the end
		u, _ := last.Update(tea.KeyMsg{Type: tea.KeyDown})
		last = u.(model)
	}

	updated2, _ := last.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	m3 := updated2.(model)

	if m3.items[m3.cursor].isHeader {
		t.Fatal("cursor landed on a header after PgUp")
	}

	updatedUp, _ := last.Update(tea.KeyMsg{Type: tea.KeyUp})
	singleStep := updatedUp.(model)
	if m3.cursor >= singleStep.cursor {
		t.Errorf("PgUp cursor = %d, want further back than a single Up step (%d)", m3.cursor, singleStep.cursor)
	}
}

func TestSkills_PgDownClampsAtLastSelectableItem(t *testing.T) {
	reg := manySkillsRegistry(20)
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g"})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	m2 := updated.(model)

	cur := m2
	for i := 0; i < 10; i++ { // generously more pages than needed to reach the end
		u, _ := cur.Update(tea.KeyMsg{Type: tea.KeyPgDown})
		cur = u.(model)
	}

	// Must land on the same item repeated single Down presses would -
	// the existing, already-correct boundary convention (see
	// TestModel_CursorClampsAtBoundaries).
	wantLast := m2
	for i := 0; i < 30; i++ {
		u, _ := wantLast.Update(tea.KeyMsg{Type: tea.KeyDown})
		wantLast = u.(model)
	}
	if cur.cursor != wantLast.cursor {
		t.Errorf("PgDown clamp landed on cursor %d, want %d (same as repeated Down)", cur.cursor, wantLast.cursor)
	}

	// No-op at the boundary: one more PgDown must not move further or wrap.
	u2, _ := cur.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	cur2 := u2.(model)
	if cur2.cursor != cur.cursor {
		t.Errorf("PgDown moved past the last item: was %d, now %d", cur.cursor, cur2.cursor)
	}
}

func TestSkills_PgUpClampsAtFirstSelectableItem(t *testing.T) {
	reg := manySkillsRegistry(20)
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g"})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	m2 := updated.(model) // cursor starts on the first selectable item

	cur := m2
	for i := 0; i < 10; i++ { // generously more pages than needed
		u, _ := cur.Update(tea.KeyMsg{Type: tea.KeyPgUp})
		cur = u.(model)
	}
	if cur.cursor != m2.cursor {
		t.Errorf("PgUp from the top moved the cursor: was %d, now %d", m2.cursor, cur.cursor)
	}
}

func TestSkills_ShortListClampsToFirstAndLastSelectable(t *testing.T) {
	// fixtureRegistry's generic items are only 2 selectable (diagnose,
	// verify) - far shorter than any real page size, exercising the
	// "list shorter than one page" case (requirement 4).
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g"})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	m2 := updated.(model)

	down, _ := m2.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	last := down.(model)
	if last.items[last.cursor].isHeader {
		t.Fatal("PgDown landed on a header")
	}
	if got := last.items[last.cursor].skill.ID; got != "verify" {
		t.Errorf("PgDown on a short list landed on %q, want %q (the last selectable item)", got, "verify")
	}

	up, _ := last.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	first := up.(model)
	if got := first.items[first.cursor].skill.ID; got != "diagnose" {
		t.Errorf("PgUp on a short list landed on %q, want %q (the first selectable item)", got, "diagnose")
	}
}

func TestPreview_PgUpPagesPreviewWhenPreviewHasFocus(t *testing.T) {
	// PgDown's mirror image is already covered by
	// TestPreview_PageDownScrollsWhenContentOverflows; this confirms
	// PgUp still pages the preview - unchanged behavior - once preview,
	// rather than the skill list, has focus.
	reg := longBodyRegistry("line1", "line2", "line3", "line4", "line5", "line6", "line7", "line8")
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g", Skills: []string{"longskill"}})

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 8})
	m2 := updated.(model)
	focused, _ := m2.changeFocus(focusPreview)
	m2 = focused.(model)

	// Page to the bottom first so there's room to page back up.
	cur := m2
	for i := 0; i < 10 && !cur.previewVP.AtBottom(); i++ { // generously more than needed
		u, _ := cur.Update(tea.KeyMsg{Type: tea.KeyPgDown})
		cur = u.(model)
	}
	if cur.previewVP.YOffset == 0 {
		t.Fatal("expected PageDown to move the scroll offset (test setup issue)")
	}

	updated2, _ := cur.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	m3 := updated2.(model)
	if m3.previewVP.YOffset >= cur.previewVP.YOffset {
		t.Errorf("expected PgUp to scroll the preview back up, before=%d after=%d", cur.previewVP.YOffset, m3.previewVP.YOffset)
	}

	// PgUp while preview is focused must not touch the skill cursor.
	if m3.cursor != cur.cursor {
		t.Errorf("expected PgUp while preview is focused not to move the skill cursor, was %d now %d", cur.cursor, m3.cursor)
	}
}
