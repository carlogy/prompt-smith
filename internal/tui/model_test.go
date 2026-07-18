package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/carlogy/prompt-smith/internal/prompt"
	"github.com/carlogy/prompt-smith/internal/registry"
)

// fixtureRegistry mirrors the pattern used by internal/prompt and
// internal/registry: a small, deterministic registry decoupled from the
// real shipped content, with one skill deliberately unsupported on
// "generic" (no Body) to exercise the SupportsTarget filter. A second
// target, "opencode" (SkillMode: reference), lets tests exercise the
// target picker: reference-mode targets support every skill regardless
// of Body (see Registry.SupportsTarget), so "agent-only" - absent on
// "generic" - appears once the target switches to "opencode".
func fixtureRegistry() *registry.Registry {
	return &registry.Registry{
		Categories: []string{"debugging", "testing"},
		Skills: []registry.Skill{
			{ID: "diagnose", Category: "debugging", Order: 10, Body: "diagnose body"},
			{ID: "verify", Category: "testing", Order: 10, Body: "verify body"},
			{ID: "agent-only", Category: "testing", Order: 20}, // no Body
		},
		Targets: map[string]registry.TargetConfig{
			"generic":  {ID: "generic", Delimiter: "xml", SkillMode: "inline"},
			"opencode": {ID: "opencode", Delimiter: "xml", SkillMode: "reference"},
		},
	}
}

func TestNewModel_FiltersGroupsAndStartsOnASelectableItem(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "test goal"})

	var headers, selectable int
	for _, it := range m.items {
		if it.isHeader {
			headers++
		} else {
			selectable++
		}
	}
	if headers != 2 {
		t.Errorf("headers = %d, want 2 (debugging, testing)", headers)
	}
	if selectable != 2 {
		t.Errorf("selectable items = %d, want 2 (agent-only excluded: unsupported on generic)", selectable)
	}
	if m.items[m.cursor].isHeader {
		t.Fatal("cursor started on a header item")
	}
}

func TestModel_CursorNavigationSkipsHeaders(t *testing.T) {
	reg := fixtureRegistry()
	// Goal is non-empty so this starts with skills focused (an empty
	// goal auto-focuses the goal field instead, as of P3c) - this test
	// is purely about cursor navigation, unrelated to that.
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "goal"})

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m2 := updated.(model)
	if m2.items[m2.cursor].isHeader {
		t.Fatal("cursor landed on a header after moving down")
	}
	if m2.cursor <= m.cursor {
		t.Errorf("cursor did not advance: was %d, now %d", m.cursor, m2.cursor)
	}

	updated2, _ := m2.Update(tea.KeyMsg{Type: tea.KeyUp})
	m3 := updated2.(model)
	if m3.cursor != m.cursor {
		t.Errorf("cursor after up = %d, want back to %d", m3.cursor, m.cursor)
	}
}

func TestModel_CursorClampsAtBoundaries(t *testing.T) {
	reg := fixtureRegistry()
	// Same non-empty-goal note as above.
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "goal"})

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m2 := updated.(model)
	if m2.cursor != m.cursor {
		t.Errorf("cursor moved above the first item: was %d, now %d", m.cursor, m2.cursor)
	}

	last := m
	for i := 0; i < 10; i++ {
		u, _ := last.Update(tea.KeyMsg{Type: tea.KeyDown})
		last = u.(model)
	}
	if last.items[last.cursor].isHeader {
		t.Fatal("cursor ended on a header")
	}
	u2, _ := last.Update(tea.KeyMsg{Type: tea.KeyDown})
	last2 := u2.(model)
	if last2.cursor != last.cursor {
		t.Errorf("cursor moved past the last item: was %d, now %d", last.cursor, last2.cursor)
	}
}

func TestModel_ToggleDoesNotMutateThePriorModel(t *testing.T) {
	// Update has a value receiver; m.items is a slice, so a naive
	// in-place mutation would corrupt the model this Update call started
	// from (shared backing array). Guard that regression explicitly.
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "goal"})

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m2 := updated.(model)

	if m.items[m.cursor].selected {
		t.Fatal("toggling on the new model mutated the prior model's items")
	}
	if !m2.items[m2.cursor].selected {
		t.Fatal("expected the new model's item to be selected")
	}
}

