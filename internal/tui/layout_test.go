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
