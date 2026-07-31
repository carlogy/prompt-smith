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

// promptMode is which full-screen text-entry prompt (if any) is
// currently intercepting input ahead of the normal picker/field
// dispatch in Update - mirrors focusZone's role for the split-pane
// view, but for the modal prompts that replace the whole screen
// instead of sharing it with a zone.
//
// This replaced a single `enteringFilename bool` field: a second bool
// for the save-preset-name prompt would have needed its own parallel
// check at every one of Update's interception points (the tea.KeyMsg
// branch AND the tea.MouseMsg branch both short-circuit on "is some
// modal prompt open", not specifically on the filename one), and that
// pair of checks would keep growing into an if-chain as more prompts
// are added. One enum keeps both interception points a single
// `switch m.mode` (or equivalent one-branch check) no matter how many
// modes exist.
type promptMode int

const (
	// promptModeNone means no modal prompt is open; input flows to the
	// normal picker/field dispatch. Zero value, matching focusZone's
	// zero-value convention (focusSkills) of "the default, no special
	// handling needed" being iota's first value rather than an
	// explicit case.
	promptModeNone promptMode = iota
	// promptModeWriteFilename is the "w" write-to-file filename
	// prompt (updateFilenameInput/viewFilenamePrompt).
	promptModeWriteFilename
	// promptModeSavePreset is the "s" save-as-preset name prompt
	// (updateSavePresetInput/viewSavePresetPrompt), including its
	// overwrite-confirm sub-state. That sub-state is tracked by
	// model.savePresetConfirm rather than a fourth promptMode value:
	// it's a modal-within-a-modal that still intercepts input at
	// exactly the same two Update interception points this enum
	// exists to unify, so it doesn't need its own case there - only
	// updateSavePresetInput itself needs to know which of the two
	// screens it's showing.
	promptModeSavePreset
)
