package tui

import "testing"

func TestVisibleWindow_FewerItemsThanHeightShowsAllWithNoOffset(t *testing.T) {
	items := make([]item, 3)
	visible, offset := visibleWindow(items, 1, 10)

	if offset != 0 {
		t.Errorf("offset = %d, want 0", offset)
	}
	if len(visible) != 3 {
		t.Errorf("len(visible) = %d, want 3 (all items, height exceeds count)", len(visible))
	}
}

func TestVisibleWindow_CursorAlwaysWithinTheVisibleSlice(t *testing.T) {
	// This must hold for every cursor position, not just ones reached by
	// stepping one-at-a-time - the formula is stateless and re-derives
	// the window from scratch each call, so it must be correct for any
	// (items, cursor, height) combination, including jumps.
	items := make([]item, 20)
	height := 5

	for cursor := 0; cursor < len(items); cursor++ {
		visible, offset := visibleWindow(items, cursor, height)

		if len(visible) != height {
			t.Fatalf("cursor=%d: len(visible) = %d, want %d", cursor, len(visible), height)
		}
		if cursor < offset || cursor >= offset+len(visible) {
			t.Errorf("cursor=%d not within visible window [%d, %d)", cursor, offset, offset+len(visible))
		}
	}
}

func TestVisibleWindow_LastCursorShowsTheTailNotPastIt(t *testing.T) {
	items := make([]item, 20)
	height := 5

	_, offset := visibleWindow(items, len(items)-1, height)
	wantMaxOffset := len(items) - height
	if offset != wantMaxOffset {
		t.Errorf("offset at last cursor = %d, want %d (show the tail, don't overshoot)", offset, wantMaxOffset)
	}
}

func TestVisibleWindow_ScrollIsMinimalAndSymmetric(t *testing.T) {
	items := make([]item, 20)
	height := 5

	// Crossing the fold by one cursor step should scroll by exactly one
	// line, not jump.
	_, offsetBefore := visibleWindow(items, height-1, height) // last item still in the initial window
	_, offsetAfter := visibleWindow(items, height, height)    // one step further
	if offsetAfter-offsetBefore != 1 {
		t.Errorf("offset moved by %d crossing the fold by one step, want 1", offsetAfter-offsetBefore)
	}

	// Moving back up by one should undo it exactly.
	_, offsetBack := visibleWindow(items, height-1, height)
	if offsetBack != offsetBefore {
		t.Errorf("offset after moving back up = %d, want %d (symmetric)", offsetBack, offsetBefore)
	}
}

func TestVisibleWindow_NonPositiveHeightReturnsAllItemsAsASafeFallback(t *testing.T) {
	items := make([]item, 5)
	visible, offset := visibleWindow(items, 2, 0)
	if len(visible) != 5 || offset != 0 {
		t.Errorf("visibleWindow with height<=0 = (%d items, offset %d), want (5, 0)", len(visible), offset)
	}
}
