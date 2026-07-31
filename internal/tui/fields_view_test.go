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
	for i := 0; i < 7; i++ {
		u, _ := cur.Update(tea.KeyMsg{Type: tea.KeyTab})
		cur = u.(model)
	}
	if cur.focus != focusPreview {
		t.Fatalf("expected preview focus after 7 tabs, got %v", cur.focus)
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
	// must always produce exactly totalFieldsHeight() lines (numFields
	// no longer doubles as a height once Examples' textarea renders
	// more than one row - see layout.go), wrapping produces more.
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
	if len(lines) != totalFieldsHeight() {
		t.Errorf("viewFields produced %d lines, want exactly %d (a field row wrapped): %q",
			len(lines), totalFieldsHeight(), stripANSI(fieldsBlock))
	}

	maxWidth := l.leftContentWidth - scrollbarWidth
	for i, line := range lines {
		if w := lipgloss.Width(line); w > maxWidth {
			t.Errorf("field row %d width = %d, want <= %d: %q", i, w, maxWidth, stripANSI(line))
		}
	}
}

// TestView_FieldLabelsFitNarrowPane is the fields-pane counterpart to
// truncate_test.go's TestViewSkillList_NarrowWidth_OneRowPerItem,
// covering the structurally identical defect one pane over (see
// 2ae848a for the skill-list fix this mirrors): viewFields composed
// its five "Label: value" rows and joined them with the multi-line
// Examples block, then handed the WHOLE block to a single
// lipgloss.Style.Width call. Width() pads short lines but WRAPS long
// ones mid-word - and at a terminal narrow enough that leftContentWidth
// drops into the teens, "Constraints" (fieldLabelWidth, the longest
// label at 11 columns) plus its 2-col cursor-marker prefix and 2-col
// ": " separator alone - BEFORE any value is even considered - already
// exceeds the pane's content width. Termwidth 50 lands leftContentWidth
// at exactly 12 (50/3=16, -paneHOverhead(4)=12; see
// TestComputeLayout_SplitsWidthAndReservesFooterAndBorders for the
// same arithmetic), squarely in the roadmap's specified 40-60 range and
// well past that threshold, so this is expected to fail on unfixed
// code with "Constraints" breaking into "Constrai"/"nts:" across two
// physical rows - pushing the block past totalFieldsHeight()'s budget.
func TestView_FieldLabelsFitNarrowPane(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g"})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 50, Height: 24})
	m2 := updated.(model)

	l := computeLayout(m2.termWidth, m2.termHeight)
	block := stripANSI(m2.viewFields(l.leftContentWidth))
	lines := strings.Split(block, "\n")

	if len(lines) != totalFieldsHeight() {
		t.Fatalf("viewFields produced %d lines, want exactly %d (totalFieldsHeight); a row wrapped. output:\n%s",
			len(lines), totalFieldsHeight(), block)
	}

	maxWidth := l.leftContentWidth - scrollbarWidth
	for i, line := range lines {
		if h := lipgloss.Height(line); h != 1 {
			t.Errorf("line %d has height %d, want 1 (no row should wrap): %q", i, h, line)
		}
		if w := lipgloss.Width(line); w > maxWidth {
			t.Errorf("line %d width = %d, want <= %d: %q", i, w, maxWidth, line)
		}
	}
}

// TestView_ExtremeNarrowFieldsDoesNotPanic is the fields-pane
// counterpart to truncate_test.go's
// TestViewSkillList_ExtremeNarrowWidth_DoesNotPanic: at widths too
// small for even the label's ellipsis to fit, viewFields must degrade
// gracefully (rows may still overflow/wrap - that's an acceptable
// extreme-narrow tradeoff no other test in this file asserts against)
// rather than panic, e.g. from a negative slice bound.
func TestView_ExtremeNarrowFieldsDoesNotPanic(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g"})

	for _, width := range []int{11, 5, 1, 0, -3} {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("viewFields(%d) panicked: %v", width, r)
				}
			}()
			_ = m.viewFields(width)
		}()
	}
}

// TestView_NarrowTerminalFieldsPaneDoesNotPanic exercises the same
// narrow case end-to-end through the full Update/View path
// (WindowSizeMsg, not a direct viewFields call), mirroring
// truncate_test.go's TestView_NarrowTerminalDoesNotPanic for the skill
// list - a real narrow terminal, not a hand-picked width passed
// straight to the rendering function.
func TestView_NarrowTerminalFieldsPaneDoesNotPanic(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g"})

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 50, Height: 20})
	m2 := updated.(model)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("View() panicked at a 50-column terminal: %v", r)
		}
	}()
	_ = m2.View()
}
