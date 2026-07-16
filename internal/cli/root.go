// Package cli wires the promptsmith command surface: the root "generate"
// command plus the "list" and "validate" subcommands.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Execute runs the root command and exits the process on error.
func Execute() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// newRootCmd builds the promptsmith root command. The root command itself
// performs prompt generation (e.g. `promptsmith "fix the flaky test"`);
// "list" and "validate" are explicit subcommands.
//
// Generation is wired in a later phase (internal/assemble); for now the
// root command exists so the CLI surface, help text, and subcommands are
// in place and testable.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "promptsmith [flags] <goal>",
		Short: "Generate portable, skill-aware prompts for any LLM or agent harness",
		Long: `promptsmith assembles a deterministic, copy-paste prompt from a goal,
a set of methodology skills, and a target harness (generic, opencode, claude-code).

No LLM runs at generation time: the prompt is assembled from a built-in
registry of skills and a shared template with per-target deltas.`,
		Args:         cobra.ArbitraryArgs,
		SilenceUsage: true,
		RunE:         runGenerate,
	}

	root.AddCommand(newListCmd())
	root.AddCommand(newValidateCmd())

	return root
}
