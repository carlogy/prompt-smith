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

	// disabledSkillStyle renders a skill row the current target
	// doesn't support (viewSkillList, item.disabled). Faint alone is
	// deliberately NOT the only signal that a row is disabled -
	// viewSkillList also swaps its "[ ]"/"[x]" marker for "[-]" on
	// these rows, mirroring the web UI's own rule that unsupported-ness
	// must never be conveyed by dimming/color alone (a low-contrast
	// terminal palette can render Faint(true) as barely
	// distinguishable from normal text). Reuses footerStyle's
	// Faint(true) rather than a new literal, for the same "advisory/
	// secondary" visual vocabulary hintsBodyStyle already borrows it
	// for.
	disabledSkillStyle = footerStyle

	// openTagStyle shares activeColor with the cursor/border styles
	// above rather than repeating its own literal, which is the fix
	// for the drift this file exists to prevent in the first place:
	// before this change, this line hardcoded lipgloss.Color("14")
	// independently of activeColor, so the two happened to match by
	// coincidence, not by construction.
	openTagStyle  = lipgloss.NewStyle().Bold(true).Foreground(activeColor)
	closeTagStyle = lipgloss.NewStyle().Faint(true)

	// helpKeyStyle/helpDescStyle/helpSeparatorStyle replace
	// bubbles/help's own defaults (help.New(), bubbles@v1.0.0/help/
	// help.go) on the help.Model this package constructs (see
	// newModel). That's NOT working around a non-adaptive default -
	// help.New()'s AdaptiveColor{Light: "#909090", Dark: "#626262"}
	// (etc.) already IS light/dark aware, same mechanism as
	// activeColor above. The reason to override it anyway is
	// activeColor's other half: this TUI deliberately restricts
	// itself to ANSI-16 codes everywhere, never truecolor hex, so it
	// keeps degrading gracefully on terminals with a limited palette
	// and stays visually consistent with the rest of the footer
	// (footerStyle, same Faint(true), no color at all) instead of
	// introducing the one truecolor-hex style in an otherwise ANSI-16
	// UI.
	helpKeyStyle       = lipgloss.NewStyle().Faint(true)
	helpDescStyle      = lipgloss.NewStyle().Faint(true)
	helpSeparatorStyle = lipgloss.NewStyle().Faint(true)

	// errorColor is the preview pane's error banner red, picked the
	// same asymmetric way activeColor picks its cyan above: bright red
	// "9" on a dark terminal (this UI's default rendering target),
	// the plain ANSI-16 counterpart "1" on light, where the bright
	// code would be harder to read against a pale background. Kept as
	// its own color (not reused from elsewhere in this file) because
	// nothing else in this package means "error" - activeColor means
	// "focused", a distinct concept that happens to share no state
	// with this one.
	errorColor = lipgloss.AdaptiveColor{Light: "1", Dark: "9"}
	// errorBannerStyle renders recomputePreview's build-error banner
	// (model.go), replacing the old plain "error: " + err.Error()
	// string that used to go straight into the preview viewport with
	// no styling at all.
	errorBannerStyle = lipgloss.NewStyle().Bold(true).Foreground(errorColor)

	// hintsHeadingStyle/hintsBodyStyle render recomputePreview's
	// advisory promptlint findings block (see hints.go's renderHints),
	// shown above the built prompt on a successful build. hintsBodyStyle
	// reuses footerStyle's Faint(true) treatment rather than a new
	// literal - both mark text as secondary/advisory, never the primary
	// thing on screen - keeping that "advisory" visual vocabulary to
	// one look across the whole package.
	hintsHeadingStyle = lipgloss.NewStyle().Bold(true)
	hintsBodyStyle    = footerStyle

	// noCursorLineStyle neutralizes bubbles/textarea's default
	// CursorLine background tint (textarea.go's DefaultStyles:
	// Background(AdaptiveColor{Light: "255", Dark: "0"})) on the
	// Examples field. That default renders a filled block behind the
	// cursor's line meant to stand out against a textarea's own
	// otherwise-unstyled body text; here it clashes with
	// errorBannerStyle/hintsBodyStyle/highlightTags's own foreground
	// styling one pane over, and with cursorLineStyle's convention of
	// marking focus via foreground+bold rather than a filled
	// background, which every other zone in this package already
	// uses. Assigned to examplesInput.FocusedStyle.CursorLine in
	// newModel - a genuine no-op lipgloss.Style, not a color chosen to
	// blend in, so the cursor line renders exactly like any other line.
	noCursorLineStyle = lipgloss.NewStyle()
)
