// Package cli wires the promptsmith command surface: the root "generate"
// command plus the "list" and "validate" subcommands.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/carlogy/prompt-smith/internal/registry"
)

// Execute loads the registry, builds the command tree, and runs it,
// exiting the process on error.
func Execute() {
	reg, err := registry.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := newRootCmd(reg).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// newRootCmd builds the promptsmith root command. The root command itself
// performs prompt generation (e.g. `promptsmith "fix the flaky test"`);
// "list" and "validate" are explicit subcommands.
func newRootCmd(reg *registry.Registry) *cobra.Command {
	root := &cobra.Command{
		Use:   "promptsmith [flags] <goal>",
		Short: "Generate portable, skill-aware prompts for any LLM or agent harness",
		Long: `promptsmith assembles a deterministic, copy-paste prompt from a goal,
a set of methodology skills, and a target harness (generic, opencode, claude-code).

No LLM runs at generation time: the prompt is assembled from a built-in
registry of skills and per-target rendering rules.`,
		Example: `  promptsmith "fix the flaky checkout test"
  promptsmith -t opencode -s diagnose,verify "fix the flaky checkout test"
  promptsmith -s diagnose -c "fix the bug"          # copy to clipboard
  promptsmith --tui                                 # interactive picker`,
		Version: buildVersion(), // enables the --version flag cobra provides automatically
		Args:    cobra.ArbitraryArgs,
		// We print errors ourselves in Execute (and tests read the
		// returned error directly), so don't let cobra double-print.
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	addGenerateFlags(root, reg)

	root.AddCommand(newListCmd(reg))
	root.AddCommand(newValidateCmd(reg))
	root.AddCommand(newVersionCmd())

	return root
}
