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
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"

	"github.com/carlogy/prompt-smith/internal/fielddesc"
)

// fieldDescriptorKey maps a text-field focus zone to its
// internal/fielddesc key, so the footer can show a per-field
// descriptor sentence instead of a generic "type to edit" hint - the
// same sentence the web UI shows under each field (see
// server/page.go's Hints). Target has no entry here: it's in the TUI's
// focus cycle (see focus.go), but it's a left/right selector rather
// than a typed-into text field, so footerHelpFor handles it with its
// own case below instead of via this generic text-field lookup.
// Examples has no entry either, for a different reason: it shares the
// same descriptor sentence (fielddesc.Examples), but its keybind
// suffix can't be the shared "tab next \u00b7 esc unfocus" every other
// text field gets, because Enter behaves differently there (see
// updateExamplesField) - so it needs its own case too.
var fieldDescriptorKey = map[focusZone]string{
	focusGoal:         fielddesc.Goal,
	focusContext:      fielddesc.Context,
	focusConstraints:  fielddesc.Constraints,
	focusRole:         fielddesc.Role,
	focusOutputFormat: fielddesc.OutputFormat,
}

// footerDescriptorFor returns the descriptor-sentence half of the
// footer for the currently-focused zone - what the field is *for*,
// not how to work it (that half comes from bubbles/help via
// keyMap.ShortHelp, keys.go - the two are rendered separately and
// concatenated by viewFooter, never nested into one Style.Render).
// focusSkills/focusPreview return "": neither is a "field" with
// something to describe: their whole footer row is keybinds, exactly
// like before this package adopted bubbles/help. Falls back to a
// generic descriptor for any mapped field whose fielddesc sentence
// somehow comes back empty (defensive; fielddesc's own completeness
// test means this isn't expected to trigger for the fields covered by
// fieldDescriptorKey today).
func footerDescriptorFor(zone focusZone) string {
	switch zone {
	case focusTarget:
		return fielddesc.Sentence(fielddesc.Target)
	case focusExamples:
		return fielddesc.Sentence(fielddesc.Examples)
	default:
		if key, ok := fieldDescriptorKey[zone]; ok {
			if sentence := fielddesc.Sentence(key); sentence != "" {
				return sentence
			}
			return "type to edit"
		}
		return "" // focusSkills/focusPreview: keybinds alone carry the row
	}
}

// viewFooter renders the one-row footer beneath both panes: the
// focused zone's descriptor sentence (footerDescriptorFor), if it has
// one, followed by bubbles/help's rendering of keyMap.ShortHelp() for
// that same zone. The two pieces are rendered SEPARATELY and
// concatenated by plain string "+" rather than one Style.Render call
// wrapping both - help.View already emits its own ANSI escapes (its
// styles are wired up in newModel, from theme.go), and nesting a
// second Style.Render around that output would have footerStyle's
// SGR codes get reset partway through by help's own, mangling the
// line's styling.
func (m model) viewFooter() string {
	desc := footerDescriptorFor(m.focus)

	// help.Model.Width caps ShortHelpView's rendered width so it
	// ellipsizes instead of wrapping onto a second physical row (help's
	// own doc comment; bubbles/help has no soft-wrap of its own) - it
	// does nothing width-aware by default (Width's zero value means
	// "unbounded"). Budget it as the terminal width minus whatever the
	// descriptor above already consumed (plus the separator between
	// them). termWidth is normalized exactly like computeLayout
	// normalizes it internally (defaultTermWidth when <=0, i.e. before
	// the first WindowSizeMsg), so this stays in lockstep with the
	// same effective width the rest of the layout was sized against -
	// not l directly, since l's content widths already have
	// paneHOverhead and the pane-split subtracted out, neither of
	// which applies to the borderless footer row.
	termWidth := m.termWidth
	if termWidth <= 0 {
		termWidth = defaultTermWidth
	}
	sep := ""
	if desc != "" {
		sep = "  "
	}
	m.help.Width = termWidth - lipgloss.Width(desc) - lipgloss.Width(sep)
	if m.help.Width < 0 {
		m.help.Width = 0
	}

	return footerStyle.Render(desc) + sep + m.help.View(m.keys)
}

