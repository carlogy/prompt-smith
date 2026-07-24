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
