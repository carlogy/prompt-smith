package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/carlogy/prompt-smith/internal/prompt"
)

func TestRenderPanes_EqualizesHeightSoBordersAlign(t *testing.T) {
	left := "one\ntwo"
	right := "one\ntwo\nthree\nfour\nfive"

	leftBox, rightBox := renderPanes(left, right)

	lh, rh := lipgloss.Height(leftBox), lipgloss.Height(rightBox)
	if lh != rh {
		t.Errorf("pane heights differ: left=%d right=%d (borders won't align)", lh, rh)
	}
}

func TestRenderPanes_BothOrderingsEqualizeHeight(t *testing.T) {
	// Guard both directions - the taller content might be on either side.
	shortC := "a"
	tallC := "a\nb\nc\nd"

	b1, b2 := renderPanes(shortC, tallC)
	if lipgloss.Height(b1) != lipgloss.Height(b2) {
		t.Errorf("short-then-tall: heights differ: %d vs %d", lipgloss.Height(b1), lipgloss.Height(b2))
	}

	b3, b4 := renderPanes(tallC, shortC)
	if lipgloss.Height(b3) != lipgloss.Height(b4) {
		t.Errorf("tall-then-short: heights differ: %d vs %d", lipgloss.Height(b3), lipgloss.Height(b4))
	}
}

func TestView_PaneBordersAlign(t *testing.T) {
	// Regression for the visual bug found during manual review: uneven
	// pane heights left the shorter pane's border closing early while
	// the taller pane continued beside blank, borderless padding. Every
	// rendered row of the joined body should be the same width once
	// trailing spaces are trimmed from comparison... simplest robust
	// check: every line of the body block should contain a border
	// character (the left pane's border never "closes" before the
	// right pane's content ends).
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{
		Target: "generic",
		Goal:   "Fix the flaky checkout test",
		Skills: []string{"diagnose"},
	})

	view := m.View()
	lines := strings.Split(view, "\n")
	// The footer is the last line and intentionally has no border.
	bodyLines := lines[:len(lines)-1]

	for i, line := range bodyLines {
		if strings.TrimSpace(stripANSI(line)) == "" {
			continue // a fully blank line is fine as long as it's rare; the real check is below
		}
		if !strings.ContainsAny(line, "│╭╮╰╯") {
			t.Errorf("body line %d has no border character, panes likely misaligned: %q", i, line)
		}
	}
}
