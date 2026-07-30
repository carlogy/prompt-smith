// promptsmith - generate portable, skill-aware prompts for any LLM or agent harness.
// Copyright (C) 2026 carlogy
// SPDX-License-Identifier: AGPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package tui

import (
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/carlogy/prompt-smith/internal/naming"
	"github.com/carlogy/prompt-smith/internal/prompt"
	"github.com/carlogy/prompt-smith/internal/registry"
)

// item is one row in the skill list: either a non-selectable category
// header or a selectable skill.
type item struct {
	isHeader bool
	category string // set when isHeader
	skill    registry.Skill
	selected bool
}

// model is the Bubble Tea model for the skill picker + live preview.
// goal/context/constraints/role/outputFormat/examples and the target
// are all editable in place: the first five via their single-line
// textinput, examples via its multi-line textarea, the target via the
// arrow keys while focusTarget has focus. Every change recomputes the
// preview via prompt.Build.
type model struct {
	reg    *registry.Registry
	target string
	items  []item
	cursor int

	// termWidth/termHeight are set from tea.WindowSizeMsg; zero until
	// the first one arrives, in which case computeLayout falls back to
	// a usable default rather than a degenerate size.
	termWidth  int
	termHeight int

	// focus is which zone currently receives key input; Tab/Shift+Tab
	// cycle it. Zero value is focusSkills, matching pre-P3c behavior.
	focus focusZone

	goal              string
	goalInput         textinput.Model
	context           string
	contextInput      textinput.Model
	constraints       string
	constraintsInput  textinput.Model
	role              string
	roleInput         textinput.Model
	outputFormat      string
	outputFormatInput textinput.Model
	examples          string
	examplesInput     textarea.Model

	preview   string
	previewVP viewport.Model

	enteringFilename bool
	filenameInput    textinput.Model

	// keys/help back the footer and the "?" overlay (keys.go, view.go).
	// keys.zone is kept in sync with focus by changeFocus - the one
	// place focus ever changes - so help.View(m.keys) always describes
	// whatever zone is actually focused. help.ShowAll doubles as the
	// overlay's own open/closed flag (see viewHelpOverlay/
	// updateHelpOverlay) rather than adding a separate bool: that's
	// literally what the field is for, and bubbles/help already
	// switches its own View() behavior on it.
	keys keyMap
	help help.Model

	result Result
}

