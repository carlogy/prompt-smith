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

func TestModel_EndToEnd_ClickSelectsSkillThenEnterConfirms(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "goal"})

	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))
	// click verify (index 3) at its screen row, then confirm to stdout.
	tm.Send(tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, X: 3, Y: listTopOffset + 3})
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	final := tm.FinalModel(t).(model)
	if final.result.Action != ActionStdout {
		t.Errorf("Action = %v, want ActionStdout", final.result.Action)
	}
	found := false
	for _, s := range final.result.Inputs.Skills {
		if s == "verify" {
			found = true
		}
	}
	if !found {
		t.Errorf("Inputs.Skills = %v, want to include the clicked skill 'verify'", final.result.Inputs.Skills)
	}
}

func TestModel_EndToEnd_TypeGoalScrollPreviewTabBackToSkillsThenConfirm(t *testing.T) {
	// A long body (not a small terminal) forces genuine preview
	// overflow, so scrolling has something real to do regardless of
	// exact layout math.
	longBody := make([]string, 30)
	for i := range longBody {
		longBody[i] = "line"
	}
	reg := longBodyRegistry(longBody...)
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "", Skills: []string{"longskill"}})

	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(90, 20))

	// Empty goal -> focus starts on the goal field; type it directly.
	tm.Type("fix the flaky checkout test")

	// Tab cycle: goal(1) -> context(2) -> constraints(3) -> role(4) ->
	// outputFormat(5) -> preview(6): 5 tabs.
	for i := 0; i < 5; i++ {
		tm.Send(tea.KeyMsg{Type: tea.KeyTab})
	}
	// Preview now focused: Down scrolls it, not a skill cursor.
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})

	// One more tab wraps preview(6) -> skills(0).
	tm.Send(tea.KeyMsg{Type: tea.KeyTab})

	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	final := tm.FinalModel(t).(model)
	if final.focus != focusSkills {
		t.Errorf("focus at confirm time = %v, want focusSkills", final.focus)
	}
	if final.result.Action != ActionStdout {
		t.Errorf("Action = %v, want ActionStdout", final.result.Action)
	}
	if final.result.Inputs.Goal != "fix the flaky checkout test" {
		t.Errorf("Inputs.Goal = %q, want the goal typed into the field", final.result.Inputs.Goal)
	}
	if final.previewVP.YOffset == 0 {
		t.Error("expected the preview to have scrolled while it was focused")
	}
}
