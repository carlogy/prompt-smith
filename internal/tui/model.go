package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

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
// goal/context/constraints/role/outputFormat are fixed from the initial
// Inputs (inline editing lands in a later phase); only skill selection
// changes, which recomputes preview via prompt.Build.
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

	goal         string
	context      string
	constraints  string
	role         string
	outputFormat string

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
	m := model{
		reg:          reg,
		target:       initial.Target,
		items:        items,
		cursor:       firstSelectable(items),
		goal:         initial.Goal,
		context:      initial.Context,
		constraints:  initial.Constraints,
		role:         initial.Role,
		outputFormat: initial.OutputFormat,
		previewVP:    viewport.New(l.rightContentWidth, l.contentHeight-1),
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
		m.previewVP.Width = l.rightContentWidth
		m.previewVP.Height = l.contentHeight - 1 // -1: the "Preview" title line
		return m, nil
	case tea.KeyMsg:
		if m.enteringFilename {
			return m.updateFilenameInput(msg)
		}
		return m.updatePicker(msg)
	case tea.MouseMsg:
		// Deliberately not delegated to previewVP.Update(msg): its
		// default keymap also binds Up/Down, which must stay reserved
		// for the skill cursor. Wheel events are handled explicitly
		// instead, scoped to just wheel up/down.
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.previewVP.ScrollUp(mouseWheelLines)
		case tea.MouseButtonWheelDown:
			m.previewVP.ScrollDown(mouseWheelLines)
		}
		return m, nil
	}
	return m, nil
}

// updatePicker handles keys while the skill list has focus.
func (m model) updatePicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyUp:
		m.cursor = prevSelectable(m.items, m.cursor)
	case tea.KeyDown:
		m.cursor = nextSelectable(m.items, m.cursor)
	case tea.KeyPgUp:
		m.previewVP.PageUp()
	case tea.KeyPgDown:
		m.previewVP.PageDown()
	case tea.KeySpace:
		if !m.items[m.cursor].isHeader {
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
		m.result = Result{Inputs: m.currentInputs(), Action: ActionStdout}
		return m, tea.Quit
	case tea.KeyEsc, tea.KeyCtrlC:
		m.result = Result{Action: ActionCancel}
		return m, tea.Quit
	case tea.KeyRunes:
		switch string(msg.Runes) {
		case "c":
			m.result = Result{Inputs: m.currentInputs(), Action: ActionCopy}
			return m, tea.Quit
		case "w":
			m.enteringFilename = true
			m.filenameInput = textinput.New()
			m.filenameInput.SetValue(SuggestFilename(m.goal, time.Now()))
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
	m.previewVP.SetContent(content)
	m.previewVP.GotoTop()
}

// mouseWheelLines is how many lines one wheel tick scrolls the preview.
const mouseWheelLines = 3

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
