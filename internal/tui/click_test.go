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
// verify(3)], so verify sits at y = listTopOffset+3.
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

	if !m4.enteringFilename {
		t.Error("a click should not exit the filename prompt")
	}
	for _, it := range m4.items {
		if !it.isHeader && it.selected {
			t.Errorf("a click while entering a filename toggled %q, expected no change", it.skill.ID)
		}
	}
}
