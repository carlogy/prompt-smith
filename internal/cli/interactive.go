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

package cli

import (
	"errors"
	"os"

	"golang.org/x/term"
)

// isInteractive reports whether both stdin and stdout are attached to a
// terminal. A package variable so tests can force either branch without
// needing a real TTY.
var isInteractive = func() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

// decideUseTUI applies the interactive-picker gate: launch the TUI when
// running interactively and not explicitly skipped, for any of three
// reasons - no skills were given, no goal was given, or the user
// forced it with --tui (which pre-selects whatever --skills already
// supplied). --quick and --tui together, or --tui outside an
// interactive terminal, are user errors reported eagerly rather than
// silently falling back to a different mode.
func decideUseTUI(interactive, quick, forceTUI bool, numSkills int, goalEmpty bool) (bool, error) {
	if quick && forceTUI {
		return false, errors.New("promptsmith: --tui and --quick are mutually exclusive")
	}
	if forceTUI && !interactive {
		return false, errors.New("promptsmith: --tui requires an interactive terminal")
	}
	if !interactive || quick {
		return false, nil
	}
	return forceTUI || numSkills == 0 || goalEmpty, nil
}
