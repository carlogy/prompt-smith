package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/carlogy/prompt-smith/internal/prompt"
	"github.com/carlogy/prompt-smith/internal/registry"
)

// longBodyRegistry returns a registry with one generic-inline skill
// whose body is exactly the given lines, joined with "\n" - used to
// force real preview overflow deterministically (fixtureRegistry's
// bodies are one-liners, too short to overflow any reasonable window).
func longBodyRegistry(lines ...string) *registry.Registry {
	return &registry.Registry{
		Categories: []string{"debugging"},
		Skills: []registry.Skill{
			{ID: "longskill", Category: "debugging", Body: strings.Join(lines, "\n")},
		},
		Targets: map[string]registry.TargetConfig{
			"generic": {ID: "generic", Delimiter: "xml", SkillMode: "inline"},
		},
	}
}

func TestPreview_PageDownScrollsWhenContentOverflows(t *testing.T) {
	// The rendered preview includes the <task> section before the
	// skill's body, so exactly how many PageDowns reach the bottom
	// depends on layout math this test shouldn't need to hardcode.
	// Assert the qualitative property instead: it starts overflowing
	// and not at the bottom, and enough PageDowns reach the very end.
	reg := longBodyRegistry("line1", "line2", "line3", "line4", "line5", "line6", "line7", "line8")
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g", Skills: []string{"longskill"}})

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 8}) // small viewport
	m2 := updated.(model)

	if m2.previewVP.AtBottom() {
		t.Fatal("expected content to overflow the small viewport (test setup issue)")
	}
	if strings.Contains(stripANSI(m2.View()), "line8") {
		t.Errorf("expected line8 (near the end) not to be visible before scrolling, got:\n%s", stripANSI(m2.View()))
	}

	cur := m2
	for i := 0; i < 10 && !cur.previewVP.AtBottom(); i++ { // generously more than needed
		updated, _ := cur.Update(tea.KeyMsg{Type: tea.KeyPgDown})
		cur = updated.(model)
	}

	got := stripANSI(cur.View())
	if !strings.Contains(got, "line8") {
		t.Errorf("expected line8 visible after paging to the bottom, got:\n%s", got)
	}
}

func TestPreview_MouseWheelScrolls(t *testing.T) {
	reg := longBodyRegistry("line1", "line2", "line3", "line4", "line5", "line6", "line7", "line8")
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g", Skills: []string{"longskill"}})

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 8})
	m2 := updated.(model)

	updated2, _ := m2.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress})
	m3 := updated2.(model)

	if m3.previewVP.YOffset <= m2.previewVP.YOffset {
		t.Errorf("expected mouse wheel down to scroll the preview forward, before=%d after=%d", m2.previewVP.YOffset, m3.previewVP.YOffset)
	}
}

func TestPreview_ScrollIndicatorOnlyWhenContentOverflows(t *testing.T) {
	reg := fixtureRegistry() // diagnose's body is a one-liner
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g", Skills: []string{"diagnose"}})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24}) // plenty of room
	m2 := updated.(model)

	if got := stripANSI(m2.View()); strings.Contains(got, "%") {
		t.Errorf("expected no scroll indicator when content fits, got:\n%s", got)
	}

	longReg := longBodyRegistry(strings.Split(strings.TrimSuffix(strings.Repeat("line\n", 20), "\n"), "\n")...)
	m3 := newModel(longReg, prompt.Inputs{Target: "generic", Goal: "g", Skills: []string{"longskill"}})
	updated2, _ := m3.Update(tea.WindowSizeMsg{Width: 80, Height: 8})
	m4 := updated2.(model)

	if got := stripANSI(m4.View()); !strings.Contains(got, "%") {
		t.Errorf("expected a scroll indicator when content overflows, got:\n%s", got)
	}
}

func TestPreview_ScrollResetsToTopWhenSelectionChanges(t *testing.T) {
	reg := longBodyRegistry("a", "b", "c", "d", "e", "f", "g", "h")
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g", Skills: []string{"longskill"}})

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 8})
	m2 := updated.(model)

	updated2, _ := m2.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	m3 := updated2.(model)
	if m3.previewVP.YOffset == 0 {
		t.Fatal("expected PageDown to move the scroll offset (test setup issue)")
	}

	// The cursor sits on longskill (the only selectable item); toggling
	// it off deselects it, shrinking the preview - a real recompute.
	updated3, _ := m3.Update(tea.KeyMsg{Type: tea.KeySpace})
	m4 := updated3.(model)
	if m4.previewVP.YOffset != 0 {
		t.Errorf("expected scroll to reset to top after the preview recomputed, got YOffset=%d", m4.previewVP.YOffset)
	}
}

func TestPreview_PageDownDoesNotMutateThePriorModelsScrollOffset(t *testing.T) {
	reg := longBodyRegistry("a", "b", "c", "d", "e", "f", "g", "h")
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g", Skills: []string{"longskill"}})

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 8})
	m2 := updated.(model)

	updated2, _ := m2.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	m3 := updated2.(model)

	if m2.previewVP.YOffset != 0 {
		t.Errorf("expected the prior model's scroll offset to stay 0, got %d", m2.previewVP.YOffset)
	}
	if m3.previewVP.YOffset == 0 {
		t.Error("expected the new model's scroll offset to have advanced")
	}
}

func TestPreview_ShowsScrollbarThumbOnlyWhenOverflowing(t *testing.T) {
	overflow := longBodyRegistry("l1", "l2", "l3", "l4", "l5", "l6", "l7", "l8", "l9", "l10")
	m := newModel(overflow, prompt.Inputs{Target: "generic", Goal: "g", Skills: []string{"longskill"}})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 8}) // small -> overflows
	m2 := updated.(model)
	if !strings.Contains(m2.View(), scrollThumb) {
		t.Errorf("expected a scrollbar thumb in the preview when content overflows, got:\n%s", stripANSI(m2.View()))
	}

	fits := fixtureRegistry() // one-line body
	m3 := newModel(fits, prompt.Inputs{Target: "generic", Goal: "g", Skills: []string{"diagnose"}})
	updated2, _ := m3.Update(tea.WindowSizeMsg{Width: 80, Height: 24}) // plenty of room
	m4 := updated2.(model)
	if strings.Contains(m4.View(), scrollThumb) {
		t.Errorf("expected no scrollbar thumb when nothing overflows, got:\n%s", stripANSI(m4.View()))
	}
}
