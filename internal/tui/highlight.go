package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/carlogy/prompt-smith/internal/prompthl"
)

var (
	openTagStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	closeTagStyle = lipgloss.NewStyle().Faint(true)
)

// highlightTags colorizes each <tag>/</tag> line of raw for display -
// this is a *display-only* transform. Callers must keep using the
// original raw string for anything that gets copied, written, or piped;
// the delivered prompt is never touched by this function's output.
//
// Classification (which lines are tags) is shared with the web UI's
// live preview via internal/prompthl, so both always highlight
// identically and can never drift from each other.
func highlightTags(raw string) string {
	lines := strings.Split(raw, "\n")
	for i, line := range lines {
		switch prompthl.Classify(line) {
		case prompthl.OpenTag:
			lines[i] = openTagStyle.Render(line)
		case prompthl.CloseTag:
			lines[i] = closeTagStyle.Render(line)
		}
	}
	return strings.Join(lines, "\n")
}
