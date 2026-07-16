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

const footerHelp = "\u2191/\u2193 move \u00b7 space select \u00b7 enter=stdout \u00b7 c=copy \u00b7 w=write \u00b7 esc=cancel"

// View satisfies tea.Model: a split-pane layout (skill picker left,
// live preview right) plus a footer, or the save-filename prompt when
// enteringFilename is true.
func (m model) View() string {
	if m.enteringFilename {
		return m.viewFilenamePrompt()
	}

	leftPane, rightPane := renderPanes(m.viewSkillList(), m.viewPreview())
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

func (m model) viewSkillList() string {
	var b strings.Builder
	b.WriteString("Skills\n")
	for i, it := range m.items {
		if it.isHeader {
			b.WriteString(categoryHeaderStyle.Render(strings.ToUpper(it.category)))
			b.WriteString("\n")
			continue
		}

		mark := "[ ]"
		if it.selected {
			mark = "[x]"
		}
		line := fmt.Sprintf("%s %s", mark, it.skill.ID)
		if i == m.cursor {
			b.WriteString(cursorLineStyle.Render("\u203a " + line))
		} else {
			b.WriteString("  " + line)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func (m model) viewPreview() string {
	title := previewTitleStyle.Render(fmt.Sprintf("Preview (%s)", m.target))
	if m.previewErr != nil {
		return title + "\n\nerror: " + m.previewErr.Error()
	}
	return title + "\n\n" + highlightTags(m.preview)
}

func (m model) viewFilenamePrompt() string {
	return fmt.Sprintf("Save prompt as:\n%s\n(enter to confirm, esc to cancel)", m.filenameInput.View())
}
