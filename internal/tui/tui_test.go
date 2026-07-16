package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/carlogy/prompt-smith/internal/prompt"
)

// TestModel_EndToEnd drives the model through a real Bubble Tea program
// loop (teatest simulates a terminal and dispatches through the actual
// Init/Update/View cycle) rather than calling Update directly, so it
// exercises the same message-handling path a real terminal session
// would. Run itself (a thin tea.NewProgram(...).Run() wrapper reading
// real stdin/stdout) isn't independently exercised here, matching how
// other thin entrypoints in this codebase (e.g. cli.Execute) aren't
// unit tested directly either - the behavior lives in model, which is.
func TestModel_EndToEnd_ToggleSkillThenEnterConfirmsStdout(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "goal"})

	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))

	tm.Send(tea.KeyMsg{Type: tea.KeySpace}) // select diagnose (cursor starts here)
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter}) // confirm to stdout

	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	final := tm.FinalModel(t).(model)
	if final.result.Action != ActionStdout {
		t.Errorf("Action = %v, want ActionStdout", final.result.Action)
	}
	if len(final.result.Inputs.Skills) != 1 || final.result.Inputs.Skills[0] != "diagnose" {
		t.Errorf("Inputs.Skills = %v, want [diagnose]", final.result.Inputs.Skills)
	}
}

func TestModel_EndToEnd_WThenTypeThenEnterConfirmsWrite(t *testing.T) {
	// The pre-filled suggestion leaves the cursor at the end (bubbles'
	// textinput has no "select all" concept), so typing appends rather
	// than replaces - exactly like a real terminal. Simulate a user
	// clearing the suggested name with backspace before typing their
	// own, and confirm the final value is exactly what they typed, with
	// no leftover prefix from the suggestion.
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "goal", Skills: []string{"diagnose"}})

	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("w")})
	for i := 0; i < 80; i++ { // comfortably longer than any suggested name
		tm.Send(tea.KeyMsg{Type: tea.KeyBackspace})
	}
	tm.Type("custom-name.txt") // teatest helper: sends one KeyMsg per rune
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	final := tm.FinalModel(t).(model)
	if final.result.Action != ActionWrite {
		t.Errorf("Action = %v, want ActionWrite", final.result.Action)
	}
	if final.result.WritePath != "custom-name.txt" {
		t.Errorf("WritePath = %q, want %q", final.result.WritePath, "custom-name.txt")
	}
}

func TestModel_EndToEnd_EscCancelsWithoutTypingAnything(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "goal"})

	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))
	tm.Send(tea.KeyMsg{Type: tea.KeyEsc})
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	final := tm.FinalModel(t).(model)
	if final.result.Action != ActionCancel {
		t.Errorf("Action = %v, want ActionCancel", final.result.Action)
	}
}
