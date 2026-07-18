package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/carlogy/prompt-smith/internal/fielddesc"
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
			want:    []string{"What you want the model to do.", "esc"},
			notWant: []string{"space select", "enter=stdout", "pgup/pgdn"},
		},
		{
			name:    "preview",
			zone:    focusPreview,
			want:    []string{"scroll", "enter=stdout"},
			notWant: []string{"space select", "type to edit"},
		},
		{
			name:    "target",
			zone:    focusTarget,
			want:    []string{fielddesc.Sentence(fielddesc.Target), "change", "esc"},
			notWant: []string{"space select", "enter=stdout", "pgup/pgdn"},
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

// TestFooterHelpFor_TextFieldsShareKeybindsButShowTheirOwnDescriptor
// replaces the old "every field gets the same hint" assumption: that
// was true when the hint was purely mechanical ("type to edit"), but
// since each field now leads with its own fielddesc sentence (Commit
// 7), sameness there would be a bug, not a feature. What must still
// hold - because the five fields *do* share identical editing
// mechanics - is the keybind suffix.
func TestFooterHelpFor_TextFieldsShareKeybindsButShowTheirOwnDescriptor(t *testing.T) {
	const keybindSuffix = "tab next \u00b7 esc unfocus"

	fields := []struct {
		zone focusZone
		key  string
	}{
		{focusGoal, fielddesc.Goal},
		{focusContext, fielddesc.Context},
		{focusConstraints, fielddesc.Constraints},
		{focusRole, fielddesc.Role},
		{focusOutputFormat, fielddesc.OutputFormat},
	}

	seen := make(map[string]bool, len(fields))
	for _, f := range fields {
		got := footerHelpFor(f.zone)
		if !strings.Contains(got, keybindSuffix) {
			t.Errorf("footerHelpFor(%v) = %q, want it to end with the shared keybind suffix %q", f.zone, got, keybindSuffix)
		}
		wantSentence := fielddesc.Sentence(f.key)
		if !strings.Contains(got, wantSentence) {
			t.Errorf("footerHelpFor(%v) = %q, want it to lead with %q", f.zone, got, wantSentence)
		}
		if seen[got] {
			t.Errorf("footerHelpFor(%v) = %q, want a distinct descriptor per field, got a duplicate", f.zone, got)
		}
		seen[got] = true
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
