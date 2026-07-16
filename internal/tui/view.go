package tui

import (
	"fmt"
	"strings"

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

// View satisfies tea.Model: a split-pane layout (skill picker left,
// live preview right) plus a footer, or the save-filename prompt when
// enteringFilename is true.
func (m model) View() string {
	if m.enteringFilename {
		return m.viewFilenamePrompt()
	}

	l := computeLayout(m.termWidth, m.termHeight)

	leftPane, rightPane := renderPanes(m.viewSkillList(l.contentHeight), m.viewPreview())
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
// rows, scrolling to keep the cursor visible as it moves.
func (m model) viewSkillList(windowHeight int) string {
	// -1: the "Skills" title consumes one row of the pane's content
	// budget, leaving windowHeight-1 rows for the scrollable list.
	listHeight := windowHeight - 1
	visible, offset := visibleWindow(m.items, m.cursor, listHeight)

	lines := make([]string, 0, len(visible)+1)
	lines = append(lines, "Skills")
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
	// strings.Join, not a trailing "\n" after every line: appending "\n"
	// after the last line too would add a phantom empty row that
	// lipgloss.Height counts, inflating this pane 1 line taller than
	// viewPreview (found via the total-height regression test - the two
	// panes must produce exactly matching heights for a given budget).
	return strings.Join(lines, "\n")
}

func (m model) viewPreview() string {
	title := fmt.Sprintf("Preview (%s)", m.target)
	if overflowing := !(m.previewVP.AtTop() && m.previewVP.AtBottom()); overflowing {
		title = fmt.Sprintf("%s \u2014 \u2191\u2193 %d%%", title, int(m.previewVP.ScrollPercent()*100))
	}
	// Single newline, no blank separator line - matches viewSkillList's
	// "Skills\n" pattern so both panes' title overhead is exactly 1 row
	// and previewVP.Height (set to contentHeight-1) covers the rest.
	return previewTitleStyle.Render(title) + "\n" + m.previewVP.View()
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
