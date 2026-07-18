package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"

	"github.com/carlogy/prompt-smith/internal/fielddesc"
)

var (
	// activeColor highlights whatever currently has focus: the cursor
	// line/row (skill cursor, focused field, preview title) and the
	// border of whichever pane (left: skills+fields, right: preview)
	// contains it. Matches the bright-cyan accent already used for tag
	// highlighting in the preview (P1's highlightTags).
	activeColor = lipgloss.Color("14")

	categoryHeaderStyle = lipgloss.NewStyle().Bold(true).Underline(true)
	cursorLineStyle     = lipgloss.NewStyle().Bold(true).Foreground(activeColor)
	paneStyle           = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	focusedPaneStyle    = paneStyle.BorderForeground(activeColor)
	footerStyle         = lipgloss.NewStyle().Faint(true)
	previewTitleStyle   = lipgloss.NewStyle().Bold(true)
)

// fieldDescriptorKey maps a text-field focus zone to its
// internal/fielddesc key, so the footer can show a per-field
// descriptor sentence instead of a generic "type to edit" hint - the
// same sentence the web UI shows under each field (see
// server/page.go's Hints). Target has no entry here: it's in the TUI's
// focus cycle (see focus.go), but it's a left/right selector rather
// than a typed-into text field, so footerHelpFor handles it with its
// own case below instead of via this generic text-field lookup.
var fieldDescriptorKey = map[focusZone]string{
	focusGoal:         fielddesc.Goal,
	focusContext:      fielddesc.Context,
	focusConstraints:  fielddesc.Constraints,
	focusRole:         fielddesc.Role,
	focusOutputFormat: fielddesc.OutputFormat,
}

// footerHelpFor returns the keybinding hint for the currently-focused
// zone: what up/down (and other zone-specific keys) actually do right
// now, since that's context-dependent - plus the always-available
// confirm/cancel keys where they apply. A text field's hint leads with
// its own descriptor sentence rather than a generic "type to edit":
// every field's editing mechanics are identical (hence the shared
// "tab next / esc unfocus" suffix), but what the field is *for* isn't,
// and that's the more useful thing to show here. Falls back to the
// generic hint for any zone with no mapped descriptor (defensive; not
// expected to trigger for the five current text fields).
func footerHelpFor(zone focusZone) string {
	switch zone {
	case focusSkills:
		return "\u2191/\u2193 move \u00b7 space select \u00b7 tab next \u00b7 enter=stdout \u00b7 c=copy \u00b7 w=write \u00b7 esc=cancel"
	case focusPreview:
		return "\u2191/\u2193 pgup/pgdn scroll \u00b7 tab next \u00b7 enter=stdout \u00b7 c=copy \u00b7 w=write \u00b7 esc=cancel"
	case focusTarget:
		return fielddesc.Sentence(fielddesc.Target) + "  \u00b7  \u2190/\u2192 change \u00b7 tab next \u00b7 esc unfocus"
	default: // a text field
		if key, ok := fieldDescriptorKey[zone]; ok {
			if sentence := fielddesc.Sentence(key); sentence != "" {
				return sentence + "  \u00b7  tab next \u00b7 esc unfocus"
			}
		}
		return "type to edit \u00b7 tab next \u00b7 esc unfocus"
	}
}

// fieldLabelWidth is padded to the longest label ("Constraints") so
// every field row's input starts at the same column.
const fieldLabelWidth = len("Constraints")

// View satisfies tea.Model: a split-pane layout (skill picker + fields
// left, live preview right) plus a footer, or the save-filename prompt
// when enteringFilename is true.
func (m model) View() string {
	if m.enteringFilename {
		return m.viewFilenamePrompt()
	}

	l := computeLayout(m.termWidth, m.termHeight)

	left := m.viewTarget(l.leftContentWidth) + "\n" + m.viewSkillList(l.skillsHeight, l.leftContentWidth) + "\n" + m.viewFields(l.leftContentWidth)
	// The left pane holds both skills and every field; only focusPreview
	// puts focus in the right pane instead.
	leftPane, rightPane := renderPanes(left, m.viewPreview(), m.focus != focusPreview)
	body := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
	return lipgloss.JoinVertical(lipgloss.Left, body, footerStyle.Render(footerHelpFor(m.focus)))
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

// viewSkillList renders the "Skills" title followed by a windowed
// slice of items (visibleWindow) sized to fit windowHeight content
// rows, scrolling to keep the cursor visible as it moves, with a
// gutter scrollbar in the last column of width. The cursor row is only
// marked with \u203a when skills is the focused zone - otherwise it
// would look active (and up/down would appear broken, since they're
// actually routed elsewhere) even when it isn't focused.
func (m model) viewSkillList(windowHeight, width int) string {
	// -1: the "Skills" title consumes one row of the pane's content
	// budget, leaving windowHeight-1 rows for the scrollable list.
	listHeight := windowHeight - 1
	visible, offset := visibleWindow(m.items, m.cursor, listHeight)

	lines := make([]string, 0, len(visible))
	for i, it := range visible {
		globalIndex := offset + i
		if it.isHeader {
			lines = append(lines, categoryHeaderStyle.Render(strings.ToUpper(it.category)))
			continue
		}

		mark := "[ ]"
		if it.selected {
			mark = "[x]"
		}
		line := fmt.Sprintf("%s %s", mark, it.skill.ID)
		if globalIndex == m.cursor && m.focus == focusSkills {
			line = cursorLineStyle.Render("\u203a " + line)
		} else {
			line = "  " + line
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
// textinput state, so viewFields can render all five uniformly.
type fieldSpec struct {
	label string
	zone  focusZone
	input textinput.Model
}

// fieldSpecs lists the five editable fields in their canonical
// (Tab-cycle) order.
func (m model) fieldSpecs() []fieldSpec {
	return []fieldSpec{
		{"Goal", focusGoal, m.goalInput},
		{"Context", focusContext, m.contextInput},
		{"Constraints", focusConstraints, m.constraintsInput},
		{"Role", focusRole, m.roleInput},
		{"Output", focusOutputFormat, m.outputFormatInput},
	}
}

// viewFields renders one row per editable field ("Label: value"),
// padded to width so it aligns with the skill list above it in the
// same pane, with the focused field's row marked (matching the skill
// cursor's and the focused preview title's \u203a convention).
func (m model) viewFields(width int) string {
	lines := make([]string, 0, numFields)
	for _, f := range m.fieldSpecs() {
		label := fmt.Sprintf("%-*s", fieldLabelWidth, f.label)
		row := fmt.Sprintf("%s: %s", label, f.input.View())
		if f.zone == m.focus {
			row = cursorLineStyle.Render("\u203a " + row)
		} else {
			row = "  " + row
		}
		lines = append(lines, row)
	}
	return lipgloss.NewStyle().Width(width - scrollbarWidth).Render(strings.Join(lines, "\n"))
}

func (m model) viewFilenamePrompt() string {
	return fmt.Sprintf(
		"Save prompt as:\n%s\n(enter to confirm, esc to cancel)\n\n"+
			"Relative paths save to the current directory (where promptsmith\n"+
			"was run); use an absolute path to save elsewhere. The parent\n"+
			"directory must already exist; \"~\" is not expanded.",
		m.filenameInput.View(),
	)
}
