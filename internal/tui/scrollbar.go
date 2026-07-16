package tui

import "math"

const (
	scrollThumb = "\u2588" // full block: the draggable-looking thumb
	scrollTrack = "\u2502" // light vertical line: the track behind it
	scrollBlank = " "      // shown when content fits (gutter stays a fixed width)

	// scrollbarWidth is the column reserved for the gutter inside a
	// pane's content area. Kept inside the existing content width so the
	// pane's outer size (and border alignment) is unchanged.
	scrollbarWidth = 1
)

// scrollbar renders a vertical scrollbar as trackHeight single-cell rows:
// a thumb (scrollThumb) over a track (scrollTrack), sized and positioned
// to reflect a viewport of visible lines over total lines at the given
// offset. When everything fits (total <= visible) it returns blank rows
// so the gutter column keeps a constant width without drawing a bar.
// trackHeight <= 0 returns nil.
//
// Pure and deterministic: both panes render their bar from this.
func scrollbar(trackHeight, total, visible, offset int) []string {
	if trackHeight <= 0 {
		return nil
	}

	rows := make([]string, trackHeight)

	if total <= visible {
		for i := range rows {
			rows[i] = scrollBlank
		}
		return rows
	}

	thumbSize := int(math.Round(float64(visible) / float64(total) * float64(trackHeight)))
	if thumbSize < 1 {
		thumbSize = 1
	}
	if thumbSize > trackHeight {
		thumbSize = trackHeight
	}

	travel := trackHeight - thumbSize
	maxOffset := total - visible
	thumbPos := int(math.Round(float64(offset) / float64(maxOffset) * float64(travel)))
	if thumbPos < 0 {
		thumbPos = 0
	}
	if thumbPos > travel {
		thumbPos = travel
	}

	for i := range rows {
		if i >= thumbPos && i < thumbPos+thumbSize {
			rows[i] = scrollThumb
		} else {
			rows[i] = scrollTrack
		}
	}
	return rows
}
