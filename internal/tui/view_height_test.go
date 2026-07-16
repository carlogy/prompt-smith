package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/carlogy/prompt-smith/internal/prompt"
)

func TestView_TotalHeightNeverExceedsTerminalHeight(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "goal", Skills: []string{"diagnose"}})

	// Since P3c, the fields section is a fixed numFields rows, on top of
	// a minimally-useful skills section (minSkillsHeight) - that's a
	// hard structural floor (plus borders + footer) the layout can't
	// shrink below, no matter how tiny the terminal actually is. Below
	// that floor, View() clamps to the floor rather than the terminal's
	// too-small height; at or above it, it must never exceed the
	// terminal - both bounds checked explicitly.
	minimumUsableHeight := numFields + minSkillsHeight + paneBorderRows + footerHeight

	for _, h := range []int{6, 7, 8, 10, 24, 40} {
		updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: h})
		m2 := updated.(model)

		got := lipgloss.Height(m2.View())
		want := h
		if want < minimumUsableHeight {
			want = minimumUsableHeight
		}
		if got > want {
			t.Errorf("termHeight=%d: View() height = %d, exceeds %d (the terminal, or the documented structural minimum)", h, got, want)
		}
	}
}

func TestView_FooterAlwaysPresentRegardlessOfContent(t *testing.T) {
	reg := longBodyRegistry("a", "b", "c", "d", "e", "f", "g", "h", "i", "j")
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "goal", Skills: []string{"longskill"}})

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 8})
	m2 := updated.(model)

	got := stripANSI(m2.View())
	if !strings.Contains(got, "cancel") {
		t.Errorf("expected the footer to always be present even with overflowing content, got:\n%s", got)
	}
}
