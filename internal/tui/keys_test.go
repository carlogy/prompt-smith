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

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/carlogy/prompt-smith/internal/prompt"
)

// TestKeySpace_MatchesLiteralSpaceBinding pins the exact hazard a
// "space" binding invites: bubbletea's KeySpace stringifies to a
// literal " " (a single space character), not the word "space" - a
// key.Binding built with key.WithKeys("space") would silently never
// match a real spacebar press. This is a pure key.Matches check, no
// model involved, so a regression here can't be masked by some other
// path (e.g. a stray fallback) still toggling the skill.
func TestKeySpace_MatchesLiteralSpaceBinding(t *testing.T) {
	keys := newKeyMap()
	msg := tea.KeyMsg{Type: tea.KeySpace}

	if !key.Matches(msg, keys.Space) {
		t.Errorf("key.Matches(KeySpace, keys.Space) = false, want true (KeySpace.String() = %q)", msg.String())
	}
}

// TestFocus_SpaceStillTogglesSkillSelection is the end-to-end
// counterpart to TestKeySpace_MatchesLiteralSpaceBinding: a real
// tea.KeySpace message, routed through Update the same way a live
// terminal session would send it, still toggles the cursor's skill -
// proving the fact-1 space-binding fix (key.WithKeys(" "), not
// key.WithKeys("space")) actually reaches the dispatch path and not
// just the standalone key.Matches call above.
func TestFocus_SpaceStillTogglesSkillSelection(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g"})
	if m.focus != focusSkills {
		t.Fatal("expected default focus on skills")
	}
	before := m.items[m.cursor].selected

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m2 := updated.(model)

	if m2.items[m2.cursor].selected == before {
		t.Errorf("expected Space to toggle the cursor's skill selection, stayed %v", before)
	}
}

// TestHelpOverlay_QuestionMarkOpensFullScreenTakeover exercises "?"
// from focusSkills (updatePicker's tea.KeyRunes branch - see the
// comment there for why this can't be a key.Matches binding): it must
// flip m.help.ShowAll and View() must switch to the full-screen
// overlay (viewHelpOverlay) instead of the normal split view.
func TestHelpOverlay_QuestionMarkOpensFullScreenTakeover(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g"})

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	m2 := updated.(model)

	if !m2.help.ShowAll {
		t.Fatal("expected \"?\" to set help.ShowAll = true")
	}
	got := stripANSI(m2.View())
	if !strings.Contains(got, "Help") {
		t.Errorf("expected View() to render the help overlay, got:\n%s", got)
	}
	// The overlay replaces the whole split view - none of the normal
	// chrome (the bordered panes) should still be on screen.
	if strings.ContainsAny(got, "\u2502\u256d\u256e\u2570\u256f") {
		t.Errorf("expected the overlay to replace the split view entirely, still saw a pane border:\n%s", got)
	}
}

// TestHelpOverlay_EscDismissesBackToTheSplitView proves the overlay
// has a discoverable, working dismiss key distinct from its own
// toggle key.
func TestHelpOverlay_EscDismissesBackToTheSplitView(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g"})

	opened, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	m2 := opened.(model)
	if !m2.help.ShowAll {
		t.Fatal("setup: expected the overlay to be open")
	}

	closed, _ := m2.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m3 := closed.(model)
	if m3.help.ShowAll {
		t.Error("expected Esc to close the help overlay")
	}
	if m3.focus != focusSkills {
		t.Errorf("expected focus to still be focusSkills after closing the overlay, got %v", m3.focus)
	}
}

