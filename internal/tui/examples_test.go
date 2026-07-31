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
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/carlogy/prompt-smith/internal/prompt"
)

// tabToExamples advances a fresh model (started on focusSkills) to
// focusExamples: skills -> goal -> context -> constraints -> role ->
// outputFormat -> examples is 6 tabs, matching focusCycle's order
// (focus.go).
func tabToExamples(t *testing.T, m model) model {
	t.Helper()
	cur := m
	for i := 0; i < 6; i++ {
		u, _ := cur.Update(tea.KeyMsg{Type: tea.KeyTab})
		cur = u.(model)
	}
	if cur.focus != focusExamples {
		t.Fatalf("expected focusExamples after 6 tabs, got %v", cur.focus)
	}
	return cur
}

func TestFocus_TabReachesExamplesInTheRightPosition(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g"})

	cur := tabToExamples(t, m)
	if !cur.examplesInput.Focused() {
		t.Error("expected examplesInput to be focused after tabbing to focusExamples")
	}

	// One more tab reaches the preview, matching focusCycle's order.
	u, _ := cur.Update(tea.KeyMsg{Type: tea.KeyTab})
	next := u.(model)
	if next.focus != focusPreview {
		t.Errorf("focus after one more tab = %v, want focusPreview", next.focus)
	}
}

func TestFocus_ShiftTabReachesExamplesInTheRightPosition(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g"})

	// Shift+Tab from skills wraps to target, then preview, then
	// examples - see focusCycle in focus.go.
	cur := m
	for i := 0; i < 3; i++ {
		u, _ := cur.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
		cur = u.(model)
	}
	if cur.focus != focusExamples {
		t.Fatalf("focus after 3 Shift+Tab(s) = %v, want focusExamples", cur.focus)
	}
}

func TestExamples_EnterInsertsNewlineAndDoesNotQuit(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g"})
	cur := tabToExamples(t, m)

	for _, r := range "line one" {
		u, _ := cur.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		cur = u.(model)
	}

	updated, cmd := cur.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := updated.(model)

	if cmd != nil {
		msg := cmd()
		if _, isQuit := msg.(tea.QuitMsg); isQuit {
			t.Error("expected Enter in the Examples field not to quit the program")
		}
	}
	if m2.result.Action == ActionStdout {
		t.Error("expected Enter in the Examples field not to set ActionStdout")
	}
	if !strings.Contains(m2.examplesInput.Value(), "\n") {
		t.Errorf("expected Enter to insert a newline into the textarea, got %q", m2.examplesInput.Value())
	}
	if !strings.HasPrefix(m2.examplesInput.Value(), "line one\n") {
		t.Errorf("expected the newline to land after the typed text, got %q", m2.examplesInput.Value())
	}
}

func TestExamples_EscReturnsFocusToSkills(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g"})
	cur := tabToExamples(t, m)

	updated, cmd := cur.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m2 := updated.(model)

	if m2.focus != focusSkills {
		t.Errorf("focus after Esc in the Examples field = %v, want focusSkills", m2.focus)
	}
	if cmd != nil {
		msg := cmd()
		if _, isQuit := msg.(tea.QuitMsg); isQuit {
			t.Error("expected Esc in the Examples field not to quit the program")
		}
	}
}

func TestNewModel_SeedsExamplesAndRoundTripsUnchanged(t *testing.T) {
	reg := fixtureRegistry()
	seeded := []string{"first example", "second example"}
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g", Examples: seeded})

	// The round-trip contract (prompt.SplitExamples(prompt.JoinExamples(x))
	// == prompt.NormalizeExamples(x)) means an untouched seed must come
	// back out of currentInputs() exactly as it went in.
	got := m.currentInputs().Examples
	if len(got) != len(seeded) {
		t.Fatalf("currentInputs().Examples = %v, want %v", got, seeded)
	}
	for i := range seeded {
		if got[i] != seeded[i] {
			t.Errorf("currentInputs().Examples[%d] = %q, want %q", i, got[i], seeded[i])
		}
	}
}