// viewHelpOverlay renders the full-screen "?" help takeover
// (updatePicker toggles m.help.ShowAll; updateHelpOverlay handles
// dismissing it) - mirrors viewFilenamePrompt's shape: a plain,
// layout-independent screen that replaces the whole split view rather
// than living inside it, listing every binding via
// keyMap.FullHelp() instead of just the focused zone's ShortHelp().
func (m model) viewHelpOverlay() string {
	return "Help\n\n" + m.help.FullHelpView(m.keys.FullHelp()) + "\n\n(press ? or esc to close)"
}

// fieldLabelWidth is padded to the longest label ("Constraints") so
// every field row's input starts at the same column.
const fieldLabelWidth = len("Constraints")

// effectiveFieldLabelWidth returns the label column width viewFields'
// single-line rows (and viewExamplesField) should pad/truncate their
// label to, given availableWidth - the same width-scrollbarWidth
// budget the caller already computed for its own Width().Render call.
// fieldLabelWidth is the width the layout aims for so every field's
// value starts in the same column - but at narrow widths there isn't
// room for markerWidth + fieldLabelWidth + ": " without WRAPPING
// (lipgloss.Style.Width pads short lines but wraps long ones mid-word,
// the same root cause 2ae848a fixed for viewSkillList's labels).
// Padding a truncated label back OUT to fieldLabelWidth via "%-*s"
// would silently undo the truncation, so the padded TARGET width
// itself has to shrink with the available space, not just the string
// - this is that shrinking budget. It's shared with model.go's
// fieldWidth calculation (the WindowSizeMsg handler) so the
// textinput's own Width and the label rendered next to it never drift
// out of sync - if they did, the textinput would be sized against a
// label width that no longer matches what's actually on screen next
// to it.
//
// Reserves minContentWidth columns for the value after the marker,
// label, and ": " are accounted for. Without that reservation, a
// label that exactly exhausts the remaining budget leaves nothing for
// the value; the fieldWidth floor in the WindowSizeMsg handler (which
// guarantees the value area is never zero-width) would then push the
// composed row back OVER budget by exactly that floor, re-wrapping
// the row this function exists to keep from wrapping. Negative or
// zero availableWidth (or a budget too small even for
// minContentWidth) floors the result at 0 - matching truncateToWidth's
// own "no budget" convention - rather than passing a negative width
// into fmt.Sprintf's "%-*s" (harmless there; Go's fmt treats a
// negative width argument as a left-justify flag on the absolute
// value, but meaningless to reason about, so this stays >= 0
// regardless).
func effectiveFieldLabelWidth(availableWidth int) int {
	markerWidth := lipgloss.Width("\u203a ")
	remaining := availableWidth - markerWidth - len(": ") - minContentWidth
	if remaining < 0 {
		remaining = 0
	}
	if remaining > fieldLabelWidth {
		remaining = fieldLabelWidth
	}
	return remaining
}

// View satisfies tea.Model: a split-pane layout (skill picker + fields
// left, live preview right) plus a footer, or a full-screen modal
// prompt (the save-filename prompt when mode is
// promptModeWriteFilename, or the save-as-preset prompt when mode is
// promptModeSavePreset) that replaces the whole thing instead.
func (m model) View() string {
	switch m.mode {
	case promptModeWriteFilename:
		return m.viewFilenamePrompt()
	case promptModeSavePreset:
		return m.viewSavePresetPrompt()
	}
	if m.help.ShowAll {
		return m.viewHelpOverlay()
	}

	l := computeLayout(m.termWidth, m.termHeight)

	left := m.viewTarget(l.leftContentWidth) + "\n" + m.viewSkillList(l.skillsHeight, l.leftContentWidth) + "\n" + m.viewFields(l.leftContentWidth)
	// The left pane holds both skills and every field; only focusPreview
	// puts focus in the right pane instead.
	leftPane, rightPane := renderPanes(left, m.viewPreview(), m.focus != focusPreview)
	body := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
	return lipgloss.JoinVertical(lipgloss.Left, body, m.viewFooter())
}

