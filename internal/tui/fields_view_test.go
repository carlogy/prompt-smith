package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/carlogy/prompt-smith/internal/prompt"
)

func TestView_ShowsAllFieldLabels(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g"})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 90, Height: 20})
	m2 := updated.(model)

	got := stripANSI(m2.View())
	for _, label := range []string{"Goal", "Context", "Constraints", "Role", "Output"} {
		if !strings.Contains(got, label) {
			t.Errorf("View() missing field label %q, got:\n%s", label, got)
		}
	}
}

func TestView_FocusedFieldIsMarked(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g"})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 90, Height: 20})
	m2 := updated.(model)

	u2, _ := m2.Update(tea.KeyMsg{Type: tea.KeyTab}) // -> goal
	m3 := u2.(model)
	got := stripANSI(m3.View())

	goalLine := lineContaining(t, got, "Goal")
	if !strings.Contains(goalLine, "\u203a") {
		t.Errorf("expected the focused Goal row to be marked with \u203a, got: %q", goalLine)
	}
}

func TestView_UnfocusedFieldsAreNotMarked(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g"})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 90, Height: 20})
	m2 := updated.(model) // focus stays on skills (goal is non-empty)

	got := stripANSI(m2.View())
	contextLine := lineContaining(t, got, "Context")
	if strings.Contains(contextLine, "\u203a") {
		t.Errorf("expected an unfocused Context row NOT to be marked, got: %q", contextLine)
	}
}

func TestView_FocusedPreviewIsMarked(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g"})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 90, Height: 20})
	m2 := updated.(model)

	cur := m2
	for i := 0; i < 6; i++ {
		u, _ := cur.Update(tea.KeyMsg{Type: tea.KeyTab})
		cur = u.(model)
	}
	if cur.focus != focusPreview {
		t.Fatalf("expected preview focus after 6 tabs, got %v", cur.focus)
	}

	got := stripANSI(cur.View())
	previewLine := lineContaining(t, got, "Preview")
	if !strings.Contains(previewLine, "\u203a") {
		t.Errorf("expected the focused Preview title to be marked with \u203a, got: %q", previewLine)
	}
}

func lineContaining(t *testing.T, s, substr string) string {
	t.Helper()
	for _, line := range strings.Split(s, "\n") {
		if strings.Contains(line, substr) {
			return line
		}
	}
	t.Fatalf("no line containing %q found in:\n%s", substr, s)
	return ""
}

func TestView_FieldRowsDoNotWrapWithLongValues(t *testing.T) {
	// Every field row is prefixed with "\u203a " (focused) or "  "
	// (unfocused) AFTER the input's own width was budgeted - if that
	// 2-char prefix isn't ALSO subtracted from the input's width, the
	// composed row is 2 cols wider than the pane's content width, and
	// lipgloss.Width wraps it onto a second physical line instead of
	// leaving it to the input's own horizontal scroll. A per-line WIDTH
	// check can't catch this (each wrapped sub-line individually fits,
	// by definition) - the real signal is the LINE COUNT: viewFields
	// must always produce exactly numFields lines, wrapping produces
	// more.
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "goal"})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 90, Height: 20})
	m2 := updated.(model)

	u, _ := m2.Update(tea.KeyMsg{Type: tea.KeyTab}) // -> goal
	cur := u.(model)
	for _, r := range "this is a fairly long goal that should not wrap the row" {
		uu, _ := cur.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		cur = uu.(model)
	}

	l := computeLayout(cur.termWidth, cur.termHeight)
	fieldsBlock := cur.viewFields(l.leftContentWidth)
	lines := strings.Split(fieldsBlock, "\n")
	if len(lines) != numFields {
		t.Errorf("viewFields produced %d lines, want exactly %d (a field row wrapped): %q",
			len(lines), numFields, stripANSI(fieldsBlock))
	}

	maxWidth := l.leftContentWidth - scrollbarWidth
	for i, line := range lines {
		if w := lipgloss.Width(line); w > maxWidth {
			t.Errorf("field row %d width = %d, want <= %d: %q", i, w, maxWidth, stripANSI(line))
		}
	}
}