func TestExamples_DashSeparatedTextYieldsMultipleExamples(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g"})
	cur := tabToExamples(t, m)

	for _, r := range "first\n---\nsecond" {
		var u tea.Model
		if r == '\n' {
			u, _ = cur.Update(tea.KeyMsg{Type: tea.KeyEnter})
		} else {
			u, _ = cur.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		}
		cur = u.(model)
	}

	got := cur.currentInputs().Examples
	want := []string{"first", "second"}
	if len(got) != len(want) {
		t.Fatalf("currentInputs().Examples = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("currentInputs().Examples[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestExamples_LivePreviewIncludesExamplesTagAfterEdit(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g"})

	if strings.Contains(m.preview, "<examples>") {
		t.Fatalf("expected no <examples> section before typing any, got:\n%s", m.preview)
	}

	cur := tabToExamples(t, m)
	for _, r := range "an example" {
		u, _ := cur.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		cur = u.(model)
	}

	if !strings.Contains(cur.preview, "<examples>") {
		t.Errorf("expected the live preview to gain an <examples> section after typing, got:\n%s", cur.preview)
	}
	if !strings.Contains(cur.preview, "an example") {
		t.Errorf("expected the live preview to include the typed example text, got:\n%s", cur.preview)
	}
}

func TestView_FieldsBlockHeightMatchesTotalFieldsHeight(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "goal"})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 90, Height: 20})
	m2 := updated.(model)

	l := computeLayout(m2.termWidth, m2.termHeight)
	fieldsBlock := m2.viewFields(l.leftContentWidth)
	if h := lipgloss.Height(fieldsBlock); h != totalFieldsHeight() {
		t.Errorf("viewFields height = %d, want totalFieldsHeight() = %d", h, totalFieldsHeight())
	}

	// The overall View() must still respect the terminal height bound
	// with the taller (Examples-inclusive) field stack - mirrors
	// TestView_TotalHeightNeverExceedsTerminalHeight (view_height_test.go).
	got := lipgloss.Height(m2.View())
	if got > 20 {
		t.Errorf("View() height = %d, exceeds the 20-row terminal", got)
	}
}

// TestFooter_ExamplesMentionsNewlineAndTabToSubmit used to call
// m.changeFocus directly, with no tea.WindowSizeMsg ever sent - which
// left m.termWidth at its zero value. viewFooter's own fallback
// (termWidth <= 0 -> defaultTermWidth, 80) meant that omission didn't
// make the test pass at some arbitrary invalid width; it made it pass
// at exactly the one width where the PRE-fix budget rule was at its
// most broken: footerDescriptorFor's full Examples sentence (112
// columns, wider than 80 on its own) drove help.Width negative, which
// got clamped to 0, and bubbles/help's shouldAddItem only truncates
// `if m.Width > 0` (vendored at bubbles@v1.0.0/help/help.go) - so at
// Width==0 nothing ellipsized at all and the complete, untruncated 186-
// column keybind+descriptor string always contained "newline"/"tab"/
// "submit" regardless of what the fix under test actually did. Sending
// an explicit WindowSizeMsg at several realistic widths - including
// ones at and above defaultTermWidth, where the new hint-first budget
// (view.go) is what's actually responsible for keeping the submit hint
// intact - is what turns this back into a real regression guard rather
// than one that would keep passing even if the priority were reverted.
func TestFooter_ExamplesMentionsNewlineAndTabToSubmit(t *testing.T) {
	for _, w := range []int{80, 120, 200} {
		reg := fixtureRegistry()
		m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "goal"})
		updated, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: 20})
		focused, _ := updated.(model).changeFocus(focusExamples)
		m2 := focused.(model)

		got := strings.ToLower(stripANSI(m2.viewFooter()))
		for _, want := range []string{"newline", "tab", "submit"} {
			if !strings.Contains(got, want) {
				t.Errorf("width=%d: viewFooter() with focusExamples = %q, want it to mention %q", w, got, want)
			}
		}
	}
}

// TestModel_EndToEnd_TypeExamplesTabAwayThenConfirm drives the model
// through a real Bubble Tea program loop (teatest), matching how
// tui_test.go exercises every other end-to-end path, rather than
// calling Update directly: type multiple "---"-separated examples into
// the field, Tab away (proving Enter alone couldn't have submitted -
// see updateExamplesField), and confirm the final result carries both
// examples through prompt.SplitExamples.
func TestModel_EndToEnd_TypeExamplesTabAwayThenConfirm(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "goal"})

	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(90, 24))

	// skills -> goal -> context -> constraints -> role -> outputFormat
	// -> examples: 6 tabs (focusCycle's order, focus.go).
	for i := 0; i < 6; i++ {
		tm.Send(tea.KeyMsg{Type: tea.KeyTab})
	}
	tm.Type("first example")
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter}) // inserts a newline, does NOT submit
	tm.Type("---")
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	tm.Type("second example")

	tm.Send(tea.KeyMsg{Type: tea.KeyTab}) // -> preview
	tm.Send(tea.KeyMsg{Type: tea.KeyTab}) // -> target
	tm.Send(tea.KeyMsg{Type: tea.KeyTab}) // -> skills
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	final := tm.FinalModel(t).(model)
	if final.result.Action != ActionStdout {
		t.Errorf("Action = %v, want ActionStdout", final.result.Action)
	}
	want := []string{"first example", "second example"}
	got := final.result.Inputs.Examples
	if len(got) != len(want) {
		t.Fatalf("Inputs.Examples = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Inputs.Examples[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
