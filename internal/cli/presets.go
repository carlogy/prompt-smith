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

	"github.com/spf13/cobra"

	"github.com/carlogy/prompt-smith/internal/preset"
)

// noPresetsGuidance mirrors list.go's empty-skills-registry message
// (same tone: name the env var, the default path, the file layout, and
// point at the relevant README section) for the preset equivalent.
// Shared by runPresets' empty case and applyPreset's "no such preset,
// and none exist at all" case, so the wording only lives in one place.
const noPresetsGuidance = `no presets available. Create one via $PROMPTSMITH_PRESETS_DIR, or drop a <name>.yaml file into $XDG_CONFIG_HOME/promptsmith/presets (default ~/.config/promptsmith/presets). See the "Presets" section of README.md.`

// unknownPresetError builds the error applyPreset returns when
// preset.Load can't find the preset named name. It's pure (no I/O) so
// it's directly testable: names is whatever preset.ListDir already
// returned to the caller (which also owns printing ListDir's
// warnings).
//
// When other presets exist, the message names the bad preset and lists
// what IS available, e.g.:
//
//	promptsmith: unknown preset "web-reveiw"; available: standup, web-review (in /home/u/.config/promptsmith/presets)
//
// When none exist at all, it points at how to create one instead of
// listing an empty set.
func unknownPresetError(name string, names []string) error {
	if len(names) == 0 {
		return fmt.Errorf("promptsmith: unknown preset %q; %s", name, noPresetsGuidance)
	}
	dir, err := preset.Dir()
	if err != nil {
		return fmt.Errorf("promptsmith: unknown preset %q; available: %s", name, strings.Join(names, ", "))
	}
	return fmt.Errorf("promptsmith: unknown preset %q; available: %s (in %s)", name, strings.Join(names, ", "), dir)
}

// newPresetsCmd builds the "presets" subcommand: a plain list of saved
// preset names, one per line on stdout, so it composes with other
// tools (`promptsmith presets | fzf`, shell completion, etc.) instead
// of being decorated with a header or summary column.
func newPresetsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "presets",
		Short: "List saved presets (see -p/--preset)",
		Example: `  promptsmith presets
  promptsmith -p code-review "fix the bug"`,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPresets(cmd)
		},
	}
}

func runPresets(cmd *cobra.Command) error {
	names, warnings, err := preset.ListDir()
	if err != nil {
		return fmt.Errorf("promptsmith: %w", err)
	}
	for _, w := range warnings {
		fmt.Fprintln(cmd.ErrOrStderr(), "promptsmith: "+w)
	}

	if len(names) == 0 {
		fmt.Fprintln(cmd.ErrOrStderr(), "promptsmith: "+noPresetsGuidance)
		return nil
	}

	// The resolved directory is a hint on stderr, not part of the
	// stdout listing: stdout stays exactly one name per line so it
	// composes cleanly with other tools.
	if dir, err := preset.Dir(); err == nil {
		fmt.Fprintln(cmd.ErrOrStderr(), "promptsmith: presets directory: "+dir)
	}
	for _, name := range names {
		fmt.Fprintln(cmd.OutOrStdout(), name)
	}
	return nil
}
