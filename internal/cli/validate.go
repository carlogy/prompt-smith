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

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/carlogy/prompt-smith/internal/registry"
)

// newValidateCmd builds the "validate" subcommand: checks the merged
// registry - embedded skills plus any user skills from
// $PROMPTSMITH_SKILLS_DIR - the binary would actually use.
//
// reg.Validate() alone (duplicate ids, dangling categories/refs) can't
// catch the failure mode this command exists for: registry.Load reports
// a malformed, unreadable, or duplicate-id user skill as a non-fatal
// warning and skips it - the right call on the generate path, where a
// single bad drop-in shouldn't take down the whole CLI, but exactly
// backwards for a command named "validate", whose entire job is telling
// a user their skill didn't make it in. So RunE also reads the load
// warnings back off the command's context (see warningsFromContext in
// root.go) and fails if there are any, even though reg.Validate() found
// nothing wrong with what DID load.
func newValidateCmd(reg *registry.Registry) *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate the merged skill registry (embedded + user skills)",
		Long: `validate checks the registry the binary would actually use: the
embedded skills merged with any user-provided skills from
$PROMPTSMITH_SKILLS_DIR (or the XDG default, ~/.config/promptsmith/skills).

Two kinds of problems fail it:
  - structural: duplicate skill ids, a skill whose category isn't
    declared, or a ref naming an unknown target
  - anything registry.Load only warned about while merging in user
    skills - a missing or unreadable SKILL.md, or a duplicate id -
    which the generate path treats as non-fatal and skips, but which
    validate treats as a failure`,
		Example:       `  promptsmith validate`,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := reg.Validate(); err != nil {
				return err
			}
			if warnings := warningsFromContext(cmd.Context()); len(warnings) > 0 {
				// run() (root.go) prints these same warnings to stderr
				// itself, after the command tree returns but before
				// this error - so by the time a user sees this
				// message, the warnings it refers to are already
				// sitting just above it. Restating their text here
				// would just duplicate what's already on screen.
				return fmt.Errorf("registry validation failed: %d problem(s) loading user skills (see the warning(s) above)", len(warnings))
			}
			fmt.Fprintln(cmd.OutOrStdout(), "registry ok")
			return nil
		},
	}
}