// newModel builds the initial model: items filtered to what the target
// actually supports (registry.SupportsTarget), grouped by category in
// canonical order (registry.SortSkills), with initial.Skills
// pre-selected. The cursor starts on the first selectable item, and the
// preview reflects the pre-selected skills from the start.
func newModel(reg *registry.Registry, initial prompt.Inputs) model {
	items := buildItems(reg, initial.Target, initial.Skills)
	l := computeLayout(0, 0) // falls back to a usable default until the first WindowSizeMsg

	// Prompt defaults to "> " (bubbles/textinput), rendered outside the
	// width-constrained value area - "Label: > value" is both redundant
	// (the row's own label already says what this is) and, worse,
	// consumes 2 cols the field-width budget in the WindowSizeMsg
	// handler doesn't account for, which wrapped long values onto a
	// second physical row (found via
	// TestView_FieldRowsDoNotWrapWithLongValues). Clearing it removes
	// both problems at once.
	newField := func(value, placeholder string) textinput.Model {
		ti := textinput.New()
		ti.Prompt = ""
		ti.Placeholder = placeholder
		ti.SetValue(value)
		return ti
	}
	goalInput := newField(initial.Goal, "what to do")
	contextInput := newField(initial.Context, "relevant background")
	constraintsInput := newField(initial.Constraints, "must respect")
	roleInput := newField(initial.Role, "persona to adopt")
	outputFormatInput := newField(initial.OutputFormat, "response shape")

	// The Examples textarea seeds from initial.Examples via
	// prompt.JoinExamples, never by rendering the []string directly -
	// that's the one shared representation prompt.SplitExamples can
	// invert cleanly on read-back (see currentInputs). ShowLineNumbers
	// and Prompt must be cleared BEFORE SetWidth, per SetWidth's own
	// doc comment - both introduce per-line decoration none of the
	// single-line textinput fields above have (they already clear
	// their own Prompt for the same "no stray decoration" reason), and
	// SetWidth's reserved-inner-width math accounts for both, so
	// setting them after would leave stale width bookkeeping.
	examplesInput := textarea.New()
	examplesInput.Placeholder = "worked examples, separated by a line containing only ---"
	examplesInput.ShowLineNumbers = false
	examplesInput.Prompt = ""
	examplesInput.SetWidth(examplesFieldWidth(l))
	examplesInput.SetHeight(examplesRows)
	examplesInput.SetValue(prompt.JoinExamples(initial.Examples))

	// help.Model's Width is set per-render in viewFooter, not here -
	// it depends on the focused zone's descriptor sentence length,
	// which changes independently of any resize. Its Styles ARE fixed
	// up front, matching the rest of this package's styling living in
	// theme.go rather than scattered per-callsite.
	h := help.New()
	h.Styles.ShortKey = helpKeyStyle
	h.Styles.ShortDesc = helpDescStyle
	h.Styles.ShortSeparator = helpSeparatorStyle
	h.Styles.Ellipsis = helpSeparatorStyle
	h.Styles.FullKey = helpKeyStyle
	h.Styles.FullDesc = helpDescStyle
	h.Styles.FullSeparator = helpSeparatorStyle

	m := model{
		reg:               reg,
		target:            initial.Target,
		items:             items,
		cursor:            firstSelectable(items),
		goal:              initial.Goal,
		goalInput:         goalInput,
		context:           initial.Context,
		contextInput:      contextInput,
		constraints:       initial.Constraints,
		constraintsInput:  constraintsInput,
		role:              initial.Role,
		roleInput:         roleInput,
		outputFormat:      initial.OutputFormat,
		outputFormatInput: outputFormatInput,
		examples:          prompt.JoinExamples(initial.Examples),
		examplesInput:     examplesInput,
		previewVP:         viewport.New(l.rightContentWidth-scrollbarWidth, l.contentHeight-1),
		keys:              newKeyMap(),
		help:              h,
	}
	// An empty goal (bare `promptsmith` with no goal argument) starts
	// with the goal field focused so there's an immediate, obvious next
	// action; a goal already supplied via flags/args keeps the P3a/P3b
	// default of starting on the skill list.
	if initial.Goal == "" {
		focused, _ := m.changeFocus(focusGoal)
		m = focused.(model)
	}
	m.recomputePreview()
	return m
}

func buildItems(reg *registry.Registry, target string, selected []string) []item {
	selectedSet := make(map[string]bool, len(selected))
	for _, id := range selected {
		selectedSet[id] = true
	}

	skills := append([]registry.Skill(nil), reg.Skills...)
	reg.SortSkills(skills)

	var items []item
	lastCategory := ""
	for _, sk := range skills {
		if !reg.SupportsTarget(sk, target) {
			continue
		}
		if sk.Category != lastCategory {
			items = append(items, item{isHeader: true, category: sk.Category})
			lastCategory = sk.Category
		}
		items = append(items, item{skill: sk, selected: selectedSet[sk.ID]})
	}
	return items
}