// renderPanes wraps left and right in the bordered pane style, first
// equalizing their content height so both borders close on the same
// row. lipgloss.JoinHorizontal pads shorter *rendered* blocks with
// blank, borderless filler rather than extending the border - so the
// padding has to happen before the border is applied, not after.
// renderPanes wraps left and right in the bordered pane style, first
// equalizing their content height so both borders close on the same
// row. lipgloss.JoinHorizontal pads shorter *rendered* blocks with
// blank, borderless filler rather than extending the border - so the
// padding has to happen before the border is applied, not after.
// Whichever pane currently holds focus (leftFocused) gets the colored
// focusedPaneStyle border instead of the plain one, so it's visually
// obvious which column focus is in even before reading the \u203a
// marker inside it.
func renderPanes(left, right string, leftFocused bool) (string, string) {
	h := max(lipgloss.Height(left), lipgloss.Height(right))
	leftStyle, rightStyle := paneStyle, paneStyle
	if leftFocused {
		leftStyle = focusedPaneStyle
	} else {
		rightStyle = focusedPaneStyle
	}
	return leftStyle.Height(h).Render(left), rightStyle.Height(h).Render(right)
}

// viewTarget renders the single-line target picker at the top of the
// left pane: "Target: < DisplayName >", using the same fieldLabelWidth-
// padded label style and \u203a-marker convention as viewFields/
// viewSkillList, so it lines up visually with the rows beneath it.
// Deliberately plain ASCII "<"/">" around the name, not the \u2039/\u203a
// angle-quote pair - \u203a is the exact character every other zone's
// focus marker uses (see cursorLineStyle usages below), so rendering it
// unconditionally here would make TestView_ExactlyOneFocusMarkerAcrossAllZones
// see two markers whenever this row and the truly-focused zone both
// render one, or one "stray" marker on this always-visible row when
// some other zone is focused. ASCII brackets still convey "this value
// cycles with the arrow keys" without that collision.
//
// Unlike viewFields' textinputs (which clip their own value internally
// and never wrap), a plain DisplayName has no such built-in horizontal
// scroll; on the left pane's narrow content width (leftPaneFraction=3
// leaves only a handful of columns) a longer target name plus this
// row's label/bracket overhead can exceed the width budget. Capping
// with MaxWidth (which truncates) rather than Width (which word-wraps
// - lipgloss.Style.Render wraps whenever width>0, confirmed via
// TestView_TotalHeightNeverExceedsTerminalHeight going red when this
// used Width instead) keeps this row exactly one line regardless of
// name length, preserving the layout's fixed-row-count invariant.
func (m model) viewTarget(width int) string {
	label := fmt.Sprintf("%-*s", fieldLabelWidth, "Target")
	name := m.reg.Targets[m.target].DisplayName()
	row := fmt.Sprintf("%s: < %s >", label, name)
	if m.focus == focusTarget {
		row = cursorLineStyle.Render("\u203a " + row)
	} else {
		row = "  " + row
	}
	return lipgloss.NewStyle().MaxWidth(width - scrollbarWidth).Render(row)
}

// ellipsis marks a skill label or category header that truncateToWidth
// had to shorten to fit the pane. Same character bubbles/help already
// uses for its own truncation (ShortHelpView), so a squeezed row looks
// consistent with the rest of this UI's narrow-width behavior rather
// than introducing a second "this got cut off" convention.
const ellipsis = "\u2026"

// truncateToWidth truncates s to at most width display columns,
// appending ellipsis when truncation occurs. Measures with
// lipgloss.Width, not len/utf8.RuneCountInString, so multi-byte and
// wide runes are counted correctly - the labels being truncated here
// come from skill IDs, which can be user-supplied (PROMPTSMITH_SKILLS_
// DIR) and carry non-ASCII names - and cuts on rune boundaries via
// range-over-string rather than byte indices, so a truncated name
// never splits a rune in half. A local helper rather than promoting
// github.com/charmbracelet/x/ansi from an indirect to a direct
// dependency (it already ships one, transitively, via lipgloss) -
// this one function is simple enough not to justify that, and keeps
// go.mod's require block unchanged.
//
// width<=0 returns "": there's no budget for even the ellipsis alone,
// so returning empty is the graceful degradation rather than a
// negative-length slice or a panic. When width is too small to fit
// both the truncated text AND the ellipsis, this clips runes directly
// (no ellipsis) rather than one or the other overflowing - the
// degenerate case callers are expected to hit only at extreme widths.
func truncateToWidth(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}

	budget := width
	suffix := ""
	if width > lipgloss.Width(ellipsis) {
		budget = width - lipgloss.Width(ellipsis)
		suffix = ellipsis
	}

	var b strings.Builder
	w := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if w+rw > budget {
			break
		}
		b.WriteRune(r)
		w += rw
	}
	return b.String() + suffix
}

