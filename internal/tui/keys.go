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

import "github.com/charmbracelet/bubbles/key"

// keyMap is the single source of truth for every NAMED key this TUI
// recognizes (Tab, Enter, arrows, etc.) - used both to dispatch input
// via key.Matches (model.go) and, since it implements help.KeyMap
// (ShortHelp/FullHelp below), to render the footer and the "?"
// overlay (view.go). It deliberately does NOT cover "c"/"w"/"?": all
// three are tea.KeyRunes, and bubbletea's Key.String() renders a
// KeyRunes message as the runes themselves - a generic KeyRunes
// catch-all can't be expressed as a single key.Matches binding, so
// those three stay plain string comparisons in updatePicker's
// `case tea.KeyRunes:` branch, matching how "c"/"w" already worked
// there before this file existed. Copy/Write/Help below exist purely
// so the footer/overlay can still show them - they're never passed to
// key.Matches.
//
// zone is which focusZone ShortHelp should describe right now
// (skills/preview/target/examples/a generic text field each show
// different keys - see footerHelpFor's now-removed history in
// view.go) - kept in sync with model.focus by changeFocus, the one
// place focus ever changes. Matching (key.Matches calls elsewhere)
// never depends on zone; only these two rendering methods do.
type keyMap struct {
	zone focusZone

	Up       key.Binding
	Down     key.Binding
	Left     key.Binding
	Right    key.Binding
	Tab      key.Binding
	ShiftTab key.Binding
	Space    key.Binding
	Enter    key.Binding
	Esc      key.Binding
	PgUp     key.Binding
	PgDown   key.Binding
	CtrlC    key.Binding

	// Copy/Write/Save/Help are display-only (see the doc comment
	// above): their Keys() are never fed to key.Matches, only their
	// Help() text is ever read, by ShortHelp/FullHelp below. "s" is
	// dispatched the same way "c"/"w" are - a plain tea.KeyRunes
	// comparison in updatePicker's `case tea.KeyRunes:` branch (see
	// updateSavePresetInput) - for the same reason given above: it's a
	// single bare rune, not expressible as a key.Matches binding
	// alongside a generic KeyRunes catch-all.
	Copy  key.Binding
	Write key.Binding
	Save  key.Binding
	Help  key.Binding
}

// newKeyMap builds the keyMap with every binding's real keys and a
// zone-agnostic default Help text (used verbatim by FullHelp, and as
// the base ShortHelp composes zone-specific phrasing from via
// withLabel). zone starts at its zero value, focusSkills, matching
// model's own default focus; changeFocus keeps it current from there.
func newKeyMap() keyMap {
	return keyMap{
		Up:       key.NewBinding(key.WithKeys("up"), key.WithHelp("\u2191", "up")),
		Down:     key.NewBinding(key.WithKeys("down"), key.WithHelp("\u2193", "down")),
		Left:     key.NewBinding(key.WithKeys("left"), key.WithHelp("\u2190", "prev target")),
		Right:    key.NewBinding(key.WithKeys("right"), key.WithHelp("\u2192", "next target")),
		Tab:      key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next field")),
		ShiftTab: key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "prev field")),
		// KeySpace stringifies to a literal " " (bubbletea's
		// keyNames[KeySpace]), so the binding must match on " ", not
		// "space" - key.WithHelp is independent of that and free to
		// display whatever reads best.
		Space:  key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "select")),
		Enter:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm")),
		Esc:    key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
		PgUp:   key.NewBinding(key.WithKeys("pgup"), key.WithHelp("pgup", "page up")),
		PgDown: key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("pgdn", "page down")),
		CtrlC:  key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "cancel")),
		Copy:   key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "copy")),
		Write:  key.NewBinding(key.WithKeys("w"), key.WithHelp("w", "write")),
		Save:   key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "save")),
		Help:   key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	}
}