// sortedTargetIDs returns the registry's target ids, alphabetically -
// mirrors internal/server/app.go's sortedTargetIDs (Targets has no
// canonical order, unlike Categories, since it's a map; alphabetical is
// the simplest deterministic choice for cycling through with left/right).
func sortedTargetIDs(reg *registry.Registry) []string {
	ids := make([]string, 0, len(reg.Targets))
	for id := range reg.Targets {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func firstSelectable(items []item) int {
	for i, it := range items {
		if !it.isHeader {
			return i
		}
	}
	return 0
}

// examplesFieldWidth is the Examples textarea's rendered width, shared
// between newModel (before the first WindowSizeMsg) and the
// WindowSizeMsg handler (after every resize) so the two can't drift.
// Unlike the single-line fields' fieldWidth (see the WindowSizeMsg
// handler), this does NOT subtract fieldLabelWidth/": "/markerWidth -
// those account for a label sharing the SAME line as its value, which
// only applies to the five "Label: value" fields. The Examples label
// sits on its own line (viewExamplesField), so the textarea below it
// is free to claim the full pane content width, matching the skill
// list's content width above it in the same pane.
func examplesFieldWidth(l layout) int {
	w := l.leftContentWidth - scrollbarWidth
	if w < minContentWidth {
		w = minContentWidth
	}
	return w
}

// Init satisfies tea.Model.
func (m model) Init() tea.Cmd {
	return nil
}

// Update satisfies tea.Model. Selection toggling recomputes the preview
// immediately. Confirm actions quit the program (tea.Quit) with result
// populated for the caller to act on.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.termWidth, m.termHeight = msg.Width, msg.Height
		l := computeLayout(m.termWidth, m.termHeight)
		m.previewVP.Width = l.rightContentWidth - scrollbarWidth // -scrollbarWidth: reserve the gutter column
		m.previewVP.Height = l.contentHeight - 1                 // -1: the "Preview" title line

		// Field inputs share the skill list's content width (matched to
		// its scrollbar-reserved width so the pane's rendered width -
		// sized to its widest line - stays identical for both
		// sections), minus each row's "Label: " prefix AND minus the
		// 2-col "\u203a "/"  " focus-marker prefix viewFields adds
		// after this width is used (found via
		// TestView_FieldRowsDoNotWrapWithLongValues going red: without
		// this, a long value's row was 2 cols wider than budgeted and
		// lipgloss.Width wrapped it onto a second physical line instead
		// of leaving it to the input's own horizontal scroll). Measured
		// with lipgloss.Width, not len - len is byte length, and
		// \u203a is a multi-byte UTF-8 character but a single display
		// column.
		markerWidth := lipgloss.Width("\u203a ")
		fieldWidth := (l.leftContentWidth - scrollbarWidth) - fieldLabelWidth - len(": ") - markerWidth
		if fieldWidth < minContentWidth {
			fieldWidth = minContentWidth
		}
		m.goalInput.Width = fieldWidth
		m.contextInput.Width = fieldWidth
		m.constraintsInput.Width = fieldWidth
		m.roleInput.Width = fieldWidth
		m.outputFormatInput.Width = fieldWidth

		// The Examples textarea has no inline label sharing its line
		// (see viewExamplesField), so it doesn't subtract
		// fieldLabelWidth/markerWidth like the five fields above - it
		// gets the full pane content width instead (see
		// examplesFieldWidth's doc comment). Both width AND height are
		// set here even though the height never actually changes
		// (examplesRows is fixed) - SetWidth's own doc comment requires
		// SetHeight/SetWidth be called together after Prompt/
		// ShowLineNumbers are set, and newModel already establishes
		// that pairing once; resetting both here keeps the two call
		// sites symmetric rather than one of them being "half" a
		// resize.
		m.examplesInput.SetWidth(examplesFieldWidth(l))
		m.examplesInput.SetHeight(examplesRows)

		// Re-wrap the preview to the new width - the viewport's
		// content is pre-wrapped (see recomputePreview), so a resize
		// leaves it wrapped to the stale width otherwise. This also
		// resets preview scroll to top on resize, which is acceptable.
		m.recomputePreview()
		return m, nil
	case tea.KeyMsg:
		if m.enteringFilename {
			return m.updateFilenameInput(msg)
		}
		if m.help.ShowAll {
			return m.updateHelpOverlay(msg)
		}
		switch {
		case key.Matches(msg, m.keys.Tab):
			return m.changeFocus(nextFocus(m.focus))
		case key.Matches(msg, m.keys.ShiftTab):
			return m.changeFocus(prevFocus(m.focus))
		}
		switch m.focus {
		case focusGoal:
			return m.updateGoalField(msg)
		case focusContext:
			return m.updateContextField(msg)
		case focusConstraints:
			return m.updateConstraintsField(msg)
		case focusRole:
			return m.updateRoleField(msg)
		case focusOutputFormat:
			return m.updateOutputFormatField(msg)
		case focusExamples:
			return m.updateExamplesField(msg)
		case focusTarget:
			return m.updateTargetField(msg)
		}
		return m.updatePicker(msg)
	case tea.MouseMsg:
		// Ignored entirely while the filename modal is up - the split
		// view (and its geometry) isn't on screen then.
		if m.enteringFilename {
			return m, nil
		}
		// Deliberately not delegated to previewVP.Update(msg): its
		// default keymap also binds Up/Down, which must stay reserved
		// for the skill cursor. Wheel + left-click are handled
		// explicitly instead.
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.previewVP.ScrollUp(mouseWheelLines)
		case tea.MouseButtonWheelDown:
			m.previewVP.ScrollDown(mouseWheelLines)
		case tea.MouseButtonLeft:
			if msg.Action == tea.MouseActionPress {
				return m.handleLeftClick(msg.X, msg.Y), nil
			}
		}
		return m, nil
	}
	return m, nil
}

