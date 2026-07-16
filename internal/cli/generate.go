package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/spf13/cobra"

	"github.com/carlogy/prompt-smith/internal/prompt"
	"github.com/carlogy/prompt-smith/internal/registry"
	"github.com/carlogy/prompt-smith/internal/tui"
)

// copyToClipboard puts text on the system clipboard. A package variable
// so tests can substitute a spy instead of touching the real clipboard.
var copyToClipboard = clipboard.WriteAll

// runTUIFunc launches the interactive skill picker. A package variable
// so tests can substitute a spy instead of starting a real Bubble Tea
// program (which would block reading real stdin).
var runTUIFunc = tui.Run

// errEmptyGoal is returned when no goal text was given.
var errEmptyGoal = errors.New(`promptsmith: a goal is required, e.g. promptsmith "fix the flaky test"`)

// generateOptions holds the root command's flag values.
type generateOptions struct {
	target       string
	skills       []string
	context      string
	constraints  string
	role         string
	outputFormat string
	toClipboard  bool
	out          string
	quick        bool
	tui          bool
}

// addGenerateFlags registers the generate flags on cmd and wires its RunE.
func addGenerateFlags(cmd *cobra.Command, reg *registry.Registry) {
	opts := &generateOptions{}

	cmd.Flags().StringVarP(&opts.target, "target", "t", "generic", "target harness: generic|opencode|claude-code")
	cmd.Flags().StringSliceVarP(&opts.skills, "skills", "s", nil, "skills to include (comma-separated or repeatable)")
	cmd.Flags().StringVarP(&opts.context, "context", "x", "", "background/context for the goal")
	cmd.Flags().StringVarP(&opts.constraints, "constraints", "C", "", "constraints the solution must respect")
	cmd.Flags().StringVarP(&opts.role, "role", "r", "", "role/persona to open the prompt with")
	cmd.Flags().StringVarP(&opts.outputFormat, "output-format", "f", "", "desired shape of the response")
	cmd.Flags().BoolVarP(&opts.toClipboard, "copy", "c", false, "copy the prompt to the clipboard instead of stdout")
	cmd.Flags().StringVarP(&opts.out, "out", "o", "", "write the prompt to this file instead of stdout")
	cmd.Flags().BoolVarP(&opts.quick, "quick", "q", false, "never launch the interactive picker, even in a terminal")
	cmd.Flags().BoolVar(&opts.tui, "tui", false, "launch the interactive picker even if --skills was given")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runGenerate(cmd, reg, opts, args)
	}
}

func runGenerate(cmd *cobra.Command, reg *registry.Registry, opts *generateOptions, args []string) error {
	goal := strings.TrimSpace(strings.Join(args, " "))

	useTUI, err := decideUseTUI(isInteractive(), opts.quick, opts.tui, len(opts.skills))
	if err != nil {
		return err
	}

	if useTUI {
		// goal may be empty here (bare `promptsmith`): the picker
		// collects it inline, focused on the goal field by default.
		return runInteractive(cmd, reg, opts, goal)
	}

	if goal == "" {
		return errEmptyGoal
	}

	out, err := prompt.Build(reg, prompt.Inputs{
		Target:       opts.target,
		Skills:       opts.skills,
		Goal:         goal,
		Context:      opts.context,
		Constraints:  opts.constraints,
		Role:         opts.role,
		OutputFormat: opts.outputFormat,
	})
	if err != nil {
		return err
	}

	// Note the goal-only fallback only once generation has actually
	// succeeded - an invalid target/skill should just error, not also
	// claim a goal-only prompt was generated.
	if len(opts.skills) == 0 {
		fmt.Fprintln(cmd.ErrOrStderr(), "promptsmith: no --skills given; generating a goal-only prompt (interactive skill picker arrives in a later release)")
	}

	return deliver(cmd, opts, out)
}

// runInteractive launches the picker (seeded with whatever was already
// supplied via flags/args) and applies whatever the user chose to do
// with the result, through the same delivery helpers the flag-only path
// uses.
func runInteractive(cmd *cobra.Command, reg *registry.Registry, opts *generateOptions, goal string) error {
	result, err := runTUIFunc(reg, prompt.Inputs{
		Target:       opts.target,
		Skills:       opts.skills,
		Goal:         goal,
		Context:      opts.context,
		Constraints:  opts.constraints,
		Role:         opts.role,
		OutputFormat: opts.outputFormat,
	})
	if err != nil {
		return err
	}

	if result.Action == tui.ActionCancel {
		fmt.Fprintln(cmd.ErrOrStderr(), "promptsmith: canceled")
		return nil
	}

	out, err := prompt.Build(reg, result.Inputs)
	if err != nil {
		return err
	}

	switch result.Action {
	case tui.ActionCopy:
		return copyAndConfirm(cmd, out)
	case tui.ActionWrite:
		return writeFile(result.WritePath, out)
	default: // tui.ActionStdout
		cmd.Println(out)
		return nil
	}
}

// deliver routes the assembled prompt to every requested destination
// (file, clipboard), additively; if none were requested, it prints to
// stdout. This is the flag-only path's delivery model: unlike the TUI
// (which offers exactly one action per confirm), --copy and --out can
// both apply in the same invocation.
func deliver(cmd *cobra.Command, opts *generateOptions, out string) error {
	delivered := false

	if opts.out != "" {
		if err := writeFile(opts.out, out); err != nil {
			return err
		}
		delivered = true
	}

	if opts.toClipboard {
		if err := copyAndConfirm(cmd, out); err != nil {
			return err
		}
		delivered = true
	}

	if !delivered {
		cmd.Println(out)
	}
	return nil
}

// writeFile persists out to path with owner-only permissions: a
// generated prompt can embed --context/--constraints or a goal
// containing sensitive detail (paths, internal notes), so it's kept
// unreadable to other users (gosec G306). Shared by the flag-only and
// TUI delivery paths so the guarantee is identical either way.
func writeFile(path, out string) error {
	if err := os.WriteFile(path, []byte(out+"\n"), 0o600); err != nil {
		return fmt.Errorf("promptsmith: write %s: %w", path, err)
	}
	return nil
}

// copyAndConfirm copies out to the clipboard and confirms on stderr,
// keeping stdout clean for scripting/piping. Shared by the flag-only and
// TUI delivery paths.
func copyAndConfirm(cmd *cobra.Command, out string) error {
	if err := copyToClipboard(out); err != nil {
		return fmt.Errorf("promptsmith: copy to clipboard: %w", err)
	}
	fmt.Fprintln(cmd.ErrOrStderr(), "promptsmith: copied to clipboard")
	return nil
}
