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

package naming

import (
	"testing"
	"time"
)

func TestSuggestFilename(t *testing.T) {
	ts := time.Date(2026, 7, 16, 14, 3, 12, 0, time.UTC)

	cases := []struct {
		name string
		goal string
		want string
	}{
		{
			name: "typical goal",
			goal: "Fix the flaky checkout test",
			want: "promptsmith-20260716T140312Z-fix-the-flaky-checkout-test.txt",
		},
		{
			name: "empty goal falls back to timestamp only",
			goal: "",
			want: "promptsmith-20260716T140312Z.txt",
		},
		{
			name: "whitespace-only goal falls back to timestamp only",
			goal: "   ",
			want: "promptsmith-20260716T140312Z.txt",
		},
		{
			name: "punctuation is sanitized and collapsed",
			goal: "Debug the API's rate-limiter (v2)!!",
			want: "promptsmith-20260716T140312Z-debug-the-api-s-rate-limiter.txt",
		},
		{
			name: "long goal is capped at 6 words",
			goal: "one two three four five six seven eight nine",
			want: "promptsmith-20260716T140312Z-one-two-three-four-five-six.txt",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := SuggestFilename(tc.goal, ts)
			if got != tc.want {
				t.Errorf("SuggestFilename(%q, ts) = %q, want %q", tc.goal, got, tc.want)
			}
		})
	}
}
