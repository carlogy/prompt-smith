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

package prompthl

import "testing"

func TestClassify(t *testing.T) {
	cases := []struct {
		line string
		want Kind
	}{
		{"<task>", OpenTag},
		{"</task>", CloseTag},
		{"<output_format>", OpenTag},
		{"</output_format>", CloseTag},
		{"Fix the bug", Body},
		{"", Body},
		{"<not a valid tag", Body},
		{"find: glob", Body},
		{"Load the `diagnose` skill:", Body},
		{"<bad-tag>", Body}, // hyphen isn't in [a-z_] - the builder never emits one anyway
		{"< task>", Body},   // a space breaks the match
		{"<TASK>", Body},    // uppercase breaks the match - the builder only emits lowercase tags
		{"<task", Body},     // missing closing >
		{"task>", Body},     // missing opening <
	}

	for _, tc := range cases {
		if got := Classify(tc.line); got != tc.want {
			t.Errorf("Classify(%q) = %v, want %v", tc.line, got, tc.want)
		}
	}
}
