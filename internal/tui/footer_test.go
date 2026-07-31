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

	"github.com/carlogy/prompt-smith/internal/fielddesc"
	"github.com/carlogy/prompt-smith/internal/prompt"
)

// footerFor renders the one-row footer for zone in isolation, the way
// TestFooterHelpFor_ReflectsFocusedZone (pre-bubbles/help) exercised
// footerHelpFor directly - viewFooter needs a model to read m.focus/
// m.keys/m.help from, so this builds one and focuses it rather than
// calling any single pure function, which is the shape adopting
// bubbles/help left behind (the descriptor half and the keybind half
// now come from two different places - footerDescriptorFor and
// keyMap.ShortHelp via m.help.View - see view.go).
func footerFor(t *testing.T, zone focusZone) string {
	t.Helper()
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "goal"})
	// Wide enough that no zone's descriptor+keybind row needs
	// ellipsizing - these tests are about WHAT the footer says, not
	// how it degrades under width pressure (that's
	// TestFooter_StaysOneRowAtNarrowWidth's job).
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 20})
	focused, _ := updated.(model).changeFocus(zone)
	return stripANSI(focused.(model).viewFooter())
}

func TestFooter_ReflectsFocusedZone(t *testing.T) {
	cases := []struct {
		name    string
		zone    focusZone
		want    []string
		notWant []string
	}{
		{
			name:    "skills",
			zone:    focusSkills,
			want:    []string{"move", "select", "ok"},
			notWant: []string{"type to edit", "unfocus"},
		},
		{
			name:    "a text field (goal)",
			zone:    focusGoal,
			want:    []string{"What you want the model to do.", "esc"},
			notWant: []string{"select", "ok", "pgup"},
		},
		{
			name:    "preview",
			zone:    focusPreview,
			want:    []string{"scroll", "ok"},
			notWant: []string{"select", "type to edit"},
		},
		{
			name:    "target",
			zone:    focusTarget,
			want:    []string{fielddesc.Sentence(fielddesc.Target), "change", "esc"},
			notWant: []string{"select", "ok", "pgup"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := footerFor(t, tc.zone)
			for _, want := range tc.want {
				if !strings.Contains(got, want) {
					t.Errorf("footer for %v missing %q, got: %q", tc.zone, want, got)
				}
			}
			for _, notWant := range tc.notWant {
				if strings.Contains(got, notWant) {
					t.Errorf("footer for %v should NOT contain %q, got: %q", tc.zone, notWant, got)
				}
			}
		})
	}
}

// TestFooter_TextFieldsShareKeybindsButShowTheirOwnDescriptor replaces
// the old "every field gets the same hint" assumption: that was true
// when the hint was purely mechanical ("type to edit"), but since each
// field now leads with its own fielddesc sentence (Commit 7), sameness
// there would be a bug, not a feature. What must still hold - because
// the five fields *do* share identical editing mechanics - is the
// keybind half (now keyMap.ShortHelp's default case, rendered via
// bubbles/help rather than a hand-built string, hence checking "tab"
// and "esc" rather than one exact literal suffix).
func TestFooter_TextFieldsShareKeybindsButShowTheirOwnDescriptor(t *testing.T) {
	fields := []struct {
		zone focusZone
		key  string
	}{
		{focusGoal, fielddesc.Goal},
		{focusContext, fielddesc.Context},
		{focusConstraints, fielddesc.Constraints},
		{focusRole, fielddesc.Role},
		{focusOutputFormat, fielddesc.OutputFormat},
	}

	seen := make(map[string]bool, len(fields))
	for _, f := range fields {
		got := footerFor(t, f.zone)
		for _, wantKeybind := range []string{"tab", "esc"} {
			if !strings.Contains(got, wantKeybind) {
				t.Errorf("footer for %v = %q, want it to mention %q", f.zone, got, wantKeybind)
			}
		}
		wantSentence := fielddesc.Sentence(f.key)
		if !strings.Contains(got, wantSentence) {
			t.Errorf("footer for %v = %q, want it to lead with %q", f.zone, got, wantSentence)
		}
		if seen[got] {
			t.Errorf("footer for %v = %q, want a distinct descriptor per field, got a duplicate", f.zone, got)
		}
		seen[got] = true
	}
}