// withLabel returns a copy of b with different help text, keeping its
// underlying keys - the same physical key means something different
// depending on the focused zone (Esc cancels the whole picker from
// focusSkills/focusPreview but merely unfocuses a text field
// elsewhere; Up/Down move the skill cursor or scroll the preview), and
// help.KeyMap only ever reads a binding's Help() text, never its
// Keys(), so this is enough to get zone-appropriate phrasing without
// keeping N near-duplicate key.Binding fields per phrasing.
func withLabel(b key.Binding, keyLabel, desc string) key.Binding {
	return key.NewBinding(key.WithKeys(b.Keys()...), key.WithHelp(keyLabel, desc))
}

// ShortHelp satisfies help.KeyMap: the one-row footer's keybind half,
// switched on k.zone so it only ever advertises what's actually
// available right now - the entire reason the footer used to switch
// on focus (see footerHelpFor's history in view.go) still applies,
// just expressed through bubbles/help instead of a hand-built string.
func (k keyMap) ShortHelp() []key.Binding {
	switch k.zone {
	case focusSkills:
		// bubbles/help's ShortHelpView (vendored at
		// bubbles@v1.0.0/help/help.go) walks this slice in the exact
		// order it's returned and, the moment the next entry would
		// overflow m.help.Width, appends "…" and stops - it never
		// wraps to a second line (help.KeyMap's own doc comment: "The
		// help bubble will render help in the order in which the help
		// items are returned here"). That still makes ORDER the
		// truncation-priority signal for everything in this slice -
		// EXCEPT "esc cancel", which is deliberately pinned LAST
		// despite being the entry a stuck user most needs, because
		// convention says the exit key goes at the end of a footer,
		// not the front. Putting the conventionally-last entry in the
		// position ellipsis eats first only works because this row is
		// kept within the 80-column footer's standard width in the
		// first place - "esc cancel" surviving is a width-budget
		// guarantee here, not a priority-order one:
		//
		//   1. space select - the core mechanic of this zone; without
		//                     it the picker does nothing.
		//   2. ↑/↓ move     - needed to reach the skill you want to
		//                     select in the first place.
		//   3. enter ok     - submits/confirms; a real action, but one
		//                     step removed from picking skills.
		//   4. c/w copy/write - fast paths that duplicate what "ok"
		//                     already reaches via the preview.
		//   5. s save       - newest and least-reached-for of the real
		//                     actions; would be the first casualty if
		//                     cancel weren't fixed at the end.
		//   6. tab next     - second-to-last on purpose: Tab only
		//                     moves focus between panes that are
		//                     already visible and mouse-clickable, and
		//                     "?" documents it in full (FullHelp
		//                     below) - so of everything ranked ABOVE
		//                     cancel, this is the correct next
		//                     casualty. (Its full "next" desc is what
		//                     fixed the stray "tab  ·" double space a
		//                     prior phase introduced by shipping Tab
		//                     with an empty desc instead.)
		//   7. esc cancel   - LAST by convention, not by priority -
		//                     see above.
		//
		// Because cancel's survival now depends on the row fitting in
		// 80 columns rather than on its position, this row's width is
		// a real, finite budget again: measured at exactly 78 columns
		// with help.Model.ShortSeparator tightened from bubbles/
		// help's default " • " (3 cols) to "  " (2 cols) - see
		// newModel (model.go) - which is what buys back the ~6 columns
		// six gaps cost and is the only reason "tab next" AND "s save"
		// AND "esc cancel" all fit together at 80 without dropping
		// anything. There is ~2 columns of slack left at 80,
		// not "no budget to defend" - a future addition here needs
		// either a trade (shortening/folding an existing entry) or
		// another separator/label saving, the same kind of trade this
		// row needed before, not a free append past the 7th slot.
		// TestShortHelp_PriorityOrderSurvivesTruncation checks widths
		// {40, 60, 80, 100, 120} for exactly this reason: 80 is no
		// longer an arbitrary sample point, it's the width the whole
		// row is now tuned to survive intact.
		//
		// PgUp/PgDn (which also page the skill list, via updatePicker)
		// still isn't listed at all here, by the same reasoning taken
		// one step further - FullHelp's PgUp entry documents it where
		// there's no truncation to worry about.
		return []key.Binding{
			k.Space,
			withLabel(k.Up, "\u2191/\u2193", "move"),
			withLabel(k.Enter, "enter", "ok"),
			key.NewBinding(key.WithKeys("c", "w"), key.WithHelp("c/w", "copy/write")),
			k.Save,
			withLabel(k.Tab, "tab", "next"),
			withLabel(k.Esc, "esc", "cancel"),
		}
	case focusPreview:
		// Same reasoning as focusSkills above (including "esc cancel"
		// pinned last by convention, and this row's own 80-column
		// budget under the tightened ShortSeparator - see that
		// comment for the full explanation), mirrored for this zone's
		// own actions (scrolling stands in for move/select as the
		// "core mechanic" here) - kept deliberately parallel rather
		// than re-deriving a different ranking, since nothing about
		// this zone's keys is more or less important relative to each
		// other than the skills-zone equivalents are. c/w folds into
		// one entry the same way it does above (was two separate
		// bindings; nothing pinned that split, and folding it buys
		// room this zone's row needed for "s save", which nothing
		// pinned against either - see the task note that this zone's
		// tests assert nothing about s/save/copy/write specifically).
		// Measures shorter than focusSkills' row (76 vs 78 columns),
		// so it has a little more slack, but is deliberately NOT
		// treated as having a separate, looser budget - keeping both
		// rows built the same way is worth more than spending that
		// slack on something skills' row can't afford.
		return []key.Binding{
			withLabel(k.Up, "\u2191/\u2193 pgup/pgdn", "scroll"),
			withLabel(k.Enter, "enter", "ok"),
			key.NewBinding(key.WithKeys("c", "w"), key.WithHelp("c/w", "copy/write")),
			k.Save,
			withLabel(k.Tab, "tab", "next"),
			withLabel(k.Esc, "esc", "cancel"),
		}
	case focusTarget:
		return []key.Binding{
			withLabel(k.Left, "\u2190/\u2192", "change"),
			withLabel(k.Tab, "tab", "next"),
			withLabel(k.Esc, "esc", "unfocus"),
		}
	case focusExamples:
		// Deliberately spells out that Enter inserts a newline here
		// instead of doing nothing special, and that submitting
		// requires tabbing out first (see updateExamplesField in
		// model.go) - a user tabbing in expecting Enter to behave like
		// it does in every other field would otherwise have no way to
		// discover the difference. Don't collapse this back into the
		// generic default case below; that's the bug this exists to
		// prevent.
		return []key.Binding{
			withLabel(k.Enter, "enter", "insert newline"),
			withLabel(k.Tab, "tab", "next field (then enter to submit)"),
			withLabel(k.Esc, "esc", "unfocus"),
		}
	default: // a generic single-line text field
		return []key.Binding{
			withLabel(k.Tab, "tab", "next"),
			withLabel(k.Esc, "esc", "unfocus"),
		}
	}
}

// FullHelp satisfies help.KeyMap: the "?" overlay's exhaustive
// reference, grouped by category - unlike ShortHelp this is NOT
// zone-dependent, since the overlay's whole point is showing every key
// this TUI recognizes at once, not just what's reachable right now.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{
			withLabel(k.Up, "\u2191/\u2193", "move skill cursor / scroll preview"),
			withLabel(k.Left, "\u2190/\u2192", "change target"),
			k.Tab,
			k.ShiftTab,
		},
		{
			k.Space,
			withLabel(k.Enter, "enter", "confirm (stdout) / insert newline in examples"),
			k.Copy,
			k.Write,
			withLabel(k.Save, "s", "save as preset"),
		},
		{
			withLabel(k.PgUp, "pgup/pgdn", "page skill list / preview"),
			withLabel(k.Esc, "esc", "cancel / unfocus"),
			k.CtrlC,
			k.Help,
		},
	}
}
