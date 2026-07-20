package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/carlogy/prompt-smith/internal/prompt"
)

// longLineWords builds a single space-separated line with n words, wide
// enough to overflow any reasonable preview pane width when unwrapped.
func longLineWords(n int) string {
	words := make([]string, n)
	for i := range words {
		words[i] = "word"
	}
	return strings.Join(words, " ")
}

// TestPreview_LongLineWraps proves the bubbles v1 viewport bug is fixed:
// a very long body line must be soft-wrapped to the preview pane's
// width rather than overflowing/getting clipped.
func TestPreview_LongLineWraps(t *testing.T) {
	reg := longBodyRegistry(longLineWords(60)) // one line, 60 words - far wider than any 80-col pane
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g", Skills: []string{"longskill"}})

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m2 := updated.(model)

	lines := strings.Split(stripANSI(m2.previewVP.View()), "\n")
	for _, line := range lines {
		if w := lipgloss.Width(line); w > m2.previewVP.Width {
			t.Errorf("line exceeds viewport width %d (got %d): %q", m2.previewVP.Width, w, line)
		}
	}

	// The long line must have actually wrapped onto more than one
	// physical line - not just been clipped to a single one.
	wordLines := 0
	for _, line := range lines {
		if strings.Contains(line, "word") {
			wordLines++
		}
	}
	if wordLines <= 1 {
		t.Errorf("expected the long line to wrap across multiple physical lines, got %d", wordLines)
	}
}

// TestPreview_ReflowsOnResize proves a resize re-wraps existing content
// to the new width (Change 2): a narrower viewport must produce
// strictly more wrapped lines than a wider one for the same content.
func TestPreview_ReflowsOnResize(t *testing.T) {
	reg := longBodyRegistry(longLineWords(60))
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g", Skills: []string{"longskill"}})

	narrow, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 24})
	mNarrow := narrow.(model)

	wide, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
	mWide := wide.(model)

	if mNarrow.previewVP.TotalLineCount() <= mWide.previewVP.TotalLineCount() {
		t.Errorf("expected narrow resize to produce strictly more wrapped lines than wide resize, narrow=%d wide=%d",
			mNarrow.previewVP.TotalLineCount(), mWide.previewVP.TotalLineCount())
	}
}

// TestPreview_WrapsEachFieldsContent proves that a long value in ANY of
// the five editable fields (Goal, Context, Constraints, Role,
// OutputFormat) reaches the preview pane and is soft-wrapped to the
// viewport width, rather than being clipped or left unwrapped.
func TestPreview_WrapsEachFieldsContent(t *testing.T) {
	long := longLineWords(60)

	cases := []struct {
		name   string
		inputs prompt.Inputs
	}{
		{"Goal", prompt.Inputs{Target: "generic", Goal: long}},
		{"Context", prompt.Inputs{Target: "generic", Goal: "g", Context: long}},
		{"Constraints", prompt.Inputs{Target: "generic", Goal: "g", Constraints: long}},
		{"Role", prompt.Inputs{Target: "generic", Goal: "g", Role: long}},
		{"OutputFormat", prompt.Inputs{Target: "generic", Goal: "g", OutputFormat: long}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reg := fixtureRegistry()
			m := newModel(reg, tc.inputs)

			updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
			m2 := updated.(model)

			lines := strings.Split(stripANSI(m2.previewVP.View()), "\n")
			for _, line := range lines {
				if w := lipgloss.Width(line); w > m2.previewVP.Width {
					t.Errorf("line exceeds viewport width %d (got %d): %q", m2.previewVP.Width, w, line)
				}
			}

			// The long field value must have actually wrapped onto more
			// than one physical line - not just been clipped to a
			// single one (or missing from the preview entirely).
			wordLines := 0
			for _, line := range lines {
				if strings.Contains(line, "word") {
					wordLines++
				}
			}
			if wordLines <= 1 {
				t.Errorf("expected %s's long value to wrap across multiple physical lines, got %d", tc.name, wordLines)
			}
		})
	}
}