// viewSkillList renders the "Skills" title followed by a windowed
// slice of items (visibleWindow) sized to fit windowHeight content
// rows, scrolling to keep the cursor visible as it moves, with a
// gutter scrollbar in the last column of width. The cursor row is only
// marked with \u203a when skills is the focused zone - otherwise it
// would look active (and up/down would appear broken, since they're
// actually routed elsewhere) even when it isn't focused. A disabled
// row (the current target doesn't support that skill) always renders
// with the "[-]" marker and disabledSkillStyle's faint treatment, cursor
// or not - landing the cursor there (deliberately possible; see
// prevSelectable/nextSelectable) makes the row reachable and readable,
// not "active". Every row's plain text is truncated to fit width
// (truncateToWidth) before any style is applied, so one item always
// occupies exactly one display row - see truncateToWidth's doc comment
// for why that matters to visibleWindow's scroll math.
func (m model) viewSkillList(windowHeight, width int) string {
	// -1: the "Skills" title consumes one row of the pane's content
	// budget, leaving windowHeight-1 rows for the scrollable list.
	listHeight := windowHeight - 1
	visible, offset := visibleWindow(m.items, m.cursor, listHeight)

	// availableRowWidth is the plain-text budget every rendered row
	// (header or skill) must fit within: listBlock below constrains the
	// joined lines to exactly this width. Truncating each row's plain
	// text to this budget up front is what keeps that Width() call from
	// ever needing to WRAP a too-long line - lipgloss.Style.Width pads
	// short lines but wraps long ones mid-word, which is the root cause
	// of the narrow-terminal bug this function exists to fix: once a
	// label wrapped, one item stopped meaning one display row, which is
	// exactly the invariant visibleWindow's scroll math depends on.
	availableRowWidth := width - scrollbarWidth

	lines := make([]string, 0, len(visible))
	for i, it := range visible {
		globalIndex := offset + i
		if it.isHeader {
			header := truncateToWidth(strings.ToUpper(it.category), availableRowWidth)
			lines = append(lines, categoryHeaderStyle.Render(header))
			continue
		}

		mark := "[ ]"
		switch {
		case it.disabled:
			mark = "[-]"
		case it.selected:
			mark = "[x]"
		}

		isCursor := globalIndex == m.cursor && m.focus == focusSkills
		prefix := "  "
		if isCursor {
			prefix = "\u203a "
		}

		// Only the label is ever truncated - never the cursor prefix or
		// the marker. The marker is the only non-color signal that a row
		// is disabled ("[-]"; see disabledSkillStyle's doc comment), so
		// losing it to truncation would defeat the accessibility rule it
		// exists for. labelBudget floors at 0 (not negative) so a pane
		// too narrow even for the marker still degrades gracefully - the
		// final truncateToWidth call below, on the fully composed plain
		// line, is what actually clips that extreme case rather than
		// this one, which by then has already given up its whole budget.
		overhead := lipgloss.Width(prefix) + lipgloss.Width(mark) + 1 // +1: the space between marker and label
		labelBudget := max(availableRowWidth-overhead, 0)
		label := truncateToWidth(it.skill.ID, labelBudget)

		// Compose plain, then truncate, then style - never the other way
		// around: styling injects ANSI escapes, and measuring or slicing
		// an already-styled string corrupts both the width math and the
		// escape sequences themselves.
		plain := truncateToWidth(fmt.Sprintf("%s%s %s", prefix, mark, label), availableRowWidth)

		var line string
		switch {
		case it.disabled:
			line = disabledSkillStyle.Render(plain)
		case isCursor:
			line = cursorLineStyle.Render(plain)
		default:
			line = plain
		}
		lines = append(lines, line)
	}

	// Pad the list block to width-scrollbarWidth so the gutter sits flush
	// against the pane's right edge (kept inside the existing content
	// width, leaving the pane's outer size / border alignment unchanged).
	listBlock := lipgloss.NewStyle().Width(width - scrollbarWidth).Render(strings.Join(lines, "\n"))
	bar := scrollbar(listHeight, len(m.items), listHeight, offset)
	body := lipgloss.JoinHorizontal(lipgloss.Top, listBlock, strings.Join(bar, "\n"))

	return "Skills\n" + body
}

