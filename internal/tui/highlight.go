package tui

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// lineKind classifies one line of a rendered prompt for highlighting
// purposes.
type lineKind int

const (
	lineBody lineKind = iota
	lineOpenTag
	lineCloseTag
)

var (
	openTagRe  = regexp.MustCompile(`^<[a-z_]+>$`)
	closeTagRe = regexp.MustCompile(`^</[a-z_]+>$`)

	openTagStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	closeTagStyle = lipgloss.NewStyle().Faint(true)
)

// classifyLine reports whether line is an opening tag (<task>), a
// closing tag (</task>), or plain body text.
func classifyLine(line string) lineKind {
	switch {
	case openTagRe.MatchString(line):
		return lineOpenTag
	case closeTagRe.MatchString(line):
		return lineCloseTag
	default:
		return lineBody
	}
}

// highlightTags colorizes each <tag>/</tag> line of raw for display -
// this is a *display-only* transform. Callers must keep using the
// original raw string for anything that gets copied, written, or piped;
// the delivered prompt is never touched by this function's output.
func highlightTags(raw string) string {
	lines := strings.Split(raw, "\n")
	for i, line := range lines {
		switch classifyLine(line) {
		case lineOpenTag:
			lines[i] = openTagStyle.Render(line)
		case lineCloseTag:
			lines[i] = closeTagStyle.Render(line)
		}
	}
	return strings.Join(lines, "\n")
}
