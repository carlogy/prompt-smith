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

// TestComputeLayout_FieldHeightSumMatchesOldCountBasedMath pins
// computeLayout's concrete output for a spread of terminal sizes. It
// started life (Phase 0) as an equivalence check that the
// numFields->totalFieldsHeight() refactor was behavior-preserving
// while every fieldHeights entry was still 1 - hence the name and the
// "hand-computed with the old formula" framing below, both kept
// because the numbers still ARE what the old bare-numFields formula
// would have produced for the five single-line fields; fieldsHeight
// is simply no longer equal to numFields now that Examples (the sixth
// field, examplesFieldHeight=4 rows) has landed. This is exactly the
// moment its own original doc comment predicted and explicitly
// permitted going red - "If this test ever goes red while fieldHeights
// is untouched, the refactor changed observable behavior" (it's NOT
// untouched now: this is Phase 1, not a refactor of Phase 0). Updating
// the expectations here, rather than deleting the test, keeps it doing
// its real job: catching any future accidental change to
// computeLayout's arithmetic.
//
// The spread of sizes deliberately includes both zero-value fallback
// (0x0) and a genuinely tiny-but-nonzero terminal (1x1), plus a size
// that lands exactly one row under minRequiredContentHeight before the
// floor kicks in (80x10) - the cases most likely to expose an
// off-by-one in the sum/count math.
func TestComputeLayout_FieldHeightSumMatchesOldCountBasedMath(t *testing.T) {
	cases := []struct {
		name string
		w, h int
		want layout
	}{
		{
			name: "0x0 falls back to the 80x24 default",
			w:    0, h: 0,
			want: layout{leftContentWidth: 22, rightContentWidth: 50, contentHeight: 21, fieldsHeight: 9, skillsHeight: 11},
		},
		{
			name: "1x1 degenerate: every dimension floors to its minimum",
			w:    1, h: 1,
			want: layout{leftContentWidth: 1, rightContentWidth: 1, contentHeight: 12, fieldsHeight: 9, skillsHeight: 2},
		},
		{
			name: "80x24, the common default terminal, via the non-fallback path",
			w:    80, h: 24,
			want: layout{leftContentWidth: 22, rightContentWidth: 50, contentHeight: 21, fieldsHeight: 9, skillsHeight: 11},
		},
		{
			name: "200x50, comfortably large",
			w:    200, h: 50,
			want: layout{leftContentWidth: 62, rightContentWidth: 130, contentHeight: 47, fieldsHeight: 9, skillsHeight: 37},
		},
		{
			name: "10x4, small width and height both floor",
			w:    10, h: 4,
			want: layout{leftContentWidth: 1, rightContentWidth: 3, contentHeight: 12, fieldsHeight: 9, skillsHeight: 2},
		},
		{
			name: "80x10, one row under minRequiredContentHeight before flooring",
			w:    80, h: 10,
			want: layout{leftContentWidth: 22, rightContentWidth: 50, contentHeight: 12, fieldsHeight: 9, skillsHeight: 2},
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
// actual point of the fieldHeights/totalFieldsHeight refactor, not a
// redundant restatement of the equivalence test above: it proves the
// seam actually responds to a field being taller than one row.
// Originally (Phase 0) this simulated a hypothetical multi-line
// Examples field via a synthetic override, since the real one hadn't
// landed yet; now that it has (fieldHeights' real last entry is
// examplesFieldHeight, 4 rows), the override below simulates a
// hypothetically even-taller Examples field instead, to prove the seam
// generalizes beyond the one height that happens to be shipped today -
// the same point, just with the baseline now real instead of
// hypothetical. Do NOT delete this once the shipped numbers stop
// looking novel - that's exactly the state this test is written to
// exercise.
func TestComputeLayout_FieldStackHeightGrowsWithPerFieldHeight(t *testing.T) {
	original := fieldHeights
	defer func() { fieldHeights = original }()

	// Baseline: the real, shipped field stack - five 1-row fields plus
	// the 4-row Examples field.
	base := computeLayout(80, 24)

	// Simulate a hypothetically even-taller Examples field (7 rows
	// instead of 4) - the stack should grow by exactly the delta (3),
	// nothing more and nothing less.
	const grownExamplesRows = 7
	const delta = grownExamplesRows - examplesFieldHeight
	fieldHeights = []int{1, 1, 1, 1, 1, grownExamplesRows}
	grown := computeLayout(80, 24)

	wantFieldsHeight := base.fieldsHeight + delta
	if grown.fieldsHeight != wantFieldsHeight {
		t.Errorf("fieldsHeight = %d, want %d (base %d + %d extra rows for the taller field)",
			grown.fieldsHeight, wantFieldsHeight, base.fieldsHeight, delta)
	}

	// At this terminal size contentHeight itself doesn't move (21 rows
	// either way, comfortably above minRequiredContentHeight for both
	// field stacks) - so the extra rows the taller field claims must
	// come out of skillsHeight, confirming the budget actually
	// reallocates rather than e.g. silently double-counting or
	// clamping the difference away.
	wantSkillsHeight := base.skillsHeight - delta
	if grown.skillsHeight != wantSkillsHeight {
		t.Errorf("skillsHeight = %d, want %d (base %d - %d rows ceded to the taller field)",
			grown.skillsHeight, wantSkillsHeight, base.skillsHeight, delta)
	}

	// minRequiredContentHeight must grow along with the field stack too
	// - otherwise a terminal just tall enough for the old stack would
	// let the new, taller stack silently overflow past the terminal
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
