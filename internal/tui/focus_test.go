package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/carlogy/prompt-smith/internal/prompt"
)

func TestFocus_StartsOnSkills(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g"})

	if m.focus != focusSkills {
		t.Errorf("initial focus = %v, want focusSkills", m.focus)
	}
}

func TestFocus_TabCyclesForwardWithWraparound(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g"})

	want := []focusZone{
		focusGoal, focusContext, focusConstraints, focusRole, focusOutputFormat,
		focusPreview, focusTarget, focusSkills, // wraps back around
	}
	cur := m
	for i, w := range want {
		updated, _ := cur.Update(tea.KeyMsg{Type: tea.KeyTab})
		cur = updated.(model)
		if cur.focus != w {
			t.Errorf("after %d Tab(s): focus = %v, want %v", i+1, cur.focus, w)
		}
	}
}

func TestFocus_ShiftTabCyclesBackwardWithWraparound(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g"})

	want := []focusZone{
		focusTarget, focusPreview, focusOutputFormat, focusRole, focusConstraints, focusContext,
		focusGoal, focusSkills, // wraps back around
	}
	cur := m
	for i, w := range want {
		updated, _ := cur.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
		cur = updated.(model)
		if cur.focus != w {
			t.Errorf("after %d Shift+Tab(s): focus = %v, want %v", i+1, cur.focus, w)
		}
	}
}

func TestFocus_TabDoesNotMutateThePriorModel(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g"})

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	_ = updated.(model)

	if m.focus != focusSkills {
		t.Errorf("prior model's focus changed to %v, want it to stay focusSkills", m.focus)
	}
}

func TestFocus_SkillsFocusedUpDownStillMoveCursor(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g"})
	if m.focus != focusSkills {
		t.Fatal("expected default focus on skills")
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m2 := updated.(model)
	if m2.cursor == m.cursor {
		t.Error("expected Down to move the skill cursor when skills is focused")
	}
}

func TestFocus_PreviewFocusedUpDownScrollPreview(t *testing.T) {
	reg := longBodyRegistry("a", "b", "c", "d", "e", "f", "g", "h")
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g", Skills: []string{"longskill"}})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 8})
	m2 := updated.(model)

	// Tab order: skills -> goal -> context -> constraints -> role ->
	// outputFormat -> preview = 6 tabs to reach the preview.
	cur := m2
	for i := 0; i < 6; i++ {
		u, _ := cur.Update(tea.KeyMsg{Type: tea.KeyTab})
		cur = u.(model)
	}
	if cur.focus != focusPreview {
		t.Fatalf("expected focus on preview after 6 tabs, got %v", cur.focus)
	}

	before := cur.previewVP.YOffset
	updated2, _ := cur.Update(tea.KeyMsg{Type: tea.KeyDown})
	m3 := updated2.(model)
	if m3.previewVP.YOffset <= before {
		t.Errorf("expected Down to scroll the preview forward when it's focused, before=%d after=%d", before, m3.previewVP.YOffset)
	}

	updated3, _ := m3.Update(tea.KeyMsg{Type: tea.KeyUp})
	m4 := updated3.(model)
	if m4.previewVP.YOffset >= m3.previewVP.YOffset {
		t.Errorf("expected Up to scroll the preview backward when it's focused, before=%d after=%d", m3.previewVP.YOffset, m4.previewVP.YOffset)
	}
}

func TestFocus_TypingInGoalFieldUpdatesGoalAndPreview(t *testing.T) {
	// Goal starts empty, so focus is already on the goal field by
	// default (P3c's bare-invocation behavior, pinned separately by
	// TestNewModel_EmptyGoalStartsWithGoalFocused) - no initial Tab
	// needed to reach it.
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "", Skills: []string{"diagnose"}})
	if m.focus != focusGoal {
		t.Fatalf("expected focus on goal by default with an empty goal, got %v", m.focus)
	}

	cur := m
	for _, r := range "hello" {
		u, _ := cur.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		cur = u.(model)
	}

	if cur.goal != "hello" {
		t.Errorf("goal = %q, want %q", cur.goal, "hello")
	}
	if !strings.Contains(cur.preview, "hello") {
		t.Errorf("expected the live preview to reflect the typed goal, got:\n%s", cur.preview)
	}
}

