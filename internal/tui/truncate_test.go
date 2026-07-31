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
	"github.com/carlogy/prompt-smith/internal/registry"
)

// narrowWidthRegistry reproduces the exact names from the diagnosed
// screenshot bug: a long skill ID ("generalize-not-hardcode") under a
// long category ("communication", rendered upper-cased as
// "COMMUNICATION" by viewSkillList) - both long enough to wrap onto
// multiple display rows at a narrow content width if truncation isn't
// applied, which is precisely the failure mode this fix targets. A
// second, disabled skill (no Body, unsupported on "generic") is
// included so the "[-]" marker's survival can be asserted alongside
// the same long-label truncation.
func narrowWidthRegistry() *registry.Registry {
	return &registry.Registry{
		Categories: []string{"communication"},
		Skills: []registry.Skill{
			{ID: "generalize-not-hardcode", Category: "communication", Order: 10, Body: "body"},
			{ID: "another-unsupported-skill", Category: "communication", Order: 20}, // no Body: disabled
		},
		Targets: map[string]registry.TargetConfig{
			"generic": {ID: "generic", Delimiter: "xml", SkillMode: "inline"},
		},
	}
}

// TestTruncateToWidth_FitsUnchanged is the direct-unit-level guard for
// the "normal case is untouched" requirement: a string that already
// fits must come back byte-for-byte identical, no ellipsis appended.
func TestTruncateToWidth_FitsUnchanged(t *testing.T) {
	got := truncateToWidth("diagnose", 24)
	if got != "diagnose" {
		t.Errorf("truncateToWidth(fits) = %q, want unchanged %q", got, "diagnose")
	}
}

// TestTruncateToWidth_TruncatesWithEllipsis proves the truncated form
// fits the requested width exactly and ends in the ellipsis rune, on
// rune (not byte) boundaries.
func TestTruncateToWidth_TruncatesWithEllipsis(t *testing.T) {
	got := truncateToWidth("generalize-not-hardcode", 10)
	if w := lipgloss.Width(got); w > 10 {
		t.Errorf("truncateToWidth width = %d, want <= 10 (%q)", w, got)
	}
	if !strings.HasSuffix(got, ellipsis) {
		t.Errorf("truncateToWidth(%q, 10) = %q, want it to end in the ellipsis", "generalize-not-hardcode", got)
	}
}

// TestTruncateToWidth_NonASCIIRuneBoundary guards the rune-boundary
// requirement directly: skill names can come from
// PROMPTSMITH_SKILLS_DIR and carry non-ASCII characters, so slicing on
// byte boundaries could split a multi-byte rune and corrupt the
// output (or, with a naive approach, panic on a bad slice index).
func TestTruncateToWidth_NonASCIIRuneBoundary(t *testing.T) {
	got := truncateToWidth("caf\u00e9-r\u00e9sum\u00e9-writer", 6)
	if !utf8ValidNoop(got) {
		t.Errorf("truncateToWidth produced invalid UTF-8: %q", got)
	}
	if w := lipgloss.Width(got); w > 6 {
		t.Errorf("truncateToWidth width = %d, want <= 6 (%q)", w, got)
	}
}

// TestTruncateToWidth_DegenerateWidthsDoNotPanic covers width<=0 and
// width smaller than the ellipsis itself - the graceful-degradation
// cases requirement 3 calls out explicitly.
func TestTruncateToWidth_DegenerateWidthsDoNotPanic(t *testing.T) {
	for _, w := range []int{-5, -1, 0, 1} {
		got := truncateToWidth("generalize-not-hardcode", w)
		if lipgloss.Width(got) > w && w > 0 {
			t.Errorf("truncateToWidth(_, %d) = %q, width %d exceeds budget", w, got, lipgloss.Width(got))
		}
		if w <= 0 && got != "" {
			t.Errorf("truncateToWidth(_, %d) = %q, want empty string", w, got)
		}
	}
}

// utf8ValidNoop is a tiny local wrapper so the test above reads as an
// assertion rather than a raw utf8.ValidString call buried in an if.
func utf8ValidNoop(s string) bool {
	for _, r := range s {
		if r == '\uFFFD' {
			return false
		}
	}
	return true
}

