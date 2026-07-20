package tui

import (
	"sort"
	"strings"
	"time"

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
// goal/context/constraints/role/outputFormat and the target are all
// editable in place: text fields via their textinput, the target via
// the arrow keys while focusTarget has focus. Every change recomputes
// the preview via prompt.Build.
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

	preview   string
	previewVP viewport.Model

	enteringFilename bool
	filenameInput    textinput.Model

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
		previewVP:         viewport.New(l.rightContentWidth-scrollbarWidth, l.contentHeight-1),
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
		switch msg.Type {
		case tea.KeyTab:
			return m.changeFocus(nextFocus(m.focus))
		case tea.KeyShiftTab:
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
	m.focus = to

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
	}
	return m, cmd
}

// updateGoalField routes a key to the goal textinput while it's focused,
// keeps m.goal in sync with its value, and recomputes the live preview.
// Esc blurs back to the skill list rather than being passed to the
// field (which would do nothing) or canceling the whole TUI.
func (m model) updateGoalField(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyEsc {
		return m.changeFocus(focusSkills)
	}
	cmd := m.updateTextField(msg, &m.goalInput, &m.goal)
	return m, cmd
}

// updateContextField mirrors updateGoalField for the context field.
func (m model) updateContextField(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyEsc {
		return m.changeFocus(focusSkills)
	}
	cmd := m.updateTextField(msg, &m.contextInput, &m.context)
	return m, cmd
}

// updateConstraintsField mirrors updateGoalField for the constraints field.
func (m model) updateConstraintsField(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyEsc {
		return m.changeFocus(focusSkills)
	}
	cmd := m.updateTextField(msg, &m.constraintsInput, &m.constraints)
	return m, cmd
}

// updateRoleField mirrors updateGoalField for the role field.
func (m model) updateRoleField(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyEsc {
		return m.changeFocus(focusSkills)
	}
	cmd := m.updateTextField(msg, &m.roleInput, &m.role)
	return m, cmd
}

// updateOutputFormatField mirrors updateGoalField for the output-format field.
func (m model) updateOutputFormatField(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyEsc {
		return m.changeFocus(focusSkills)
	}
	cmd := m.updateTextField(msg, &m.outputFormatInput, &m.outputFormat)
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
	switch msg.Type {
	case tea.KeyEsc:
		return m.changeFocus(focusSkills)
	case tea.KeyLeft, tea.KeyRight:
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
		if msg.Type == tea.KeyLeft {
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
	switch msg.Type {
	case tea.KeyUp:
		if m.focus == focusPreview {
			m.previewVP.ScrollUp(arrowScrollLines)
		} else {
			m.cursor = prevSelectable(m.items, m.cursor)
		}
	case tea.KeyDown:
		if m.focus == focusPreview {
			m.previewVP.ScrollDown(arrowScrollLines)
		} else {
			m.cursor = nextSelectable(m.items, m.cursor)
		}
	case tea.KeyPgUp:
		m.previewVP.PageUp()
	case tea.KeyPgDown:
		m.previewVP.PageDown()
	case tea.KeySpace:
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
	case tea.KeyEnter:
		if m.goalIsEmpty() {
			return m, nil
		}
		m.result = Result{Inputs: m.currentInputs(), Action: ActionStdout}
		return m, tea.Quit
	case tea.KeyEsc, tea.KeyCtrlC:
		m.result = Result{Action: ActionCancel}
		return m, tea.Quit
	case tea.KeyRunes:
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
		}
	}
	return m, nil
}

// updateFilenameInput handles keys while the write-to-file filename
// input has focus (opened by "w" in updatePicker).
func (m model) updateFilenameInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		m.result = Result{
			Inputs:    m.currentInputs(),
			Action:    ActionWrite,
			WritePath: m.filenameInput.Value(),
		}
		return m, tea.Quit
	case tea.KeyEsc:
		// Abandon the write, return focus to the picker - not a full
		// cancel of the whole TUI.
		m.enteringFilename = false
		return m, nil
	}

	var cmd tea.Cmd
	m.filenameInput, cmd = m.filenameInput.Update(msg)
	return m, cmd
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
