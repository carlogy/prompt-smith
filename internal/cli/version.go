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
	"runtime/debug"

	"github.com/spf13/cobra"
)

// version is overridden at build time via ldflags, e.g.:
//
//	go build -ldflags "-X github.com/carlogy/prompt-smith/internal/cli.version=v1.2.3"
//
// GoReleaser sets this on every release build (see .goreleaser.yaml) so
// the printed version matches the git tag exactly, without depending on
// debug.ReadBuildInfo's pseudo-version heuristics. Left empty for
// `go build`/`go install` from a local checkout, where buildVersion
// falls back to formatVersion below.
var version string

// buildVersion returns the ldflags-injected version if set (see the
// package-level version var above - this is how GoReleaser stamps
// release builds with the exact git tag). Otherwise it falls back to
// deriving a version from the running binary's embedded build info,
// which works with both `go install module@version` and local builds
// from a git checkout with no ldflags at all.
func buildVersion() string {
	if version != "" {
		return version
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	return formatVersion(info)
}

// formatVersion is buildVersion's pure formatting logic, separated out
// so it's testable against synthetic debug.BuildInfo values instead of
// only whatever this test binary's own build happens to produce.
//
// A real tag (go install module@v1.2.3) or Go's own auto-generated
// pseudo-version (v0.0.0-<timestamp>-<hash>[+dirty]) already embeds
// everything useful in Main.Version - trust it as-is. Only fall back to
// reading raw VCS settings when Main.Version is empty or the generic
// "(devel)" placeholder (Go didn't derive anything useful on its own),
// to avoid reporting a redundant, duplicated revision/dirty suffix on
// top of what Go already embedded.
func formatVersion(info *debug.BuildInfo) string {
	if v := info.Main.Version; v != "" && v != "(devel)" {
		return v
	}

	var revision string
	var modified bool
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}

	if revision == "" {
		return "(devel)"
	}
	if len(revision) > 7 {
		revision = revision[:7]
	}
	if modified {
		return fmt.Sprintf("(devel) (%s, dirty)", revision)
	}
	return fmt.Sprintf("(devel) (%s)", revision)
}

// newVersionCmd builds the "version" subcommand. Prints in the same
// "promptsmith version X" format as cobra's built-in --version flag
// (enabled via newRootCmd's Version field), so both conventions people
// reach for agree.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the promptsmith version",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "promptsmith version %s\n", buildVersion())
		},
	}
}