// changeFocus blurs whichever field is currently focused, sets focus to
// to, and focuses that zone's field if it has one (skills/preview don't).
// Returns any tea.Cmd the newly-focused field wants (e.g. cursor blink).
func (m model) changeFocus(to focusZone) (tea.Model, tea.Cmd) {
	m.goalInput.Blur()
	m.contextInput.Blur()
	m.constraintsInput.Blur()
	m.roleInput.Blur()
	m.outputFormatInput.Blur()
	m.examplesInput.Blur()
	m.focus = to
	m.keys.zone = to // keeps help.View(m.keys) describing whatever zone is actually focused

	var cmd tea.Cmd
	switch to {
	case focusGoal:
		cmd = m.goalInput.Focus()
	case focusContext:
		cmd = m.contextInput.Focus()
	case focusConstraints:
		cmd = m.constraintsInput.Focus()
	case focusRole:
		cmd = m.roleInput.Focus()
	case focusOutputFormat:
		cmd = m.outputFormatInput.Focus()
	case focusExamples:
		cmd = m.examplesInput.Focus()
	}
	return m, cmd
}

// updateGoalField routes a key to the goal textinput while it's focused,
// keeps m.goal in sync with its value, and recomputes the live preview.
// Esc blurs back to the skill list rather than being passed to the
// field (which would do nothing) or canceling the whole TUI.
func (m model) updateGoalField(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Esc) {
		return m.changeFocus(focusSkills)
	}
	cmd := m.updateTextField(msg, &m.goalInput, &m.goal)
	return m, cmd
}

// updateContextField mirrors updateGoalField for the context field.
func (m model) updateContextField(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Esc) {
		return m.changeFocus(focusSkills)
	}
	cmd := m.updateTextField(msg, &m.contextInput, &m.context)
	return m, cmd
}

// updateConstraintsField mirrors updateGoalField for the constraints field.
func (m model) updateConstraintsField(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Esc) {
		return m.changeFocus(focusSkills)
	}
	cmd := m.updateTextField(msg, &m.constraintsInput, &m.constraints)
	return m, cmd
}

// updateRoleField mirrors updateGoalField for the role field.
func (m model) updateRoleField(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Esc) {
		return m.changeFocus(focusSkills)
	}
	cmd := m.updateTextField(msg, &m.roleInput, &m.role)
	return m, cmd
}

