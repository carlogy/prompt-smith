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

import "github.com/carlogy/prompt-smith/internal/prompt"

// Action is what the user chose to do with the finished prompt.
type Action int

const (
	ActionCancel Action = iota
	ActionStdout
	ActionCopy
	ActionWrite
)

// Result is what Run returns: the finalized inputs plus what to do with
// the assembled prompt. Run never performs the action itself (no file
// writes, no clipboard) - the caller (internal/cli) does, reusing the
// same delivery logic as the non-interactive path.
type Result struct {
	Inputs    prompt.Inputs
	Action    Action
	WritePath string // set when Action == ActionWrite
}
