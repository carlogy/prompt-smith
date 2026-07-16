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

	for _, h := range []int{6, 7, 8, 10, 24, 40} {
		updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: h})
		m2 := updated.(model)

		got := lipgloss.Height(m2.View())
		if got > h {
			t.Errorf("termHeight=%d: View() height = %d, exceeds the terminal (footer must always fit)", h, got)
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
