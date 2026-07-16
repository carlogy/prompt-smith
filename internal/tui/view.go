package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

var (
	categoryHeaderStyle = lipgloss.NewStyle().Bold(true).Underline(true)
	cursorLineStyle     = lipgloss.NewStyle().Bold(true)
	paneStyle           = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	footerStyle         = lipgloss.NewStyle().Faint(true)
	previewTitleStyle   = lipgloss.NewStyle().Bold(true)
)

const footerHelp = "\u2191/\u2193 move \u00b7 space select \u00b7 pgup/pgdn scroll preview \u00b7 enter=stdout \u00b7 c=copy \u00b7 w=write \u00b7 esc=cancel"

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

	left := m.viewSkillList(l.skillsHeight, l.leftContentWidth) + "\n" + m.viewFields(l.leftContentWidth)
	leftPane, rightPane := renderPanes(left, m.viewPreview())
	body := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
	return lipgloss.JoinVertical(lipgloss.Left, body, footerStyle.Render(footerHelp))
}

// renderPanes wraps left and right in the bordered pane style, first
// equalizing their content height so both borders close on the same
// row. lipgloss.JoinHorizontal pads shorter *rendered* blocks with
// blank, borderless filler rather than extending the border - so the
// padding has to happen before the border is applied, not after.
func renderPanes(left, right string) (string, string) {
	h := max(lipgloss.Height(left), lipgloss.Height(right))
	return paneStyle.Height(h).Render(left), paneStyle.Height(h).Render(right)
}

// viewSkillList renders the "Skills" title followed by a windowed
// slice of items (visibleWindow) sized to fit windowHeight content
// rows, scrolling to keep the cursor visible as it moves, with a
// gutter scrollbar in the last column of width.
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
		if globalIndex == m.cursor {
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
