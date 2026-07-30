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
	"strings"

	"github.com/carlogy/prompt-smith/internal/prompthl"
)

// highlightTags colorizes each <tag>/</tag> line of raw for display -
// this is a *display-only* transform. Callers must keep using the
// original raw string for anything that gets copied, written, or piped;
// the delivered prompt is never touched by this function's output.
//
// Classification (which lines are tags) is shared with the web UI's
// live preview via internal/prompthl, so both always highlight
// identically and can never drift from each other.
func highlightTags(raw string) string {
	lines := strings.Split(raw, "\n")
	for i, line := range lines {
		switch prompthl.Classify(line) {
		case prompthl.OpenTag:
			lines[i] = openTagStyle.Render(line)
		case prompthl.CloseTag:
			lines[i] = closeTagStyle.Render(line)
		}
	}
	return strings.Join(lines, "\n")
}