// updateOutputFormatField mirrors updateGoalField for the output-format field.
func (m model) updateOutputFormatField(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Esc) {
		return m.changeFocus(focusSkills)
	}
	cmd := m.updateTextField(msg, &m.outputFormatInput, &m.outputFormat)
	return m, cmd
}

// updateExamplesField routes a key to the Examples textarea while it's
// focused. Esc blurs back to the skill list, matching every other
// field - but Enter is deliberately NOT special-cased the way Esc is:
// it's forwarded to the textarea unmodified, and the textarea's own
// default keymap already binds Enter (and ctrl+m) to inserting a
// newline. That's exactly what multi-line, "---"-separated examples
// need, and it means this is the one field where Enter can't
// confirm/submit the whole picker the way it does everywhere else
// (Enter in updatePicker, or Ctrl+C/Esc canceling) - the user has to
// Tab away first. That's an accepted tradeoff, not an oversight: making
// Enter submit here would take away the only way to type a literal
// newline into an example. footerHelpFor's focusExamples case exists
// specifically to make the tradeoff discoverable, so don't "fix" this
// by intercepting Enter to quit - that would silently break the one
// thing this field exists for.
func (m model) updateExamplesField(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Esc) {
		return m.changeFocus(focusSkills)
	}
	var cmd tea.Cmd
	m.examplesInput, cmd = m.examplesInput.Update(msg)
	m.examples = m.examplesInput.Value()
	m.recomputePreview()
	return m, cmd
}

// updateTargetField handles keys while the target picker has focus:
// Left/Right cycle to the previous/next target id (alphabetical,
// wrapping), and Esc blurs back to the skill list (matching every text
// field's Esc behavior). A target change rebuilds items from scratch -
// buildItems re-filters by registry.SupportsTarget, so a skill
// unsupported on the new target drops out, matching the web UI - while
// preserving which currently-selected skills are still supported, then
// resets the cursor to the first selectable item and recomputes the
// preview.
func (m model) updateTargetField(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Esc):
		return m.changeFocus(focusSkills)
	case key.Matches(msg, m.keys.Left, m.keys.Right):
		ids := sortedTargetIDs(m.reg)
		if len(ids) == 0 {
			return m, nil
		}
		cur := 0
		for i, id := range ids {
			if id == m.target {
				cur = i
				break
			}
		}
		if key.Matches(msg, m.keys.Left) {
			cur = (cur - 1 + len(ids)) % len(ids)
		} else {
			cur = (cur + 1) % len(ids)
		}
		m.target = ids[cur]

		selected := m.selectedIDs() // capture before reassigning m.items
		m.items = buildItems(m.reg, m.target, selected)
		m.cursor = firstSelectable(m.items)
		m.recomputePreview()
	}
	return m, nil
}

// updateTextField routes msg to input, syncs *target with the field's
// new value, and recomputes the live preview. Shared by every editable
// field's update method so the routing/sync/recompute pattern isn't
// duplicated per field.
func (m *model) updateTextField(msg tea.KeyMsg, input *textinput.Model, target *string) tea.Cmd {
	var cmd tea.Cmd
	*input, cmd = input.Update(msg)
	*target = input.Value()
	m.recomputePreview()
	return cmd
}

