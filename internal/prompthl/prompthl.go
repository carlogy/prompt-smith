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

// Package prompthl classifies lines of a rendered prompt (see
// prompt.Build) for highlighting purposes: every section tag (<task>,
// </task>, ...) is emitted alone on its own line, so a whole-line
// match is enough to tell an opening tag, a closing tag, and
// everything else (plain body text) apart.
//
// Shared by the TUI's live preview (internal/tui) and the web UI's
// live preview (internal/server), so both highlight identically and
// can never drift from each other, or from what prompt.Build actually
// emits. Classification is presentation-only: a content line that
// happens to look like a tag is merely mis-colored by a caller, never
// unsafe - callers are responsible for using the original text for
// anything copied, written, or piped.
package prompthl

import "regexp"

// Kind classifies one line of a rendered prompt.
type Kind int

const (
	Body Kind = iota
	OpenTag
	CloseTag
)

var (
	openTagRe  = regexp.MustCompile(`^<[a-z_]+>$`)
	closeTagRe = regexp.MustCompile(`^</[a-z_]+>$`)
)

// Classify reports whether line is an opening tag (<task>), a closing
// tag (</task>), or plain body text.
func Classify(line string) Kind {
	switch {
	case openTagRe.MatchString(line):
		return OpenTag
	case closeTagRe.MatchString(line):
		return CloseTag
	default:
		return Body
	}
}