func TestView_FooterChangesWithFocus(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "goal"})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 90, Height: 20})
	m2 := updated.(model) // focus=skills (goal already non-empty)

	got1 := stripANSI(m2.View())
	if !strings.Contains(got1, "select") {
		t.Errorf("expected the skills-focused footer to mention select, got:\n%s", got1)
	}

	u, _ := m2.Update(tea.KeyMsg{Type: tea.KeyTab}) // -> goal field
	m3 := u.(model)
	got2 := stripANSI(m3.View())
	if strings.Contains(got2, "select") {
		t.Errorf("expected the field-focused footer NOT to mention select, got:\n%s", got2)
	}
}

// TestFooter_StaysOneRowAtNarrowWidth is the regression guard for the
// nested-render/unbounded-help.Width hazard viewFooter's own doc
// comment warns about: without m.help.Width set, bubbles/help doesn't
// ellipsize its short help, so a narrow terminal wraps the keybind
// half onto a second physical line, silently breaking computeLayout's
// one-row-footer assumption (footerHeight=1, layout.go). 40 columns is
// narrow enough that the skills zone's full keybind list ("↑/↓ move ·
// space select · tab next field · enter confirm · c copy · w write ·
// esc cancel") could not otherwise fit on one line.
func TestFooter_StaysOneRowAtNarrowWidth(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "goal"})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 20})
	m2 := updated.(model) // focus=skills: the longest keybind list

	footer := m2.viewFooter()
	if h := lipgloss.Height(footer); h != 1 {
		t.Errorf("viewFooter() height at width 40 = %d, want exactly 1 row; got: %q", h, stripANSI(footer))
	}
}

// TestFooter_OneRowAndExamplesSubmitHintAcrossWidths is the permanent,
// table-driven regression guard for the hint-first/descriptor-second
// footer budget (viewFooter, view.go): every focus zone (focusCycle,
// focus.go, rather than an ad hoc subset) must render its footer in
// exactly one physical row, and never wider than the terminal it was
// given, at every width from 40 to 200 - regardless of whether the
// overflow would come from this package's own budget math or from
// bubbles/help's shouldAddItem itself (vendored at bubbles@v1.0.0/
// help/help.go has a boundary quirk, verified empirically at
// focusExamples/width 60, where it gives up on truncating and appends
// a full item past its own Width when even the ellipsis wouldn't fit -
// viewFooter's closing MaxWidth clamp is what catches that case).
// focusExamples' submit hint - the only explanation that Enter inserts
// a newline there instead of submitting, see keys.go's focusExamples
// case - must additionally stay fully visible at every width from 80
// upward, the width this TUI treats as its un-configured baseline
// (defaultTermWidth, layout.go).
func TestFooter_OneRowAndExamplesSubmitHintAcrossWidths(t *testing.T) {
	const examplesSubmitHint = "tab next field (then enter to submit)"
	widths := []int{40, 60, 80, 100, 120, 160, 200}

	for _, zone := range focusCycle {
		for _, w := range widths {
			reg := fixtureRegistry()
			m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "goal"})
			updated, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: 20})
			focused, _ := updated.(model).changeFocus(zone)
			footer := focused.(model).viewFooter()

			if h := lipgloss.Height(footer); h != 1 {
				t.Errorf("zone=%v width=%d: viewFooter() height = %d, want 1; got %q", zone, w, h, stripANSI(footer))
			}
			if fw := lipgloss.Width(footer); fw > w {
				t.Errorf("zone=%v width=%d: viewFooter() rendered width = %d, exceeds the terminal width; got %q", zone, w, fw, stripANSI(footer))
			}

			if zone == focusExamples && w >= 80 {
				got := stripANSI(footer)
				if !strings.Contains(got, examplesSubmitHint) {
					t.Errorf("zone=focusExamples width=%d: footer missing submit hint %q, got %q", w, examplesSubmitHint, got)
				}
			}
		}
	}
}