func TestFocus_SpaceDoesNothingWhenPreviewFocused(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g"})

	cur := m
	for i := 0; i < 6; i++ { // skills -> ... -> preview
		u, _ := cur.Update(tea.KeyMsg{Type: tea.KeyTab})
		cur = u.(model)
	}
	if cur.focus != focusPreview {
		t.Fatalf("expected preview focus after 6 tabs, got %v", cur.focus)
	}

	updated, _ := cur.Update(tea.KeyMsg{Type: tea.KeySpace})
	m2 := updated.(model)
	if m2.items[cur.cursor].selected {
		t.Error("expected Space to do nothing (not toggle a skill) while the preview is focused")
	}
}

func TestFocus_EscBlursGoalFieldToSkillsWithoutCanceling(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g"})
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab}) // -> goal
	m2 := updated.(model)

	updated2, cmd := m2.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m3 := updated2.(model)

	if m3.focus != focusSkills {
		t.Errorf("focus after Esc in the goal field = %v, want focusSkills", m3.focus)
	}
	// Note: m3.result.Action isn't a meaningful check here - ActionCancel
	// is the enum's zero value, so it'd read "true" even if nothing had
	// happened. The cmd check below (no tea.QuitMsg) is what actually
	// proves Esc didn't trigger a full cancel.
	if cmd != nil {
		msg := cmd()
		if _, isQuit := msg.(tea.QuitMsg); isQuit {
			t.Error("expected Esc in a text field not to quit the program")
		}
	}
}

func TestFocus_ActionKeysFireFromPreviewFocusToo(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g", Skills: []string{"diagnose"}})

	cur := m
	for i := 0; i < 6; i++ {
		u, _ := cur.Update(tea.KeyMsg{Type: tea.KeyTab})
		cur = u.(model)
	}
	if cur.focus != focusPreview {
		t.Fatalf("expected preview focus after 6 tabs, got %v", cur.focus)
	}

	updated, cmd := cur.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := updated.(model)
	if m2.result.Action != ActionStdout {
		t.Errorf("Action = %v, want ActionStdout (enter should confirm from preview focus too)", m2.result.Action)
	}
	if cmd == nil {
		t.Fatal("expected Enter from preview focus to quit")
	}
}

func TestFocus_TypingInEachRemainingFieldUpdatesItAndPreview(t *testing.T) {
	cases := []struct {
		name      string
		tabs      int // tabs from focusSkills to reach this field
		wantZone  focusZone
		readValue func(m model) string
	}{
		{"context", 2, focusContext, func(m model) string { return m.context }},
		{"constraints", 3, focusConstraints, func(m model) string { return m.constraints }},
		{"role", 4, focusRole, func(m model) string { return m.role }},
		{"outputFormat", 5, focusOutputFormat, func(m model) string { return m.outputFormat }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reg := fixtureRegistry()
			m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "goal", Skills: []string{"diagnose"}})

			cur := m
			for i := 0; i < tc.tabs; i++ {
				u, _ := cur.Update(tea.KeyMsg{Type: tea.KeyTab})
				cur = u.(model)
			}
			if cur.focus != tc.wantZone {
				t.Fatalf("after %d tabs, focus = %v, want %v", tc.tabs, cur.focus, tc.wantZone)
			}

			for _, r := range "xyz" {
				u, _ := cur.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
				cur = u.(model)
			}

			if got := tc.readValue(cur); got != "xyz" {
				t.Errorf("%s value = %q, want %q", tc.name, got, "xyz")
			}
			if !strings.Contains(cur.preview, "xyz") {
				t.Errorf("expected preview to reflect typed %s, got:\n%s", tc.name, cur.preview)
			}
		})
	}
}

