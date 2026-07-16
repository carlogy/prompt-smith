package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/carlogy/prompt-smith/internal/prompt"
)

func TestSkills_ShowsScrollbarThumbOnlyWhenListOverflows(t *testing.T) {
	// fixtureRegistry on "generic" is 4 items:
	// [header:debugging, diagnose, header:testing, verify].
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g"})

	// windowHeight=3 -> listHeight=2 rows for 4 items: overflows.
	overflow := m.viewSkillList(3, 24)
	if !strings.Contains(overflow, scrollThumb) {
		t.Errorf("expected a scrollbar thumb when the skill list overflows, got:\n%s", stripANSI(overflow))
	}

	// windowHeight=20 -> listHeight=19 rows: all 4 items fit, no thumb.
	fits := m.viewSkillList(20, 24)
	if strings.Contains(fits, scrollThumb) {
		t.Errorf("expected no scrollbar thumb when the skill list fits, got:\n%s", stripANSI(fits))
	}
}

func TestSkills_ScrollbarSitsWithinTheContentWidth(t *testing.T) {
	// Every rendered line of the skill list block must be exactly the
	// requested content width, so the bar aligns flush against the pane
	// border rather than floating at a ragged right edge.
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g"})

	const width = 24
	block := m.viewSkillList(3, width)
	for i, line := range strings.Split(block, "\n") {
		if i == 0 {
			continue // the "Skills" title line isn't part of the width-constrained body
		}
		if w := lipgloss.Width(line); w != width {
			t.Errorf("skill body line %d width = %d, want %d: %q", i, w, width, stripANSI(line))
		}
	}
}
