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
	"regexp"
	"strings"
	"testing"
)

func TestHighlightTags_PreservesRawTextSemantically(t *testing.T) {
	// This must hold regardless of whether the test environment's color
	// profile actually emits ANSI codes (lipgloss disables styling
	// entirely on a non-TTY stdout, confirmed separately) - stripping any
	// codes that WERE added must always recover the original bytes
	// exactly, so highlighting can never corrupt what gets copied.
	raw := "<task>\nFix the bug\n</task>\n\n<approach>\nDo the thing\n</approach>"
	got := highlightTags(raw)
	if stripANSI(got) != raw {
		t.Errorf("stripped output != raw input:\ngot:  %q\nwant: %q", stripANSI(got), raw)
	}
}

func TestHighlightTags_BodyLineUntouchedEvenWithStyling(t *testing.T) {
	got := highlightTags("<task>\nFix the bug\n</task>")
	if !strings.Contains(got, "Fix the bug") {
		t.Errorf("expected the literal body line to survive, got:\n%s", got)
	}
}

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}
