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

	"github.com/carlogy/prompt-smith/internal/promptlint"
)

// renderHints renders promptlint's advisory findings as a styled block
// - a "Suggestions" heading (hintsHeadingStyle) followed by one bullet
// per finding (hintsBodyStyle) - for recomputePreview to prepend above
// the built prompt. Returns "" for an empty findings slice, so a
// well-formed prompt (or a build error, which never reaches this
// call - see recomputePreview) renders no block at all rather than a
// stray empty heading.
//
// Unlike the web UI's preview.html (which lists every finding on its
// own <li>, unconditionally) or the CLI's warnLintFindings (which
// collapses the three pure-absence findings into one sentence to fit
// a single stderr line), this renders every finding on its own line
// without collapsing: the preview pane, like the web UI's, has the
// room, and unlike the CLI it's not fighting for space against other
// stderr output.
func renderHints(findings []promptlint.Finding) string {
	if len(findings) == 0 {
		return ""
	}
	lines := make([]string, 0, len(findings)+1)
	lines = append(lines, hintsHeadingStyle.Render("Suggestions"))
	for _, f := range findings {
		lines = append(lines, hintsBodyStyle.Render("- "+f.Message))
	}
	return strings.Join(lines, "\n")
}
