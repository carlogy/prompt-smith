package cli

import (
	"errors"
	"os"

	"golang.org/x/term"
)

// isInteractive reports whether both stdin and stdout are attached to a
// terminal. A package variable so tests can force either branch without
// needing a real TTY.
var isInteractive = func() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

// decideUseTUI applies the interactive-picker gate: launch the TUI when
// running interactively and not explicitly skipped, either because no
// skills were given or the user forced it with --tui (which pre-selects
// whatever --skills already supplied). --quick and --tui together, or
// --tui outside an interactive terminal, are user errors reported
// eagerly rather than silently falling back to a different mode.
func decideUseTUI(interactive, quick, forceTUI bool, numSkills int) (bool, error) {
	if quick && forceTUI {
		return false, errors.New("promptsmith: --tui and --quick are mutually exclusive")
	}
	if forceTUI && !interactive {
		return false, errors.New("promptsmith: --tui requires an interactive terminal")
	}
	if !interactive || quick {
		return false, nil
	}
	return forceTUI || numSkills == 0, nil
}
