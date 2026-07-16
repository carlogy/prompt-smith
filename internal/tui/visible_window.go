package tui

// visibleWindow returns the slice of items visible in a window of the
// given height, plus offset (the index of the first visible item),
// chosen so cursor is always within the visible slice: the window
// scrolls the minimum amount needed to keep cursor in view, in either
// direction. This is a pure, stateless derivation from (items, cursor,
// height) alone - no separate "current scroll position" is stored, so
// it's correct for any cursor value, including jumps, not just
// one-step-at-a-time movement.
func visibleWindow(items []item, cursor, height int) ([]item, int) {
	if height <= 0 || len(items) <= height {
		return items, 0
	}

	maxOffset := len(items) - height
	offset := cursor - height + 1
	if offset < 0 {
		offset = 0
	}
	if offset > maxOffset {
		offset = maxOffset
	}

	return items[offset : offset+height], offset
}