func (m model) viewPreview() string {
	title := fmt.Sprintf("Preview (%s)", m.target)
	if overflowing := !(m.previewVP.AtTop() && m.previewVP.AtBottom()); overflowing {
		title = fmt.Sprintf("%s \u2014 \u2191\u2193 %d%%", title, int(m.previewVP.ScrollPercent()*100))
	}
	if m.focus == focusPreview {
		title = "\u203a " + title
	}

	// Gutter scrollbar in the last column, beside the viewport content
	// (viewport width was already reduced by scrollbarWidth to make room,
	// so the pane's outer width - and border alignment - is unchanged).
	bar := scrollbar(m.previewVP.Height, m.previewVP.TotalLineCount(), m.previewVP.Height, m.previewVP.YOffset)
	body := lipgloss.JoinHorizontal(lipgloss.Top, m.previewVP.View(), strings.Join(bar, "\n"))

	// Single newline, no blank separator line - matches viewSkillList's
	// "Skills\n" pattern so both panes' title overhead is exactly 1 row
	// and previewVP.Height (set to contentHeight-1) covers the rest.
	return previewTitleStyle.Render(title) + "\n" + body
}

// fieldSpec pairs one editable field's label, focus zone, and current
// textinput state, so viewFields can render the five single-line
// fields uniformly. Examples is deliberately NOT one of these: a
// textinput's View() is always exactly one line, which is what lets
// this struct/loop share a "Label: value" row shape across every field
// it holds - a textarea's View() isn't, so Examples gets its own
// render path (viewExamplesField) instead of a slot here.
type fieldSpec struct {
	label string
	zone  focusZone
	input textinput.Model
}

// fieldSpecs lists the five single-line editable fields in their
// canonical (Tab-cycle) order. Examples is the sixth field in that
// same order but isn't included here - see fieldSpec's doc comment.
func (m model) fieldSpecs() []fieldSpec {
	return []fieldSpec{
		{"Goal", focusGoal, m.goalInput},
		{"Context", focusContext, m.contextInput},
		{"Constraints", focusConstraints, m.constraintsInput},
		{"Role", focusRole, m.roleInput},
		{"Output", focusOutputFormat, m.outputFormatInput},
	}
}

// viewFields renders one row per single-line field ("Label: value"),
// padded to width so it aligns with the skill list above it in the
// same pane, with the focused field's row marked (matching the skill
// cursor's and the focused preview title's \u203a convention) -
// followed by the Examples field's own multi-line block
// (viewExamplesField). The two are joined into one string and
// width-constrained together so Examples' textarea (already sized to
// this same width - see examplesFieldWidth) lines up flush with the
// single-line rows above it rather than through a second, independent
// Style.Width call that could round differently.
//
// Each label is truncated (truncateToWidth) to labelWidth - a budget
// that itself shrinks with the pane (effectiveFieldLabelWidth) -
// before the "%-*s" padding, never the value: field VALUE wrapping is
// intentional and the textinput owns it (see this function's earlier
// doc comment further down about horizontal scroll), so truncating
// the whole composed row the way viewSkillList does would break
// behavior that's deliberate here. Only the LABEL half of the row is
// ever this function's business to shorten.
func (m model) viewFields(width int) string {
	availableWidth := width - scrollbarWidth
	labelWidth := effectiveFieldLabelWidth(availableWidth)

	lines := make([]string, 0, numFields-1) // -1: Examples renders as its own block, appended below
	for _, f := range m.fieldSpecs() {
		label := fmt.Sprintf("%-*s", labelWidth, truncateToWidth(f.label, labelWidth))
		row := fmt.Sprintf("%s: %s", label, f.input.View())
		if f.zone == m.focus {
			row = cursorLineStyle.Render("\u203a " + row)
		} else {
			row = "  " + row
		}
		lines = append(lines, row)
	}
	block := strings.Join(lines, "\n") + "\n" + m.viewExamplesField(labelWidth)
	return lipgloss.NewStyle().Width(availableWidth).Render(block)
}

