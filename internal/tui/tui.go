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
	tea "github.com/charmbracelet/bubbletea"

	"github.com/carlogy/prompt-smith/internal/prompt"
	"github.com/carlogy/prompt-smith/internal/registry"
)

// Options configures Run.
type Options struct {
	// ExistingPresets is the bare names (no ".yaml", no directory) of
	// presets already on disk, as reported by the caller (internal/
	// preset's listing, via internal/cli) - see model.existingPresets's
	// doc comment for why this is a plain name list rather than a
	// callback or a filesystem handle into internal/preset. Zero value
	// (nil) means "no presets exist yet", which is exactly correct for
	// a caller with nothing on disk to report.
	ExistingPresets []string
	// NoHints mirrors the CLI's --no-hints flag: it suppresses this
	// picker's promptlint findings (see recomputePreview in model.go)
	// the same way --no-hints suppresses warnLintFindings's stderr
	// output in internal/cli/generate.go, and the same way
	// server.Options.NoHints suppresses the web UI's findings. Without
	// this, --tui would give --no-hints one meaning on the command
	// line and a silent no-op under the picker - the same flag would
	// stop meaning "I don't want hints" depending on which mode it's
	// combined with. Zero value (false) means hints are shown,
	// matching today's behavior before this field existed.
	NoHints bool
}

// Run launches the interactive skill picker + live preview and returns
// the user's finalized choice. initial seeds the goal and any optional
// fields already supplied via flags/args, plus any skills already
// selected (e.g. via --tui with --skills, which pre-checks them).
// opts.ExistingPresets is the bare names (no ".yaml", no directory) of
// presets already on disk, as reported by the caller (internal/preset's
// listing, via internal/cli) - see model.existingPresets's doc comment
// for why this is a plain name list rather than a callback or a
// filesystem handle into internal/preset.
//
// Run never performs the chosen action itself (no file writes, no
// clipboard) - the caller applies Result the same way it would flag-only
// input, so delivery logic is never duplicated between the two paths.
//
// Mouse cell-motion is enabled for preview wheel-scrolling; this
// captures mouse events while the TUI is open, which disables the
// terminal's native click-drag text selection until it exits (the
// footer's "c=copy" action covers copying the whole prompt instead).
func Run(reg *registry.Registry, initial prompt.Inputs, opts Options) (Result, error) {
	m := newModel(reg, initial)
	m.existingPresets = opts.ExistingPresets
	m.noHints = opts.NoHints
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	finalModel, err := p.Run()
	if err != nil {
		return Result{}, err
	}
	return finalModel.(model).result, nil
}
