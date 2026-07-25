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
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/carlogy/prompt-smith/internal/registry"
)

// newListCmd builds the "list" subcommand: skills grouped by category in
// canonical order, optionally filtered to those supported on a target.
func newListCmd(reg *registry.Registry) *cobra.Command {
	var target string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available skills by category and target",
		Example: `  promptsmith list
  promptsmith list -t claude-code`,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd, reg, target)
		},
	}
	cmd.Flags().StringVarP(&target, "target", "t", "", "filter to skills supported on this target")

	return cmd
}

func runList(cmd *cobra.Command, reg *registry.Registry, target string) error {
	if target != "" {
		if _, ok := reg.Targets[target]; !ok {
			return fmt.Errorf("promptsmith: unknown target %q", target)
		}
	}

	if len(reg.Skills) == 0 {
		fmt.Fprintln(cmd.ErrOrStderr(), "promptsmith: no skills available. Add your own via $PROMPTSMITH_SKILLS_DIR, or drop SKILL.md files into $XDG_CONFIG_HOME/promptsmith/skills (default ~/.config/promptsmith/skills), laid out as <category>/<skill-id>/SKILL.md (or <skill-id>/SKILL.md for the catch-all \"custom\" category). See the \"Custom skills\" section of README.md.")
		return nil
	}

	skills := append([]registry.Skill(nil), reg.Skills...)
	reg.SortSkills(skills)

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	lastCategory := ""
	shown := 0
	for _, sk := range skills {
		if target != "" && !reg.SupportsTarget(sk, target) {
			continue
		}
		shown++
		if sk.Category != lastCategory {
			if lastCategory != "" {
				fmt.Fprintln(w)
			}
			fmt.Fprintln(w, strings.ToUpper(sk.Category))
			lastCategory = sk.Category
		}
		fmt.Fprintf(w, "  %s\t%s\n", sk.ID, sk.WhenToUse)
	}
	if shown == 0 {
		fmt.Fprintf(cmd.ErrOrStderr(), "promptsmith: no skills support target %q. Run `promptsmith list` without -t to see all skills.\n", target)
		return nil
	}
	return w.Flush()
}
