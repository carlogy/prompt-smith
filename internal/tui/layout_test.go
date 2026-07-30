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

import "testing"

func TestComputeLayout_SplitsWidthAndReservesFooterAndBorders(t *testing.T) {
	l := computeLayout(80, 24)

	// 1 line for the footer + 2 border rows per pane reserved.
	wantContentHeight := 24 - 1 - 2
	if l.contentHeight != wantContentHeight {
		t.Errorf("contentHeight = %d, want %d", l.contentHeight, wantContentHeight)
	}

	// Left pane gets roughly a third, right gets the rest, both net of
	// border+padding overhead (4 cols/pane: 2 border + 2 padding).
	wantLeft := 80/3 - 4
	if l.leftContentWidth != wantLeft {
		t.Errorf("leftContentWidth = %d, want %d", l.leftContentWidth, wantLeft)
	}

	wantRight := (80 - 80/3) - 4
	if l.rightContentWidth != wantRight {
		t.Errorf("rightContentWidth = %d, want %d", l.rightContentWidth, wantRight)
	}
}

func TestComputeLayout_ClampsTinyTerminalsToAMinimum(t *testing.T) {
	l := computeLayout(10, 4)

	if l.contentHeight < 1 {
		t.Errorf("contentHeight = %d, want >= 1 even for a tiny terminal", l.contentHeight)
	}
	if l.leftContentWidth < 1 || l.rightContentWidth < 1 {
		t.Errorf("content widths must stay >= 1, got left=%d right=%d", l.leftContentWidth, l.rightContentWidth)
	}
}

func TestComputeLayout_ZeroSizeFallsBackToAUsableDefault(t *testing.T) {
	// Before the first WindowSizeMsg arrives, dimensions are the zero
	// value; layout must still produce something usable, not a
	// degenerate/negative size.
	l := computeLayout(0, 0)

	if l.contentHeight < 1 || l.leftContentWidth < 1 || l.rightContentWidth < 1 {
		t.Errorf("zero-size input must fall back to a usable default, got %+v", l)
	}
}

// TestComputeLayout_FieldHeightSumMatchesOldCountBasedMath is the
// regression net for the fieldsHeight/minRequiredContentHeight
// refactor from a bare numFields (a field *count*) to
// totalFieldsHeight() (a sum of per-field heights). Every entry in
// fieldHeights is still 1 in this phase, so numFields and
// totalFieldsHeight() are numerically identical - meaning every
// expected value below was hand-computed with the *old* formula
// (fieldsHeight = numFields = 5) and must still match computeLayout's
// output exactly. If this test ever goes red while fieldHeights is
// untouched, the refactor changed observable behavior, which it isn't
// supposed to.
//
// The spread of sizes deliberately includes both zero-value fallback
// (0x0) and a genuinely tiny-but-nonzero terminal (1x1), plus a size
// that lands exactly one row under minRequiredContentHeight before the
// floor kicks in (80x10: raw contentHeight = 10-1-2 = 7, one under the
// 8-row floor) - the three cases most likely to expose an off-by-one
// if the sum/count swap introduced one.
func TestComputeLayout_FieldHeightSumMatchesOldCountBasedMath(t *testing.T) {
	cases := []struct {
		name string
		w, h int
		want layout
	}{
		{
			name: "0x0 falls back to the 80x24 default",
			w:    0, h: 0,
			want: layout{leftContentWidth: 22, rightContentWidth: 50, contentHeight: 21, fieldsHeight: 5, skillsHeight: 15},
		},
		{
			name: "1x1 degenerate: every dimension floors to its minimum",
			w:    1, h: 1,
			want: layout{leftContentWidth: 1, rightContentWidth: 1, contentHeight: 8, fieldsHeight: 5, skillsHeight: 2},
		},
		{
			name: "80x24, the common default terminal, via the non-fallback path",
			w:    80, h: 24,
			want: layout{leftContentWidth: 22, rightContentWidth: 50, contentHeight: 21, fieldsHeight: 5, skillsHeight: 15},
		},
		{
			name: "200x50, comfortably large",
			w:    200, h: 50,
			want: layout{leftContentWidth: 62, rightContentWidth: 130, contentHeight: 47, fieldsHeight: 5, skillsHeight: 41},
		},
		{
			name: "10x4, small width and height both floor",
			w:    10, h: 4,
			want: layout{leftContentWidth: 1, rightContentWidth: 3, contentHeight: 8, fieldsHeight: 5, skillsHeight: 2},
		},
		{
			name: "80x10, one row under minRequiredContentHeight before flooring",
			w:    80, h: 10,
			want: layout{leftContentWidth: 22, rightContentWidth: 50, contentHeight: 8, fieldsHeight: 5, skillsHeight: 2},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := computeLayout(c.w, c.h)
			if got != c.want {
				t.Errorf("computeLayout(%d, %d) = %+v, want %+v", c.w, c.h, got, c.want)
			}
		})
	}
}

