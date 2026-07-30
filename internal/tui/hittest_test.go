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

import (
	"testing"

	"github.com/carlogy/prompt-smith/internal/registry"
)

func hitTestItems() []item {
	return []item{
		{isHeader: true, category: "planning"},            // index 0
		{skill: registry.Skill{ID: "s1"}},                 // index 1
		{skill: registry.Skill{ID: "s2"}},                 // index 2
		{skill: registry.Skill{ID: "s3"}, disabled: true}, // index 3
	}
}

func TestItemAtPoint(t *testing.T) {
	items := hitTestItems()
	const (
		leftPaneWidth = 20
		listHeight    = 10
	)

	cases := []struct {
		name      string
		x, y      int
		offset    int
		wantIndex int
		wantOK    bool
	}{
		{"first list row is a header -> reject", 3, listTopOffset, 0, 0, false},
		{"second list row -> s1", 3, listTopOffset + 1, 0, 1, true},
		{"third list row -> s2", 3, listTopOffset + 2, 0, 2, true},
		{"fourth list row is disabled -> reject", 3, listTopOffset + 3, 0, 0, false},
		{"below the last item (blank) -> reject", 3, listTopOffset + 4, 0, 0, false},
		{"top border row -> reject", 3, 0, 0, 0, false},
		{"title row -> reject", 3, 1, 0, 0, false},
		{"click in the right pane (x beyond left pane) -> reject", 25, listTopOffset + 1, 0, 0, false},
		{"negative x -> reject", -1, listTopOffset + 1, 0, 0, false},
		{"with offset, first visible row maps to s1", 3, listTopOffset, 1, 1, true},
		{"row beyond the visible window height -> reject", 3, listTopOffset + listHeight, 0, 0, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			idx, ok := itemAtPoint(tc.x, tc.y, leftPaneWidth, listHeight, tc.offset, items)
			if ok != tc.wantOK {
				t.Fatalf("itemAtPoint ok = %v, want %v", ok, tc.wantOK)
			}
			if ok && idx != tc.wantIndex {
				t.Errorf("itemAtPoint index = %d, want %d", idx, tc.wantIndex)
			}
		})
	}
}
