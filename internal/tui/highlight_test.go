package tui

import (
	"regexp"
	"strings"
	"testing"
)

func TestHighlightTags_PreservesRawTextSemantically(t *testing.T) {
	// This must hold regardless of whether the test environment's color
	// profile actually emits ANSI codes (lipgloss disables styling
	// entirely on a non-TTY stdout, confirmed separately) - stripping any
	// codes that WERE added must always recover the original bytes
	// exactly, so highlighting can never corrupt what gets copied.
	raw := "<task>\nFix the bug\n</task>\n\n<approach>\nDo the thing\n</approach>"
	got := highlightTags(raw)
	if stripANSI(got) != raw {
		t.Errorf("stripped output != raw input:\ngot:  %q\nwant: %q", stripANSI(got), raw)
	}
}

func TestHighlightTags_BodyLineUntouchedEvenWithStyling(t *testing.T) {
	got := highlightTags("<task>\nFix the bug\n</task>")
	if !strings.Contains(got, "Fix the bug") {
		t.Errorf("expected the literal body line to survive, got:\n%s", got)
	}
}

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}