func TestModel_ToggleUpdatesSelectionAndPreview(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "test goal"})

	// cursor starts on "diagnose" (first selectable item); toggle it on.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m2 := updated.(model)

	if !m2.items[m2.cursor].selected {
		t.Fatal("expected the item under cursor to become selected")
	}
	if !strings.Contains(m2.preview, "diagnose body") {
		t.Errorf("expected preview to include diagnose's body, got:\n%s", m2.preview)
	}

	// toggle it back off.
	updated2, _ := m2.Update(tea.KeyMsg{Type: tea.KeySpace})
	m3 := updated2.(model)
	if m3.items[m3.cursor].selected {
		t.Fatal("expected the item under cursor to become unselected")
	}
	if strings.Contains(m3.preview, "diagnose body") {
		t.Errorf("expected preview to no longer include diagnose's body, got:\n%s", m3.preview)
	}
}

func TestModel_PreviewReflectsMultipleSelections(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "test goal"})

	u1, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace}) // select diagnose (cursor starts here)
	m1 := u1.(model)
	u2, _ := m1.Update(tea.KeyMsg{Type: tea.KeyDown}) // move to verify
	m2 := u2.(model)
	u3, _ := m2.Update(tea.KeyMsg{Type: tea.KeySpace}) // select verify too
	m3 := u3.(model)

	if !strings.Contains(m3.preview, "diagnose body") {
		t.Errorf("expected preview to include diagnose's body, got:\n%s", m3.preview)
	}
	if !strings.Contains(m3.preview, "verify body") {
		t.Errorf("expected preview to include verify's body, got:\n%s", m3.preview)
	}
}

func TestModel_InitialPreviewReflectsPreselectedSkills(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "test goal", Skills: []string{"diagnose"}})

	if !strings.Contains(m.preview, "diagnose body") {
		t.Errorf("expected initial preview to reflect pre-selected skills, got:\n%s", m.preview)
	}

	found := false
	for _, it := range m.items {
		if !it.isHeader && it.skill.ID == "diagnose" {
			found = it.selected
		}
	}
	if !found {
		t.Fatal("expected diagnose to be pre-selected")
	}
}

func TestModel_EnterConfirmsToStdout(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "goal", Skills: []string{"diagnose"}})

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := updated.(model)

	if m2.result.Action != ActionStdout {
		t.Errorf("Action = %v, want ActionStdout", m2.result.Action)
	}
	if len(m2.result.Inputs.Skills) != 1 || m2.result.Inputs.Skills[0] != "diagnose" {
		t.Errorf("Inputs.Skills = %v, want [diagnose]", m2.result.Inputs.Skills)
	}
	assertQuits(t, cmd)
}

func TestModel_CConfirmsToClipboard(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "goal", Skills: []string{"diagnose"}})

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	m2 := updated.(model)

	if m2.result.Action != ActionCopy {
		t.Errorf("Action = %v, want ActionCopy", m2.result.Action)
	}
	assertQuits(t, cmd)
}

func TestModel_EscCancels(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "goal"})

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m2 := updated.(model)

	if m2.result.Action != ActionCancel {
		t.Errorf("Action = %v, want ActionCancel", m2.result.Action)
	}
	assertQuits(t, cmd)
}

func TestModel_CtrlCCancels(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "goal"})

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m2 := updated.(model)

	if m2.result.Action != ActionCancel {
		t.Errorf("Action = %v, want ActionCancel", m2.result.Action)
	}
	assertQuits(t, cmd)
}

func TestModel_WThenEnterConfirmsWriteWithSuggestedName(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "fix the bug", Skills: []string{"diagnose"}})

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("w")})
	m2 := updated.(model)
	if cmd != nil {
		t.Fatal("expected opening the filename input not to quit")
	}
	if !m2.enteringFilename {
		t.Fatal("expected enteringFilename to be true after pressing w")
	}
	if m2.filenameInput.Value() == "" {
		t.Fatal("expected the filename input to be pre-filled with a suggestion")
	}

	updated2, cmd2 := m2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m3 := updated2.(model)
	if m3.result.Action != ActionWrite {
		t.Errorf("Action = %v, want ActionWrite", m3.result.Action)
	}
	if m3.result.WritePath == "" {
		t.Error("expected WritePath to be set")
	}
	assertQuits(t, cmd2)
}

