package tui

// layout is the computed size budget derived from the terminal
// dimensions: how many content columns each pane gets (inside its
// border+padding) and how many content rows both panes get (inside
// their border, after reserving the footer line).
type layout struct {
	leftContentWidth  int
	rightContentWidth int
	contentHeight     int
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
	}
}
