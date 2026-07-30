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

// Package cli wires the promptsmith command surface: the root "generate"
// command plus the "list", "presets", and "validate" subcommands.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/carlogy/prompt-smith/internal/registry"
)

// Execute loads the registry, builds the command tree, and runs it,
// exiting the process on error. Non-fatal problems loading user skills
// (see registry.Load) are printed to stderr but don't stop execution.
func Execute() {
	reg, warnings, err := registry.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, w := range warnings {
		fmt.Fprintln(os.Stderr, "promptsmith: "+w)
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
a set of methodology skills, and a target harness (generic, opencode,
claude-code, gemini-cli, codex).

The goal may be passed positionally or via -g/--goal; the two are
mutually exclusive.

No LLM runs at generation time: the prompt is assembled from a built-in
registry of skills and per-target rendering rules.`,
		Example: `  promptsmith "fix the flaky checkout test"
  promptsmith -t opencode -s diagnose,verify "fix the flaky checkout test"
  promptsmith -t gemini-cli -s diagnose "fix the flaky checkout test"
  promptsmith -s diagnose -g "fix the flaky checkout test"  # goal via flag
  promptsmith -s diagnose -c "no new deps" "fix the bug"    # constraints
  promptsmith -s diagnose -y "fix the bug"                  # copy to clipboard
  promptsmith -p code-review "fix the bug"                  # reuse a saved preset
  promptsmith --tui                                         # interactive picker`,
		Version: buildVersion(), // enables the --version flag cobra provides automatically
		Args:    cobra.ArbitraryArgs,
		// We print errors ourselves in Execute (and tests read the
		// returned error directly), so don't let cobra double-print.
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	addGenerateFlags(root, reg)

	root.AddCommand(newListCmd(reg))
	root.AddCommand(newPresetsCmd())
	root.AddCommand(newValidateCmd(reg))
	root.AddCommand(newVersionCmd())

	return root
}
