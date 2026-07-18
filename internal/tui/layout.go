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

	numFields       = 5 // goal, context, constraints, role, output-format
	targetHeight    = 1 // the "Target: < ... >" line at the top of the left pane
	minSkillsHeight = 2 // "Skills" title + at least 1 visible list row
)

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
	// it's too small to fit the fixed fieldsHeight AND the fixed
	// targetHeight AND a minimally-useful skills section, floor it here
	// - not by letting skillsHeight alone go to a degenerate size,
	// which would make viewSkillList's listHeight hit 0 and fall back
	// to showing every item unbounded, silently overflowing the whole
	// layout past the terminal height (the bug this comment is
	// guarding against; found via
	// TestView_TotalHeightNeverExceedsTerminalHeight going red after
	// the fields section was added).
	minRequiredContentHeight := numFields + targetHeight + minSkillsHeight
	if contentHeight < minRequiredContentHeight {
		contentHeight = minRequiredContentHeight
	}

	fieldsHeight := numFields
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
