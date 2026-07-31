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
	"context"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/carlogy/prompt-smith/internal/registry"
)

// warningsContextKey is the context key run uses to make
// registry.Load's warnings reachable from runUI (see generate.go)
// without widening newRootCmd's or addGenerateFlags's signature.
// Both are exercised directly by roughly ninety call sites across this
// package's tests - every one of them builds a root command via
// newRootCmd(reg) alone and none of them care about warnings - so
// adding a parameter there would force every one of them to change
// just to satisfy a mechanism only the --ui path needs (see runUI's
// doc comment for what that path actually does with them). Cobra
// already threads a context.Context through the whole command tree
// (Command.Context/SetContext), so run stores warnings on it once,
// here, right before Execute, and runUI reads them back via
// warningsFromContext.
type warningsContextKey struct{}

// withWarnings returns ctx carrying warnings, readable back via
// warningsFromContext.
func withWarnings(ctx context.Context, warnings []string) context.Context {
	return context.WithValue(ctx, warningsContextKey{}, warnings)
}

// warningsFromContext returns the warnings withWarnings stored on ctx,
// or nil if none were stored - which is every test in this package
// that builds its own root command via newRootCmd and calls Execute()
// on it directly without ever calling withWarnings/SetContext (see
// warningsContextKey's doc comment). cobra defaults a command's
// context to context.Background() when Execute runs if nothing set
// one, so ctx.Value here is always safe to call, never a nil-pointer
// risk.
func warningsFromContext(ctx context.Context) []string {
	warnings, _ := ctx.Value(warningsContextKey{}).([]string)
	return warnings
}

// Execute loads the registry, builds the command tree, and runs it,
// exiting the process with a non-zero status on error. It's a thin
// wrapper around run (below) so the actual behavior is testable without
// a real os.Exit call - see run's doc comment for the warnings-timing
// decision that's the whole reason run exists as a separate function.
func Execute() {
	os.Exit(run(os.Stdout, os.Stderr, os.Args[1:]))
}

// run is Execute's testable core: explicit writers and args in, an
// exit code out, no os.Exit anywhere inside it. Nothing exercised this
// path before (every test in this package builds its own root command
// via newRootCmd and calls Execute() on THAT directly, bypassing
// registry.Load and this function entirely), so pulling the warnings
// print out of Execute without also pulling it out into something a
// test can call would have made the new ordering unverifiable.
//
// Non-fatal problems loading user skills (see registry.Load) are
// printed to stderr, but only AFTER the command tree finishes running,
// not before it starts - a deliberate change from the original
// ordering. The interactive picker opens an alt-screen session
// (tea.WithAltScreen, see internal/tui/tui.go) that most terminals
// don't restore scrollback across, so a warning printed before the
// picker launches is gone for good the moment it appears; by the time
// Execute used to print warnings (right after registry.Load, well
// before newRootCmd(reg).Execute() ever runs), a malformed skill in
// PROMPTSMITH_SKILLS_DIR was already effectively invisible to anyone
// who ended up in the picker or the --ui web server. Printing after
// Execute returns means the warning is the last thing left on screen
// once whichever mode the user ran - picker, web UI, or a plain
// non-interactive generation - has finished.
//
// Warnings print BEFORE the terminal error (if any), not after: a
// command failure is the more urgent, more likely-to-be-acted-on
// message of the two, so it belongs last, closest to the user's
// cursor, rather than buried under a warning about something that
// (being non-fatal) probably isn't what they're currently debugging.
// This also means the error branch below can't just os.Exit(1)
// immediately the way the original code did right after
// newRootCmd(reg).Execute()'s own error check - that would swallow
// every warning printed above it. Returning an int instead of exiting
// keeps both prints reachable on the same path. (registry.Load's own
// error return - the other os.Exit(1) below - is unaffected: every
// return statement in registry.Load that carries a non-nil error also
// carries a nil warnings slice, so there's nothing to swallow there.)
func run(stdout, stderr io.Writer, args []string) int {
	reg, warnings, err := registry.Load()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	root := newRootCmd(reg)
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetArgs(args)
	// See warningsContextKey's doc comment for why this rides the
	// command's own context rather than a new parameter threaded
	// through newRootCmd: --ui (runUI, in generate.go) is the only
	// thing downstream that ever reads it back.
	root.SetContext(withWarnings(context.Background(), warnings))
	cmdErr := root.Execute()

	for _, w := range warnings {
		fmt.Fprintln(stderr, "promptsmith: "+w)
	}

	if cmdErr != nil {
		fmt.Fprintln(stderr, cmdErr)
		return 1
	}
	return 0
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
  promptsmith -s diagnose -c "no new deps" "fix the flaky checkout test"    # constraints
  promptsmith -s diagnose -y "fix the flaky checkout test"                  # copy to clipboard
  promptsmith -p code-review "fix the flaky checkout test"                  # reuse a saved preset
  promptsmith --tui                                         # interactive picker`,
		Version: buildVersion(), // enables the --version flag cobra provides automatically
		Args:    cobra.ArbitraryArgs,
		// We print errors ourselves (in run, or read directly by
		// tests that call Execute() on the command they built), so
		// don't let cobra double-print.
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
