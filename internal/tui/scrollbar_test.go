package tui

import "testing"

// thumbBounds returns the count of thumb rows and the index of the first
// and last thumb row (-1 if none), asserting the thumb is contiguous.
func thumbBounds(t *testing.T, rows []string) (count, first, last int) {
	t.Helper()
	first, last = -1, -1
	for i, r := range rows {
		if r == scrollThumb {
			if first == -1 {
				first = i
			}
			last = i
			count++
		}
	}
	if first != -1 && count != last-first+1 {
		t.Fatalf("thumb is not contiguous: rows=%v", rows)
	}
	return count, first, last
}

func TestScrollbar_FitsShowsNoThumb(t *testing.T) {
	rows := scrollbar(10, 5, 5, 0) // total == visible: everything fits
	if len(rows) != 10 {
		t.Fatalf("len(rows) = %d, want 10 (gutter column stays a fixed width)", len(rows))
	}
	if n, _, _ := thumbBounds(t, rows); n != 0 {
		t.Errorf("expected no thumb when content fits, found %d thumb rows", n)
	}
}

func TestScrollbar_NonPositiveHeightReturnsNil(t *testing.T) {
	if rows := scrollbar(0, 100, 10, 0); rows != nil {
		t.Errorf("scrollbar with height<=0 = %v, want nil", rows)
	}
}

func TestScrollbar_ThumbAtTopWhenOffsetZero(t *testing.T) {
	rows := scrollbar(10, 20, 5, 0)
	n, first, _ := thumbBounds(t, rows)
	if n == 0 || first != 0 {
		t.Errorf("expected thumb at the top (first row) when offset=0, got first=%d n=%d rows=%v", first, n, rows)
	}
}

func TestScrollbar_ThumbAtBottomWhenOffsetMax(t *testing.T) {
	total, visible, height := 20, 5, 10
	rows := scrollbar(height, total, visible, total-visible) // max offset
	n, _, last := thumbBounds(t, rows)
	if n == 0 || last != height-1 {
		t.Errorf("expected thumb at the bottom (last row) at max offset, got last=%d n=%d rows=%v", last, n, rows)
	}
}

func TestScrollbar_ThumbSizeProportionalAndAtLeastOne(t *testing.T) {
	// Huge content relative to the track: thumb should shrink to the
	// 1-row minimum, never to zero.
	rows := scrollbar(10, 1000, 3, 0)
	if n, _, _ := thumbBounds(t, rows); n != 1 {
		t.Errorf("expected a 1-row thumb for huge content, got %d rows: %v", n, rows)
	}

	// Roughly half visible -> roughly half-height thumb.
	rows = scrollbar(10, 20, 10, 0)
	if n, _, _ := thumbBounds(t, rows); n != 5 {
		t.Errorf("expected a ~5-row thumb for 10/20 visible, got %d: %v", n, rows)
	}
}

func TestScrollbar_ThumbStaysWithinTrackAndKeepsSizeAcrossOffsets(t *testing.T) {
	total, visible, height := 30, 7, 12
	var wantSize int
	for offset := 0; offset <= total-visible; offset++ {
		rows := scrollbar(height, total, visible, offset)
		if len(rows) != height {
			t.Fatalf("offset=%d: len(rows)=%d, want %d", offset, len(rows), height)
		}
		n, first, last := thumbBounds(t, rows)
		if n == 0 {
			t.Fatalf("offset=%d: expected a thumb (content overflows)", offset)
		}
		if first < 0 || last > height-1 {
			t.Errorf("offset=%d: thumb out of track bounds [%d,%d]", offset, first, last)
		}
		if wantSize == 0 {
			wantSize = n
		} else if n != wantSize {
			t.Errorf("offset=%d: thumb size changed to %d, want constant %d", offset, n, wantSize)
		}
	}
}