// updatePicker handles keys while the skill list has focus.
func (m model) updatePicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Up):
		if m.focus == focusPreview {
			m.previewVP.ScrollUp(arrowScrollLines)
		} else {
			m.cursor = prevSelectable(m.items, m.cursor)
		}
	case key.Matches(msg, m.keys.Down):
		if m.focus == focusPreview {
			m.previewVP.ScrollDown(arrowScrollLines)
		} else {
			m.cursor = nextSelectable(m.items, m.cursor)
		}
	case key.Matches(msg, m.keys.PgUp):
		m.previewVP.PageUp()
	case key.Matches(msg, m.keys.PgDown):
		m.previewVP.PageDown()
	case key.Matches(msg, m.keys.Space):
		if m.focus == focusSkills && !m.items[m.cursor].isHeader {
			// Update has a value receiver, but m.items is a slice:
			// copying the struct does NOT copy the backing array, so
			// mutating m.items[i] in place would corrupt the model
			// this Update call started from. Copy the slice first so
			// the two stay independent.
			items := append([]item(nil), m.items...)
			items[m.cursor].selected = !items[m.cursor].selected
			m.items = items
			m.recomputePreview()
		}
	case key.Matches(msg, m.keys.Enter):
		if m.goalIsEmpty() {
			return m, nil
		}
		m.result = Result{Inputs: m.currentInputs(), Action: ActionStdout}
		return m, tea.Quit
	case key.Matches(msg, m.keys.Esc, m.keys.CtrlC):
		m.result = Result{Action: ActionCancel}
		return m, tea.Quit
	case msg.Type == tea.KeyRunes:
		switch string(msg.Runes) {
		case "c":
			if m.goalIsEmpty() {
				return m, nil
			}
			m.result = Result{Inputs: m.currentInputs(), Action: ActionCopy}
			return m, tea.Quit
		case "w":
			if m.goalIsEmpty() {
				return m, nil
			}
			m.enteringFilename = true
			m.filenameInput = textinput.New()
			m.filenameInput.SetValue(naming.SuggestFilename(m.goal, time.Now()))
			m.filenameInput.Focus()
		case "?":
			// Only reachable here (updatePicker), which - per model's
			// Update switch above - is only ever called with
			// focus==focusSkills or focus==focusPreview: every other
			// zone's case in that switch returns before falling
			// through to this call. That's what keeps "?" from
			// swallowing a literal "?" typed into a text field.
			m.help.ShowAll = !m.help.ShowAll
		}
	}
	return m, nil
}

// updateFilenameInput handles keys while the write-to-file filename
// input has focus (opened by "w" in updatePicker).
func (m model) updateFilenameInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Enter):
		m.result = Result{
			Inputs:    m.currentInputs(),
			Action:    ActionWrite,
			WritePath: m.filenameInput.Value(),
		}
		return m, tea.Quit
	case key.Matches(msg, m.keys.Esc):
		// Abandon the write, return focus to the picker - not a full
		// cancel of the whole TUI.
		m.enteringFilename = false
		return m, nil
	}

	var cmd tea.Cmd
	m.filenameInput, cmd = m.filenameInput.Update(msg)
	return m, cmd
}

// updateHelpOverlay handles keys while the full-screen "?" help
// overlay is up (opened by "?" in updatePicker's tea.KeyRunes branch -
// see the comment there for why "?" can't be a key.Matches binding).
// Mirrors updateFilenameInput's shape: intercept everything while this
// mode is active, dismiss on either its own toggle key or Esc, ignore
// every other key rather than letting it leak through to whatever zone
// was focused underneath.
func (m model) updateHelpOverlay(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Esc) {
		m.help.ShowAll = false
		return m, nil
	}
	if msg.Type == tea.KeyRunes && string(msg.Runes) == "?" {
		m.help.ShowAll = false
		return m, nil
	}
	return m, nil
}

// handleLeftClick maps a click to a skill row (via the same geometry the
// view renders with) and, if it lands on a selectable item, moves the
// cursor there and toggles it - the mouse equivalent of navigating with
// the arrows and pressing space.
func (m model) handleLeftClick(x, y int) model {
	l := computeLayout(m.termWidth, m.termHeight)
	leftPaneWidth := l.leftContentWidth + paneHOverhead
	listHeight := l.contentHeight - 1
	_, offset := visibleWindow(m.items, m.cursor, listHeight)

	idx, ok := itemAtPoint(x, y, leftPaneWidth, listHeight, offset, m.items)
	if !ok {
		return m
	}

	m.cursor = idx
	// Copy before mutating: m.items shares its backing array with the
	// model this Update started from (see the space-toggle note).
	items := append([]item(nil), m.items...)
	items[idx].selected = !items[idx].selected
	m.items = items
	m.recomputePreview()
	return m
}

