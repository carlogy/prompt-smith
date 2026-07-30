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

// Package tui's styling lives here, in one file, on purpose. It used
// to be split across view.go (the pane/cursor/footer styles) and
// highlight.go (the tag-highlight styles), with the same accent color
// hardcoded as a literal in both places. That's exactly the setup
// where a future palette change edits one file, forgets the other,
// and the UI quietly drifts out of sync with itself - which is what
// had already happened here (see activeColor below). Centralizing
// every style in one file makes "change the palette" a one-file diff
// again, and makes it obvious at a glance whether two things are
// meant to share a color or just happen to look the same today.
package tui

import "github.com/charmbracelet/lipgloss"

var (
	// activeColor highlights whatever currently has focus: the cursor
	// line/row (skill cursor, focused field, preview title), the
	// border of whichever pane (left: skills+fields, right: preview)
	// contains it, and the open-tag highlight in the preview
	// (highlightTags). It's an AdaptiveColor rather than a bare
	// lipgloss.Color so the accent stays legible on light-background
	// terminals too - bright-cyan "14" against a light background is
	// close to invisible. Dark MUST stay "14": that's the value this
	// UI has always rendered with on a dark terminal (the common
	// case, and what every existing test's stripANSI(View()) golden
	// output implicitly assumes), so changing it would be a visible
	// regression disguised as a light-mode fix. Light uses "6" (plain
	// cyan), the standard ANSI-16 non-bright counterpart to bright
	// cyan "14" - picked over a truecolor hex value to stay
	// consistent with the rest of the TUI, which only ever uses
	// ANSI-16 codes, and to keep degrading gracefully on terminals
	// with a limited palette.
	activeColor = lipgloss.AdaptiveColor{Light: "6", Dark: "14"}

	categoryHeaderStyle = lipgloss.NewStyle().Bold(true).Underline(true)
	cursorLineStyle     = lipgloss.NewStyle().Bold(true).Foreground(activeColor)
	paneStyle           = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	focusedPaneStyle    = paneStyle.BorderForeground(activeColor)
	footerStyle         = lipgloss.NewStyle().Faint(true)
	previewTitleStyle   = lipgloss.NewStyle().Bold(true)

	// openTagStyle shares activeColor with the cursor/border styles
	// above rather than repeating its own literal, which is the fix
	// for the drift this file exists to prevent in the first place:
	// before this change, this line hardcoded lipgloss.Color("14")
	// independently of activeColor, so the two happened to match by
	// coincidence, not by construction.
	openTagStyle  = lipgloss.NewStyle().Bold(true).Foreground(activeColor)
	closeTagStyle = lipgloss.NewStyle().Faint(true)
)