// TestHelpOverlay_QuestionMarkAgainAlsoDismisses mirrors the overlay's
// own toggle key working both ways, matching how ShowAll itself is a
// bool flip in updatePicker.
func TestHelpOverlay_QuestionMarkAgainAlsoDismisses(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g"})

	opened, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	closed, _ := opened.(model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	m2 := closed.(model)

	if m2.help.ShowAll {
		t.Error("expected a second \"?\" to close the help overlay")
	}
}

// TestHelpOverlay_QuestionMarkTypesLiterallyIntoTextFields is the
// negation requirement: "?" is only bound to the overlay in
// focusSkills/focusPreview (routed through updatePicker), so a user
// editing e.g. the goal field must still be able to type a literal
// "?" character rather than have it hijacked into opening the
// overlay.
func TestHelpOverlay_QuestionMarkTypesLiterallyIntoTextFields(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: ""}) // empty goal -> focus starts on focusGoal
	if m.focus != focusGoal {
		t.Fatal("setup: expected focus on the goal field")
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	m2 := updated.(model)

	if m2.help.ShowAll {
		t.Error("expected \"?\" while a text field is focused NOT to open the help overlay")
	}
	if m2.goal != "?" {
		t.Errorf("expected \"?\" to be typed literally into the goal field, got %q", m2.goal)
	}
}

// TestShortHelp_FocusSkillsAdvertisesSave proves the "s" save-preset
// hint made it into the one-row footer's skills-zone keybind list
// (ShortHelp's focusSkills case) despite that row already sitting at
// its 80-column budget before "s" was added (see the doc comment
// there) - confirming the shortening described there actually made
// room rather than silently dropping the new entry off the
// ellipsized tail. footer_test.go/view_height_test.go cover the
// row-stays-one-line and never-disappears guarantees; this covers the
// content itself, which those two intentionally don't (see the task
// note not to edit either file).
func TestShortHelp_FocusSkillsAdvertisesSave(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "goal"})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	m2 := updated.(model) // focus=skills

	got := stripANSI(m2.viewFooter())
	if !strings.Contains(got, "s save") {
		t.Errorf("expected the skills-focused footer to advertise \"s save\", got: %q", got)
	}
}

// TestShortHelp_PriorityOrderSurvivesTruncation pins the acceptance
// criteria behind ShortHelp's focusSkills/focusPreview cases (see the
// doc comment above them in keys.go): "esc cancel" is pinned LAST by
// convention rather than by need, which only works because the row is
// kept within the standard 80-column footer in the first place - so
// what this test actually pins is that 80-column fit, not "cancel
// survives truncation" the way an order-is-priority scheme alone would
// give you. At 80 and above, the full row (including "s save" and
// "tab next", the two entries that would be the first casualties if
// the row didn't fit) must render intact with "esc cancel" as its
// final entry. Below 80, ellipsis is expected to start eating the
// row - from the front of the "these can go" region, i.e. right after
// the last conventionally-placed entry - and "esc cancel" is allowed
// to go with it; the row staying exactly one line either way is
// TestFooter_StaysOneRowAtNarrowWidth's job, not this test's.
func TestShortHelp_PriorityOrderSurvivesTruncation(t *testing.T) {
	reg := fixtureRegistry()

	for _, tc := range []struct {
		name string
		zone focusZone
	}{
		{"skills", focusSkills},
		{"preview", focusPreview},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "goal"})

			for _, w := range []int{80, 100, 120} {
				updated, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: 20})
				mm, _ := updated.(model).changeFocus(tc.zone)
				got := stripANSI(mm.(model).viewFooter())
				for _, want := range []string{"cancel", "s save", "tab next"} {
					if !strings.Contains(got, want) {
						t.Errorf("width=%d: expected footer to advertise %q, got: %q", w, want, got)
					}
				}
				if !strings.HasSuffix(got, "esc cancel") {
					t.Errorf("width=%d: expected \"esc cancel\" to be the final entry, got: %q", w, got)
				}
			}

			updated, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 20})
			mm, _ := updated.(model).changeFocus(tc.zone)
			footer := mm.(model).viewFooter()
			if h := lipgloss.Height(footer); h != 1 {
				t.Errorf("width=40: viewFooter() height = %d, want exactly 1 row; got: %q", h, stripANSI(footer))
			}
		})
	}
}
