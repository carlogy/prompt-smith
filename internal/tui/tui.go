package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/carlogy/prompt-smith/internal/prompt"
	"github.com/carlogy/prompt-smith/internal/registry"
)

// Run launches the interactive skill picker + live preview and returns
// the user's finalized choice. initial seeds the goal and any optional
// fields already supplied via flags/args, plus any skills already
// selected (e.g. via --tui with --skills, which pre-checks them).
//
// Run never performs the chosen action itself (no file writes, no
// clipboard) - the caller applies Result the same way it would flag-only
// input, so delivery logic is never duplicated between the two paths.
func Run(reg *registry.Registry, initial prompt.Inputs) (Result, error) {
	p := tea.NewProgram(newModel(reg, initial), tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return Result{}, err
	}
	return finalModel.(model).result, nil
}
