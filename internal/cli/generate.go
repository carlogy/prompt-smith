package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/spf13/cobra"

	"github.com/carlogy/prompt-smith/internal/prompt"
	"github.com/carlogy/prompt-smith/internal/registry"
	"github.com/carlogy/prompt-smith/internal/server"
	"github.com/carlogy/prompt-smith/internal/tui"
)

// copyToClipboard puts text on the system clipboard. A package variable
// so tests can substitute a spy instead of touching the real clipboard.
var copyToClipboard = clipboard.WriteAll

// runTUIFunc launches the interactive skill picker. A package variable
// so tests can substitute a spy instead of starting a real Bubble Tea
// program (which would block reading real stdin).
var runTUIFunc = tui.Run

// runServerFunc launches the local web UI (see --ui) and blocks until
// ctx is done. A package variable, same reasoning as runTUIFunc: tests
// substitute a spy so they never bind a real port or open a browser.
var runServerFunc = server.Serve

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
	ui           bool
	port         int
	noBrowser    bool
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
	cmd.Flags().BoolVar(&opts.ui, "ui", false, "launch the local web UI in your browser")
	cmd.Flags().IntVar(&opts.port, "port", 0, "port for --ui to bind (default: an OS-assigned free port)")
	cmd.Flags().BoolVar(&opts.noBrowser, "no-browser", false, "with --ui, don't automatically open a browser")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runGenerate(cmd, reg, opts, args)
	}
}

func runGenerate(cmd *cobra.Command, reg *registry.Registry, opts *generateOptions, args []string) error {
	if err := validateUIFlags(cmd, opts); err != nil {
		return err
	}
	if opts.ui {
		return runUI(cmd, reg, opts)
	}

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

// validateUIFlags enforces --ui's flag relationships: --port and
// --no-browser only make sense alongside --ui, and --ui itself is
// mutually exclusive with the other ways of choosing what happens to
// the generated prompt (--tui: a different interactive mode; --quick:
// explicitly asks to skip any interactive mode; --copy/--out: the web
// UI decides delivery itself - browser copy/download - so a
// server-side delivery flag has nothing to act on).
func validateUIFlags(cmd *cobra.Command, opts *generateOptions) error {
	if !opts.ui {
		if cmd.Flags().Changed("port") {
			return errors.New("promptsmith: --port requires --ui")
		}
		if cmd.Flags().Changed("no-browser") {
			return errors.New("promptsmith: --no-browser requires --ui")
		}
		return nil
	}

	switch {
	case opts.tui:
		return errors.New("promptsmith: --ui and --tui are mutually exclusive")
	case opts.quick:
		return errors.New("promptsmith: --ui and --quick are mutually exclusive")
	case opts.toClipboard:
		return errors.New("promptsmith: --ui and --copy are mutually exclusive")
	case opts.out != "":
		return errors.New("promptsmith: --ui and --out are mutually exclusive")
	}
	return nil
}

// runUI launches the local web UI and blocks until it's interrupted
// (Ctrl-C) or otherwise stops. Unlike the TUI, --ui doesn't require an
// interactive terminal: "open a browser" doesn't depend on the calling
// process's own stdio, so it works just as well from a script.
//
// signal.NotifyContext lives here, not in the server package: Serve
// takes a plain context.Context so it can be shut down deterministically
// in a test (a context.WithCancel, not a real OS signal, which would
// affect the whole test process).
func runUI(cmd *cobra.Command, reg *registry.Registry, opts *generateOptions) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	return runServerFunc(ctx, reg, server.Options{
		Port:      opts.port,
		NoBrowser: opts.noBrowser,
		Stdout:    cmd.OutOrStdout(),
	})
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
//
// path may use "~" shorthand (expanded via expandPath) and may name
// directories that don't exist yet (created via MkdirAll, also
// owner-only). An existing file at path is overwritten silently, same
// as a shell redirect would.
func writeFile(path, out string) error {
	expanded, err := expandPath(path)
	if err != nil {
		return err
	}

	if dir := filepath.Dir(expanded); dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("promptsmith: create directory %s: %w", dir, err)
		}
	}

	if err := os.WriteFile(expanded, []byte(out+"\n"), 0o600); err != nil {
		return fmt.Errorf("promptsmith: write %s: %w", expanded, err)
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
