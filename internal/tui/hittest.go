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

// listTopOffset is how many screen rows precede the first skill-list
// item within the left pane: the top border (1) + the "Target: ..."
// line (1) + the "Skills" title (1). paneStyle uses zero vertical
// padding, so nothing else intervenes.
const listTopOffset = 3

// itemAtPoint maps a mouse click at screen coordinates (x, y) to a global
// index into items, returning ok=true only when the click lands on a
// selectable (non-header) item inside the visible skill-list window.
//
// Geometry: the left pane spans screen columns [0, leftPaneWidth); the
// list's first row is at y == listTopOffset, and each subsequent row is
// one item further from the window's offset. Clicks on the border, the
// title, the blank area past the last item, the right pane, or a
// category header all return ok=false.
func itemAtPoint(x, y, leftPaneWidth, listHeight, offset int, items []item) (int, bool) {
	if x < 0 || x >= leftPaneWidth {
		return 0, false
	}

	listRow := y - listTopOffset
	if listRow < 0 || listRow >= listHeight {
		return 0, false
	}

	globalIndex := offset + listRow
	if globalIndex < 0 || globalIndex >= len(items) {
		return 0, false
	}
	if items[globalIndex].isHeader {
		return 0, false
	}
	return globalIndex, true
}
