package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/carlogy/prompt-smith/internal/prompt"
)

// TestFocus_RightArrowOnTargetAdvancesAndRefiltersItems drives a fresh
// model to focusTarget (one Shift+Tab from the default focusSkills,
// since focusTarget sits immediately before focusSkills in the cycle),
// then presses Right and checks two things at once: m.target actually
// advanced to the next id in sortedTargetIDs order, and the item set
// was refiltered by the new target's SupportsTarget rules - "generic"
// (SkillMode: inline) excludes the Body-less "agent-only" skill, but
// "opencode" (SkillMode: reference) supports every skill regardless of
// Body, so switching from "generic" to "opencode" must make
// "agent-only" appear.
func TestFocus_RightArrowOnTargetAdvancesAndRefiltersItems(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g"})

	if hasItem(m.items, "agent-only") {
		t.Fatal("expected agent-only to be absent on the initial target (generic)")
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
		t.Error("expected agent-only to appear once the target switched to opencode (reference mode supports every skill)")
	}
	if cur2.items[cur2.cursor].isHeader {
		t.Error("expected the cursor to land on a selectable item after the target change")
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