// currentInputs builds the prompt.Inputs the current model state would
// produce: the fixed goal/context/etc from initial, plus whatever is
// currently selected.
func (m model) currentInputs() prompt.Inputs {
	return prompt.Inputs{
		Target:       m.target,
		Skills:       m.selectedIDs(),
		Goal:         m.goal,
		Context:      m.context,
		Constraints:  m.constraints,
		Role:         m.role,
		OutputFormat: m.outputFormat,
		// prompt.SplitExamples is prompt.JoinExamples's inverse (used
		// to seed m.examples in newModel) - dividing the textarea's one
		// free-form string back into the []string prompt.Inputs.Examples
		// expects, on "---"-only lines. Read from m.examples (kept in
		// sync by updateExamplesField), not m.examplesInput.Value()
		// directly, matching every other field's read-back pattern here.
		Examples: prompt.SplitExamples(m.examples),
	}
}

// goalIsEmpty reports whether the goal is blank (whitespace-only counts
// as blank). Confirm actions (stdout/copy/write) are blocked while
// true, matching the same "goal is required" policy the non-interactive
// flag path enforces (errEmptyGoal) - Build itself doesn't require a
// goal (an empty one just omits <task>), but a goal-less prompt is
// rarely useful, so both paths hold the same line. Cancel is exempt:
// you can always back out, empty goal or not.
func (m model) goalIsEmpty() bool {
	return strings.TrimSpace(m.goal) == ""
}

// selectedIDs returns the ids of every currently-selected skill, in the
// same canonical order they appear in items (already sorted by
// registry.SortSkills when items was built).
func (m model) selectedIDs() []string {
	var ids []string
	for _, it := range m.items {
		if !it.isHeader && it.selected {
			ids = append(ids, it.skill.ID)
		}
	}
	return ids
}

// recomputePreview rebuilds the prompt from the current selection and
// fixed fields via the same tested engine the non-interactive path uses,
// refreshes the preview viewport's content, and resets its scroll to
// the top - a stale scroll offset over new content would be confusing.
func (m *model) recomputePreview() {
	out, err := prompt.Build(m.reg, m.currentInputs())
	m.preview = out

	content := highlightTags(m.preview)
	if err != nil {
		content = "error: " + err.Error()
	}
	// bubbles v1 viewport does not soft-wrap content itself, so long
	// lines would otherwise overflow the pane horizontally and get
	// clipped. Wrap AFTER highlightTags (not before): wrapping first
	// would break prompthl.Classify's tag detection, and the long
	// overflowing lines are the unstyled body lines anyway. m.preview
	// stays the raw, unwrapped string - only this display copy wraps.
	if w := m.previewVP.Width; w > 0 {
		content = lipgloss.NewStyle().Width(w).Render(content)
	}
	m.previewVP.SetContent(content)
	m.previewVP.GotoTop()
}

// mouseWheelLines is how many lines one wheel tick scrolls the preview.
const mouseWheelLines = 3

// arrowScrollLines is how many lines Up/Down scroll the preview when
// it's focused - finer-grained than a wheel tick or PgUp/PgDn, matching
// common pager conventions (arrows = line-at-a-time, page keys = a page).
const arrowScrollLines = 1

func prevSelectable(items []item, from int) int {
	for i := from - 1; i >= 0; i-- {
		if !items[i].isHeader {
			return i
		}
	}
	return from
}

func nextSelectable(items []item, from int) int {
	for i := from + 1; i < len(items); i++ {
		if !items[i].isHeader {
			return i
		}
	}
	return from
}
