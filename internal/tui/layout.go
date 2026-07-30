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

// layout is the computed size budget derived from the terminal
// dimensions: how many content columns each pane gets (inside its
// border+padding) and how many content rows both panes get (inside
// their border, after reserving the footer line). The left pane's
// content rows split further between the target line, the skill list,
// and the fields section (targetHeight + skillsHeight + fieldsHeight
// == contentHeight).
type layout struct {
	leftContentWidth  int
	rightContentWidth int
	contentHeight     int
	fieldsHeight      int
	skillsHeight      int
}

const (
	footerHeight   = 1
	paneBorderRows = 2 // top + bottom border, one pane's outer height overhead
	paneHOverhead  = 4 // left+right border (2) + left+right padding (2)

	defaultTermWidth  = 80
	defaultTermHeight = 24

	minContentWidth  = 1
	minContentHeight = 1

	leftPaneFraction = 3 // left pane gets ~1/leftPaneFraction of the width

	numFields       = 6 // goal, context, constraints, role, output-format, examples; must equal len(fieldHeights) below - NOT all one row each, see fieldHeights
	targetHeight    = 1 // the "Target: < ... >" line at the top of the left pane
	minSkillsHeight = 2 // "Skills" title + at least 1 visible list row

	// examplesRows is the fixed height, in rows, of the Examples
	// field's textarea body - deliberately NOT content-dependent
	// (unlike, say, letting it grow with the number of lines typed),
	// so the layout budget stays deterministic and the preview pane
	// doesn't jump around while the user is mid-edit. Longer content
	// scrolls inside these 3 rows via the textarea's own viewport
	// instead of growing them.
	examplesRows = 3

	// examplesFieldHeight is the Examples field's total rendered
	// height: one row for its own label line, plus examplesRows for
	// the textarea body beneath it. It needs its own label line
	// (rather than sharing one line with its value, like every other
	// field's "Label: value" row) because a textarea's View() is a
	// multi-line block, not a single line a label could be prefixed
	// onto - see viewExamplesField in view.go.
	examplesFieldHeight = 1 + examplesRows
)

// fieldHeights lists each editable field's rendered height, in
// terminal rows, in the same order fieldSpecs (view.go) renders the
// first five, with the sixth (Examples) appended after them - see
// viewFields. The first five entries are 1: a single-line textinput is
// always exactly one row. The last is examplesFieldHeight (4): the
// Examples field is this package's first field taller than one row -
// a multi-line textarea rendering its own label-plus-body block - which
// is exactly why this list exists as a sum instead of a bare count in
// the first place (see totalFieldsHeight below).
//
// Deliberately a package var, not derived from fieldSpecs(): fieldSpecs
// is a method on model, built from live textinput.Model state, but
// computeLayout runs before any model exists - it only ever sees
// terminal dimensions - and needs this same budget from field
// *structure* alone. So, like fieldDescriptorKey vs. fieldSpecs itself
// in view.go, this list is kept in sync with numFields/fieldSpecs by
// convention and comment, not by derivation; len(fieldHeights) must
// equal numFields. Being an ordinary var (Go has no const slice
// literals) also makes it the seam a test can substitute a synthetic
// height into without constructing a whole model - see
// TestComputeLayout_FieldStackHeightGrowsWithPerFieldHeight in
// layout_test.go, which exists specifically to prove that seam works
// - originally written ahead of the Examples textarea landing, now
// exercising the real thing via a synthetic override instead of a
// hypothetical one.
var fieldHeights = []int{1, 1, 1, 1, 1, examplesFieldHeight}

// totalFieldsHeight sums fieldHeights: the vertical budget the field
// stack needs. With every entry above equal to 1 this is numerically
// identical to numFields, but expressing it as a sum instead of a bare
// count is the point of this indirection - numFields conflated "how
// many fields" with "how tall is the stack", which breaks down the
// moment one field is taller than one row.
func totalFieldsHeight() int {
	total := 0
	for _, h := range fieldHeights {
		total += h
	}
	return total
}

// computeLayout derives the pane content sizes from the terminal
// dimensions reported by tea.WindowSizeMsg. Zero (before the first
// message arrives) or unreasonably small dimensions fall back to a
// usable default/minimum rather than producing a degenerate size.
func computeLayout(termWidth, termHeight int) layout {
	if termWidth <= 0 {
		termWidth = defaultTermWidth
	}
	if termHeight <= 0 {
		termHeight = defaultTermHeight
	}

	contentHeight := termHeight - footerHeight - paneBorderRows
	if contentHeight < minContentHeight {
		contentHeight = minContentHeight
	}
	// contentHeight is the shared left/right pane height (both panes
	// always render to exactly this many rows - viewPreview by
	// construction via previewVP.Height, and the left pane because
	// targetHeight+skillsHeight+fieldsHeight sums back to it below). If
	// it's too small to fit the field stack's height budget AND the
	// fixed targetHeight AND a minimally-useful skills section, floor
	// it here - not by letting skillsHeight alone go to a degenerate
	// size, which would make viewSkillList's listHeight hit 0 and fall
	// back to showing every item unbounded, silently overflowing the
	// whole layout past the terminal height (the bug this comment is
	// guarding against; found via
	// TestView_TotalHeightNeverExceedsTerminalHeight going red after
	// the fields section was added). totalFieldsHeight() replaces a
	// bare numFields here - the field stack's budget is its fields'
	// summed *height*, not how many of them there are, and those two
	// stopped being interchangeable the moment a field could be taller
	// than one row.
	minRequiredContentHeight := totalFieldsHeight() + targetHeight + minSkillsHeight
	if contentHeight < minRequiredContentHeight {
		contentHeight = minRequiredContentHeight
	}

	fieldsHeight := totalFieldsHeight()
	skillsHeight := contentHeight - fieldsHeight - targetHeight

	leftOuterWidth := termWidth / leftPaneFraction
	rightOuterWidth := termWidth - leftOuterWidth

	leftContentWidth := leftOuterWidth - paneHOverhead
	if leftContentWidth < minContentWidth {
		leftContentWidth = minContentWidth
	}
	rightContentWidth := rightOuterWidth - paneHOverhead
	if rightContentWidth < minContentWidth {
		rightContentWidth = minContentWidth
	}

	return layout{
		leftContentWidth:  leftContentWidth,
		rightContentWidth: rightContentWidth,
		contentHeight:     contentHeight,
		fieldsHeight:      fieldsHeight,
		skillsHeight:      skillsHeight,
	}
}