// viewExamplesField renders the Examples field: a label on its own
// line - using the same \u203a marker / cursorLineStyle convention as
// every field above - followed by the textarea's own examplesRows-tall
// rendered block. Every field above packs "Label: value" onto one
// physical line because a textinput's View() is always exactly one
// line; a textarea's View() is a multi-line block that can't share a
// line with a label the same way, which is exactly what
// fieldHeights' examplesFieldHeight entry (1 label row +
// examplesRows) budgets for. The textarea itself is deliberately NOT
// re-wrapped or re-marked line-by-line here: it was already sized to
// this pane's content width via examplesFieldWidth, so rendering its
// View() output as-is is what keeps that width consistent with the
// single-line rows instead of double-applying a width constraint.
//
// labelWidth comes from the caller (viewFields) rather than this
// function recomputing it from fieldLabelWidth directly - it's the
// same shrink-with-the-pane budget every other field's label is
// padded/truncated to (effectiveFieldLabelWidth), so "Examples" lines
// up with "Goal"/"Context"/etc. even once narrow widths have shrunk
// them all below fieldLabelWidth.
func (m model) viewExamplesField(labelWidth int) string {
	label := fmt.Sprintf("%-*s", labelWidth, truncateToWidth("Examples", labelWidth))
	if m.focus == focusExamples {
		label = cursorLineStyle.Render("\u203a " + label)
	} else {
		label = "  " + label
	}
	return label + "\n" + m.examplesInput.View()
}

// viewFilenamePrompt's trailing paragraph documents what writeFile
// (internal/cli/generate.go) actually does with the path it's given -
// both save-path surfaces (this prompt's result.WritePath and --out)
// go through that same function, so the behavior it describes is
// identical either way. It used to claim the opposite of both: that
// "~" isn't expanded and the parent directory must already exist.
// Neither was true - writeFile has always called expandPath (which
// resolves "~"/"~user") and os.MkdirAll before writing - so the
// wording here is corrected to match, mirroring the phrasing
// README.md's --out flag row already uses for the identical behavior
// ("accepts ~/~user; missing parent directories are created") rather
// than inventing a second way to say the same thing.
func (m model) viewFilenamePrompt() string {
	return fmt.Sprintf(
		"Save prompt as:\n%s\n(enter to confirm, esc to cancel)\n\n"+
			"Relative paths save to the current directory (where promptsmith\n"+
			"was run); use an absolute path to save elsewhere. Accepts \"~\"\n"+
			"or \"~user\" for a home directory; missing parent directories\n"+
			"are created automatically.",
		m.filenameInput.View(),
	)
}

// viewSavePresetPrompt renders promptModeSavePreset's modal: the
// name-entry screen normally, or the overwrite-confirm screen instead
// once savePresetConfirm is set (updateSavePresetInput sets it the
// moment a submitted name collides with existingPresets). Mirrors
// viewFilenamePrompt's shape - a plain, layout-independent screen
// replacing the whole split view - for the name-entry half; the
// confirm half is intentionally much shorter, since by that point the
// user has already read the name-entry screen's explanation once and
// only needs the yes/no choice restated.
//
// The name-entry paragraph explains what a preset captures instead of
// documenting a save-path convention the way viewFilenamePrompt's
// trailing paragraph does - there's no path here to explain, and the
// one fact worth stating up front is WHY the input starts empty
// instead of pre-filled the way the filename prompt's is: see
// presetNameInput's doc comment (model.go) and pinned decision #4.
func (m model) viewSavePresetPrompt() string {
	if m.savePresetConfirm {
		return fmt.Sprintf(
			"A preset named %q already exists.\n\n"+
				"(y)es, overwrite it   (n)o, pick a different name",
			m.presetNameInput.Value(),
		)
	}
	return fmt.Sprintf(
		"Save inputs as a preset:\n%s\n(enter to confirm, esc to cancel)\n\n"+
			"A preset captures role, context, constraints, output format,\n"+
			"examples, skills, and target for reuse across future goals -\n"+
			"the goal itself is never saved, since every generation starts\n"+
			"with a fresh one.",
		m.presetNameInput.View(),
	)
}