// TestFieldHeights_LengthMatchesNumFields guards the invariant
// layout.go documents but cannot enforce at compile time: fieldHeights
// and numFields are two independent package vars/consts kept in sync
// by convention and comment, not by derivation, because
// computeLayout is pure and runs before any model exists - it never
// has a live fieldSpecs() to count against. If someone adds a field to
// fieldSpecs() and bumps numFields but forgets to grow fieldHeights
// (or vice versa), nothing else in the package would notice the drift.
// This test is the only thing that would.
func TestFieldHeights_LengthMatchesNumFields(t *testing.T) {
	if len(fieldHeights) != numFields {
		t.Errorf("len(fieldHeights) = %d, want %d (numFields); the two are synced by convention, not derivation - see the fieldHeights doc comment in layout.go", len(fieldHeights), numFields)
	}
}

// TestComputeLayout_FieldStackHeightGrowsWithPerFieldHeight is the
// actual point of this refactor, not a redundant restatement of the
// equivalence test above: it proves the fieldHeights/totalFieldsHeight
// seam actually responds to a field being taller than one row, ahead
// of a later phase wiring in a multi-line "Examples" textarea in place
// of one of the current single-line textinputs. Do NOT delete this
// once every fieldHeights entry is back to looking like "all 1s" after
// that phase lands - that's exactly the state this test is written to
// exercise (via a synthetic override, since constructing a real
// multi-line field isn't in scope here).
func TestComputeLayout_FieldStackHeightGrowsWithPerFieldHeight(t *testing.T) {
	original := fieldHeights
	defer func() { fieldHeights = original }()

	// Baseline: all 5 fields at their current height (1 row each).
	base := computeLayout(80, 24)

	// Simulate swapping the last field for a 3-row field (e.g. a short
	// Examples textarea) - the stack should grow by exactly (3-1) = 2
	// extra rows, nothing more and nothing less.
	fieldHeights = []int{1, 1, 1, 1, 3}
	grown := computeLayout(80, 24)

	wantFieldsHeight := base.fieldsHeight + 2
	if grown.fieldsHeight != wantFieldsHeight {
		t.Errorf("fieldsHeight = %d, want %d (base %d + 2 extra rows for the taller field)",
			grown.fieldsHeight, wantFieldsHeight, base.fieldsHeight)
	}

	// At this terminal size contentHeight itself doesn't move (21 rows
	// either way, comfortably above minRequiredContentHeight for both
	// field stacks) - so the 2 extra rows the taller field claims must
	// come out of skillsHeight, confirming the budget actually
	// reallocates rather than e.g. silently double-counting or
	// clamping the difference away.
	wantSkillsHeight := base.skillsHeight - 2
	if grown.skillsHeight != wantSkillsHeight {
		t.Errorf("skillsHeight = %d, want %d (base %d - 2 rows ceded to the taller field)",
			grown.skillsHeight, wantSkillsHeight, base.skillsHeight)
	}

	// minRequiredContentHeight must grow along with the field stack too
	// - otherwise a terminal just tall enough for the old 5-row stack
	// would let the new 7-row stack silently overflow past the terminal
	// height on a small terminal, the same class of bug
	// TestView_TotalHeightNeverExceedsTerminalHeight (view_height_test.go)
	// guards against for the fixed-height case. A 1-row-tall terminal
	// forces contentHeight to hit that floor so this asserts the floor
	// itself, not just contentHeight passing through unclamped.
	tiny := computeLayout(80, 1)
	wantMinRequired := totalFieldsHeight() + targetHeight + minSkillsHeight
	if tiny.contentHeight != wantMinRequired {
		t.Errorf("contentHeight = %d, want %d (minRequiredContentHeight floor, grown field stack)",
			tiny.contentHeight, wantMinRequired)
	}
}
