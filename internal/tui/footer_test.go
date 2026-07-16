package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/carlogy/prompt-smith/internal/prompt"
)

func TestFooterHelpFor_ReflectsFocusedZone(t *testing.T) {
	cases := []struct {
		name    string
		zone    focusZone
		want    []string
		notWant []string
	}{
		{
			name:    "skills",
			zone:    focusSkills,
			want:    []string{"move", "select", "enter=stdout"},
			notWant: []string{"type to edit", "unfocus"},
		},
		{
			name:    "a text field (goal)",
			zone:    focusGoal,
			want:    []string{"type", "esc"},
			notWant: []string{"space select", "enter=stdout", "pgup/pgdn"},
		},
		{
			name:    "preview",
			zone:    focusPreview,
			want:    []string{"scroll", "enter=stdout"},
			notWant: []string{"space select", "type to edit"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := footerHelpFor(tc.zone)
			for _, want := range tc.want {
				if !strings.Contains(got, want) {
					t.Errorf("footerHelpFor(%v) missing %q, got: %q", tc.zone, want, got)
				}
			}
			for _, notWant := range tc.notWant {
				if strings.Contains(got, notWant) {
					t.Errorf("footerHelpFor(%v) should NOT contain %q, got: %q", tc.zone, notWant, got)
				}
			}
		})
	}
}

func TestFooterHelpFor_EveryTextFieldGetsTheSameHint(t *testing.T) {
	// All five fields share identical editing mechanics, so they should
	// share the same footer text - this locks that they don't drift
	// independently as fields were added one at a time in P3c.
	want := footerHelpFor(focusGoal)
	for _, z := range []focusZone{focusContext, focusConstraints, focusRole, focusOutputFormat} {
		if got := footerHelpFor(z); got != want {
			t.Errorf("footerHelpFor(%v) = %q, want the same as focusGoal's %q", z, got, want)
		}
	}
}

func TestView_FooterChangesWithFocus(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "goal"})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 90, Height: 20})
	m2 := updated.(model) // focus=skills (goal already non-empty)

	got1 := stripANSI(m2.View())
	if !strings.Contains(got1, "space select") {
		t.Errorf("expected the skills-focused footer to mention space select, got:\n%s", got1)
	}

	u, _ := m2.Update(tea.KeyMsg{Type: tea.KeyTab}) // -> goal field
	m3 := u.(model)
	got2 := stripANSI(m3.View())
	if strings.Contains(got2, "space select") {
		t.Errorf("expected the field-focused footer NOT to mention space select, got:\n%s", got2)
	}
}
