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

// At 80x24 the fixtureRegistry list (generic) fits with offset 0, so
// screen rows map directly: row listTopOffset+index. On generic the
// items are [header:debugging(0), diagnose(1), header:testing(2),
// verify(3), agent-only(4, disabled)], so verify sits at
// y = listTopOffset+3.
func leftClick(x, y int) tea.MouseMsg {
	return tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, X: x, Y: y}
}

func TestClick_OnSkillMovesCursorTogglesAndRecomputes(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g"})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m2 := updated.(model)

	// cursor starts on diagnose (index 1); click verify (index 3).
	updated2, _ := m2.Update(leftClick(3, listTopOffset+3))
	m3 := updated2.(model)

	if m3.cursor != 3 {
		t.Errorf("cursor = %d, want 3 (moved to the clicked verify row)", m3.cursor)
	}
	if !m3.items[3].selected {
		t.Error("expected verify to become selected on click")
	}
	if !strings.Contains(m3.preview, "verify body") {
		t.Errorf("expected preview to include verify's body after click, got:\n%s", m3.preview)
	}

	// clicking it again toggles it back off.
	updated3, _ := m3.Update(leftClick(3, listTopOffset+3))
	m4 := updated3.(model)
	if m4.items[3].selected {
		t.Error("expected a second click to deselect verify")
	}
}

func TestClick_OnDisabledRowIsANoOp(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g"})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m2 := updated.(model)

	before := m2.cursor
	// index 4 is agent-only, disabled on generic (unsupported: no Body)
	// - see fixtureRegistry/buildItems. It renders at
	// y = listTopOffset+4, same layout math as
	// TestClick_OnSkillMovesCursorTogglesAndRecomputes above.
	if m2.items[4].skill.ID != "agent-only" || !m2.items[4].disabled {
		t.Fatalf("expected items[4] to be the disabled agent-only row, got %+v", m2.items[4])
	}

	updated2, _ := m2.Update(leftClick(3, listTopOffset+4))
	m3 := updated2.(model)

	if m3.cursor != before {
		t.Errorf("cursor moved to %d on a disabled-row click, want unchanged %d", m3.cursor, before)
	}
	if m3.items[4].selected {
		t.Error("a click on a disabled row selected it, expected no selection change")
	}
}

func TestClick_OnHeaderRowIsANoOp(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g"})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m2 := updated.(model)

	before := m2.cursor
	// index 2 is the "testing" header row.
	updated2, _ := m2.Update(leftClick(3, listTopOffset+2))
	m3 := updated2.(model)

	if m3.cursor != before {
		t.Errorf("cursor moved to %d on a header click, want unchanged %d", m3.cursor, before)
	}
	for _, it := range m3.items {
		if !it.isHeader && it.selected {
			t.Errorf("a header click selected skill %q, expected no selection change", it.skill.ID)
		}
	}
}

func TestClick_DoesNotMutateThePriorModel(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g"})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m2 := updated.(model)

	updated2, _ := m2.Update(leftClick(3, listTopOffset+3))
	_ = updated2.(model)

	if m2.items[3].selected {
		t.Error("clicking on the new model mutated the prior model's items (slice aliasing)")
	}
}

func TestClick_IgnoredWhileEnteringFilename(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g"})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m2 := updated.(model)

	// open the filename input
	u, _ := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("w")})
	m3 := u.(model)

	u2, _ := m3.Update(leftClick(3, listTopOffset+3))
	m4 := u2.(model)

	if m4.mode != promptModeWriteFilename {
		t.Error("a click should not exit the filename prompt")
	}
	for _, it := range m4.items {
		if !it.isHeader && it.selected {
			t.Errorf("a click while entering a filename toggled %q, expected no change", it.skill.ID)
		}
	}
}
