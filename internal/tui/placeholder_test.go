package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/carlogy/prompt-smith/internal/prompt"
)

func TestView_EmptyFieldsShowPlaceholders(t *testing.T) {
	reg := fixtureRegistry()
	// Goal is non-empty (so focus defaults to skills, keeping this test
	// focused on placeholders); context/constraints/role/output stay
	// empty and should show their hints. A wide terminal keeps the
	// short hints from truncating (they're concise specifically so
	// they usually fit even in a narrower real column; this just avoids
	// the test depending on exact truncation-boundary math).
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "goal"})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 150, Height: 20})
	m2 := updated.(model)

	got := stripANSI(m2.View())
	for _, want := range []string{"relevant background", "must respect", "persona to adopt", "response shape"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected an empty field's placeholder to show %q, got:\n%s", want, got)
		}
	}
}

func TestView_NonEmptyFieldShowsValueNotPlaceholder(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "goal", Context: "some real context value"})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 150, Height: 20})
	m2 := updated.(model)

	got := stripANSI(m2.View())
	if !strings.Contains(got, "some real context value") {
		t.Errorf("expected the actual context value to be visible, got:\n%s", got)
	}
	if strings.Contains(got, "relevant background") {
		t.Errorf("expected a non-empty field to hide its placeholder, got:\n%s", got)
	}
}