// TestViewSkillList_NarrowWidth_OneRowPerItem is the regression test
// for the diagnosed bug: at a content width matching the observed
// failure (~11 columns - the screenshot's left-pane content width),
// every rendered item, including a long skill label and a long
// category header, must occupy exactly one display row. Before this
// fix, lipgloss.Style.Width wrapped the too-long plain text instead of
// truncating it, so N items occupied more than N rows and
// visibleWindow's item-counted scroll math fell out of sync with what
// was actually on screen.
func TestViewSkillList_NarrowWidth_OneRowPerItem(t *testing.T) {
	reg := narrowWidthRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g"})

	const windowHeight = 5 // "Skills" title + 4 list rows
	const width = 11

	out := m.viewSkillList(windowHeight, width)
	lines := strings.Split(out, "\n")

	// scrollbar() always sizes its track to listHeight regardless of how
	// many items are actually visible, and lipgloss.JoinHorizontal pads
	// the shorter of listBlock/bar to match - so the total line count is
	// always exactly windowHeight (1 title row + listHeight rows), not
	// 1 + len(visible). That padding is pre-existing scrollbar behavior,
	// unrelated to this fix; the invariant this test actually cares
	// about is that it's still windowHeight, not more - i.e. no row's
	// wrapping inflated the total past its budget.
	if len(lines) != windowHeight {
		t.Fatalf("viewSkillList produced %d lines, want exactly %d (windowHeight); output:\n%s",
			len(lines), windowHeight, stripANSI(out))
	}

	for i, line := range lines {
		if h := lipgloss.Height(line); h != 1 {
			t.Errorf("line %d has height %d, want 1 (no row should wrap): %q", i, h, stripANSI(line))
		}
	}

	if h := lipgloss.Height(out); h > windowHeight {
		t.Errorf("viewSkillList total height = %d, want <= windowHeight (%d)", h, windowHeight)
	}
}

// TestViewSkillList_DisabledMarkerSurvivesNarrowTruncation is the
// accessibility-critical case called out in the task: "[-]" is the
// only non-color signal that a skill row is disabled/unsupported, so
// it must never be truncated away, even when its label is long enough
// to need truncation itself.
func TestViewSkillList_DisabledMarkerSurvivesNarrowTruncation(t *testing.T) {
	reg := narrowWidthRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g"})

	const width = 11
	out := stripANSI(m.viewSkillList(6, width))

	if !strings.Contains(out, "[-]") {
		t.Errorf("expected the disabled row's \"[-]\" marker to survive narrow-width truncation, got:\n%s", out)
	}
	// The full label must have been shortened - it doesn't fit in 11
	// columns alongside the prefix, marker, and separator - so this is
	// also proof the truncation is actually engaging in this test.
	if strings.Contains(out, "another-unsupported-skill") {
		t.Errorf("expected the long disabled label to be truncated, got the full label:\n%s", out)
	}
}

// TestViewSkillList_ExtremeNarrowWidth_DoesNotPanic covers the
// "not even the marker fits" degenerate case: the function must clip
// gracefully rather than panic or produce a negative-length slice.
func TestViewSkillList_ExtremeNarrowWidth_DoesNotPanic(t *testing.T) {
	reg := narrowWidthRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g"})

	for _, width := range []int{10, 5, 1, 0, -3} {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("viewSkillList(6, %d) panicked: %v", width, r)
				}
			}()
			_ = m.viewSkillList(6, width)
		}()
	}
}

// TestViewSkillList_NormalWidthUnchanged is the guard that this fix
// doesn't disturb the common case: at a comfortable width where every
// fixture label already fits, output must be byte-for-byte what the
// pre-truncation composition ("prefix + marker + \" \" + label", no
// ellipsis) produced.
func TestViewSkillList_NormalWidthUnchanged(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "goal"})

	const windowHeight = 20
	const width = 24
	listHeight := windowHeight - 1

	got := stripANSI(m.viewSkillList(windowHeight, width))
	if strings.Contains(got, ellipsis) {
		t.Fatalf("expected no ellipsis at a comfortable width, got:\n%s", got)
	}

	visible, offset := visibleWindow(m.items, m.cursor, listHeight)
	lines := strings.Split(got, "\n")
	// Total line count is windowHeight, not len(visible)+1: scrollbar()
	// always sizes its track to listHeight (see
	// TestViewSkillList_NarrowWidth_OneRowPerItem's comment on the same
	// pre-existing padding), independent of this fix. Only the first
	// len(visible) rows after the title carry real content; anything
	// beyond that is blank filler.
	if len(lines) != windowHeight {
		t.Fatalf("line count = %d, want %d (windowHeight)", len(lines), windowHeight)
	}

	for i, it := range visible {
		line := strings.TrimRight(lines[i+1], " ")
		if it.isHeader {
			want := strings.ToUpper(it.category)
			if line != want {
				t.Errorf("header row %d = %q, want %q", i, line, want)
			}
			continue
		}

		mark := "[ ]"
		switch {
		case it.disabled:
			mark = "[-]"
		case it.selected:
			mark = "[x]"
		}
		prefix := "  "
		if offset+i == m.cursor && m.focus == focusSkills {
			prefix = "\u203a "
		}
		want := prefix + mark + " " + it.skill.ID
		if line != want {
			t.Errorf("skill row %d = %q, want %q (unchanged pre-truncation composition)", i, line, want)
		}
	}
}

// TestView_NarrowTerminalDoesNotPanic exercises the same narrow case
// end-to-end through the full Update/View path (WindowSizeMsg, not a
// direct viewSkillList call), mirroring how the bug actually
// manifested at runtime - a real narrow terminal, not a hand-picked
// width passed straight to the rendering function.
func TestView_NarrowTerminalDoesNotPanic(t *testing.T) {
	reg := narrowWidthRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g"})

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 20, Height: 24})
	m2 := updated.(model)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("View() panicked at a 20-column terminal: %v", r)
		}
	}()
	_ = m2.View()
}