func TestModel_EscWhileEnteringFilenameReturnsToPicker(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "goal"})

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("w")})
	m2 := updated.(model)

	updated2, cmd := m2.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m3 := updated2.(model)

	if m3.enteringFilename {
		t.Error("expected enteringFilename to be false after esc")
	}
	if cmd != nil {
		t.Error("expected esc while entering a filename not to quit the whole TUI")
	}
}

func assertQuits(t *testing.T, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected a quit command, got nil")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestView_ContainsSkillsPreviewAndFooterHints(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "goal", Skills: []string{"diagnose"}})

	got := stripANSI(m.View())

	for _, want := range []string{
		"diagnose", "verify", // skills listed
		"DEBUGGING", "TESTING", // category headers
		"diagnose body",                              // live preview content
		"enter", "copy", "write", "cancel", "select", // footer hints (skills-focused; pgup/pgdn is preview-only as of P3d)
	} {
		if !strings.Contains(got, want) {
			t.Errorf("View() missing %q, got:\n%s", want, got)
		}
	}
}

func TestView_ExcludesUnsupportedSkill(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "goal"})

	got := stripANSI(m.View())
	if strings.Contains(got, "agent-only") {
		t.Errorf("expected agent-only (unsupported on generic) to be excluded from the view, got:\n%s", got)
	}
}

func TestView_FilenameModeShowsSavePrompt(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "goal"})

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("w")})
	m2 := updated.(model)

	got := stripANSI(m2.View())
	if !strings.Contains(got, "Save") {
		t.Errorf("expected the filename-entry view to mention saving, got:\n%s", got)
	}
	if !strings.Contains(got, m2.filenameInput.Value()) {
		t.Errorf("expected the view to show the current filename input value, got:\n%s", got)
	}
}

func TestModel_WindowSizeMsgUpdatesDimensions(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "goal"})

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m2 := updated.(model)

	if m2.termWidth != 100 || m2.termHeight != 30 {
		t.Errorf("termWidth/termHeight = %d/%d, want 100/30", m2.termWidth, m2.termHeight)
	}
}

func TestView_SkillListScrollsToKeepCursorVisible(t *testing.T) {
	// fixtureRegistry on "generic" produces exactly 4 items:
	// [header:debugging, diagnose, header:testing, verify]
	// (agent-only is excluded: unsupported on generic). Height=11 ->
	// contentHeight=8, minus the fixed 5-row fields section ->
	// skillsHeight=3 -> a 2-row skill list window (skillsHeight minus
	// the "Skills" title line), so this small fixture is enough to
	// force real scrolling without a bigger one.
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "goal"})

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 11})
	m2 := updated.(model)

	got1 := stripANSI(m2.View())
	if !strings.Contains(got1, "diagnose") {
		t.Errorf("expected diagnose visible at the top of a fresh model, got:\n%s", got1)
	}
	if strings.Contains(got1, "verify") {
		t.Errorf("expected verify to be scrolled out of view initially, got:\n%s", got1)
	}

	updated2, _ := m2.Update(tea.KeyMsg{Type: tea.KeyDown}) // cursor -> verify
	m3 := updated2.(model)

	got2 := stripANSI(m3.View())
	if !strings.Contains(got2, "verify") {
		t.Errorf("expected verify to scroll into view once selected, got:\n%s", got2)
	}
	if strings.Contains(got2, "diagnose") {
		t.Errorf("expected diagnose to scroll out of view once verify is selected, got:\n%s", got2)
	}
}

func TestView_FilenamePromptDocumentsSavePathBehavior(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "goal"})

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("w")})
	m2 := updated.(model)

	got := stripANSI(m2.View())
	for _, want := range []string{"current directory", "absolute path"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected the filename prompt to document save-path behavior (%q), got:\n%s", want, got)
		}
	}
}