func TestFocus_ChangingFocusBlursThePreviousField(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g"})

	u1, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab}) // -> goal
	m1 := u1.(model)
	if !m1.goalInput.Focused() {
		t.Fatal("expected goalInput to be focused after tabbing to it")
	}

	u2, _ := m1.Update(tea.KeyMsg{Type: tea.KeyTab}) // -> context
	m2 := u2.(model)
	if m2.goalInput.Focused() {
		t.Error("expected goalInput to be blurred after tabbing away from it")
	}
	if !m2.contextInput.Focused() {
		t.Error("expected contextInput to be focused after tabbing to it")
	}
}

func TestNewModel_EmptyGoalStartsWithGoalFocused(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: ""})

	if m.focus != focusGoal {
		t.Errorf("focus = %v, want focusGoal when launched with an empty goal", m.focus)
	}
	if !m.goalInput.Focused() {
		t.Error("expected goalInput to be focused when launched with an empty goal")
	}
}

func TestNewModel_NonEmptyGoalStartsWithSkillsFocused(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "already have a goal"})

	if m.focus != focusSkills {
		t.Errorf("focus = %v, want focusSkills when a goal was already provided", m.focus)
	}
}

func TestFocus_ConfirmActionsBlockedWhenGoalEmpty(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: ""}) // focus=focusGoal, goal empty

	// Blur back to skills so we're testing action-key gating, not typing.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m2 := updated.(model)
	if m2.focus != focusSkills {
		t.Fatalf("expected focusSkills after Esc, got %v", m2.focus)
	}

	updated2, cmd := m2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m3 := updated2.(model)
	if cmd != nil {
		t.Error("expected Enter to be a no-op (not quit) when goal is empty")
	}
	if m3.result.Action == ActionStdout {
		t.Error("expected Enter not to set ActionStdout when goal is empty")
	}

	updated3, cmd3 := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	if cmd3 != nil {
		t.Error("expected 'c' to be a no-op when goal is empty")
	}
	m4 := updated3.(model)
	if m4.result.Action == ActionCopy {
		t.Error("expected 'c' not to set ActionCopy when goal is empty")
	}

	updated4, _ := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("w")})
	m5 := updated4.(model)
	if m5.enteringFilename {
		t.Error("expected 'w' not to open the filename modal when goal is empty")
	}
}

func TestFocus_ConfirmActionsWorkOnceGoalIsTyped(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: ""}) // focus=focusGoal

	cur := m
	for _, r := range "hello" {
		u, _ := cur.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		cur = u.(model)
	}
	updated, _ := cur.Update(tea.KeyMsg{Type: tea.KeyEsc}) // blur to skills
	m2 := updated.(model)

	updated2, cmd := m2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m3 := updated2.(model)
	if cmd == nil {
		t.Fatal("expected Enter to quit once the goal is non-empty")
	}
	if m3.result.Action != ActionStdout {
		t.Errorf("Action = %v, want ActionStdout", m3.result.Action)
	}
}

func TestView_ExactlyOneFocusMarkerAcrossAllZones(t *testing.T) {
	// The skill cursor and the focused field/preview title both used the
	// same \u203a marker unconditionally, so e.g. focusing a field left
	// the skill cursor's marker showing too - looking "active" when it
	// wasn't (the bug reported after smoke testing: up/down appeared not
	// to select skills, because focus was actually on a field the whole
	// time, and nothing on screen disambiguated that). The invariant:
	// exactly one \u203a marker on screen, always on the truly focused
	// zone, checked across every zone in the Tab cycle.
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "goal"})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 90, Height: 20})
	cur := updated.(model)

	for i := 0; i < len(focusCycle); i++ {
		got := stripANSI(cur.View())
		if n := strings.Count(got, "\u203a"); n != 1 {
			t.Errorf("focus=%v: expected exactly one \u203a marker, got %d in:\n%s", cur.focus, n, got)
		}
		u, _ := cur.Update(tea.KeyMsg{Type: tea.KeyTab})
		cur = u.(model)
	}
}
