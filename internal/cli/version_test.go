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
	"bytes"
	"runtime/debug"
	"strings"
	"testing"
)

func TestFormatVersion_UsesMainVersionWhenPresent(t *testing.T) {
	// A real tag (go install module@v1.2.3) or Go's own auto-generated
	// pseudo-version (v0.0.0-<timestamp>-<hash>[+dirty]) already embeds
	// everything useful - trust it as-is rather than appending a
	// second, redundant revision/dirty suffix on top.
	info := &debug.BuildInfo{Main: debug.Module{Version: "v1.2.3"}}
	if got := formatVersion(info); got != "v1.2.3" {
		t.Errorf("formatVersion() = %q, want %q", got, "v1.2.3")
	}

	pseudo := &debug.BuildInfo{Main: debug.Module{Version: "v0.0.0-20260716222712-117c5b5923b5+dirty"}}
	if got := formatVersion(pseudo); got != pseudo.Main.Version {
		t.Errorf("formatVersion() = %q, want the pseudo-version unchanged: %q", got, pseudo.Main.Version)
	}
}

func TestFormatVersion_FallsBackToVCSRevisionWhenDevel(t *testing.T) {
	info := &debug.BuildInfo{
		Main: debug.Module{Version: "(devel)"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "abcdef1234567890"},
			{Key: "vcs.modified", Value: "true"},
		},
	}
	got := formatVersion(info)
	if !strings.Contains(got, "abcdef1") {
		t.Errorf("formatVersion() = %q, want it to include the short revision", got)
	}
	if !strings.Contains(got, "dirty") {
		t.Errorf("formatVersion() = %q, want it to flag a dirty tree", got)
	}
}

func TestFormatVersion_PlainDevelWhenNoVCSInfo(t *testing.T) {
	info := &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}
	if got := formatVersion(info); got != "(devel)" {
		t.Errorf("formatVersion() = %q, want %q", got, "(devel)")
	}
}

func TestBuildVersion_ReturnsNonEmptyString(t *testing.T) {
	got := buildVersion()
	if got == "" {
		t.Error(`buildVersion() = "", want a non-empty version string`)
	}
}

func TestBuildVersion_LdflagsOverrideWinsWhenSet(t *testing.T) {
	// Simulate GoReleaser's `-X .../internal/cli.version=v1.2.3` by
	// setting the package var directly, then restore it so we don't
	// leak state into other tests.
	old := version
	defer func() { version = old }()

	version = "v1.2.3"
	if got := buildVersion(); got != "v1.2.3" {
		t.Errorf("buildVersion() = %q, want the ldflags-set version %q", got, "v1.2.3")
	}
}

func TestBuildVersion_FallsBackToReadBuildInfoWhenUnset(t *testing.T) {
	old := version
	defer func() { version = old }()

	version = ""
	info, ok := debug.ReadBuildInfo()
	if !ok {
		t.Fatal("debug.ReadBuildInfo() ok = false, want true in a test binary")
	}
	want := formatVersion(info)
	if got := buildVersion(); got != want {
		t.Errorf("buildVersion() = %q, want the ReadBuildInfo fallback %q", got, want)
	}
}

func TestVersionFlagAndSubcommand_AgreeAndAreNonEmpty(t *testing.T) {
	run := func(args []string) string {
		reg := testRegistry(t)
		root := newRootCmd(reg)
		var stdout bytes.Buffer
		root.SetOut(&stdout)
		root.SetArgs(args)
		if err := root.Execute(); err != nil {
			t.Fatalf("Execute(%v) error = %v", args, err)
		}
		return stdout.String()
	}

	flagOut := run([]string{"--version"})
	subOut := run([]string{"version"})

	if flagOut == "" || subOut == "" {
		t.Fatalf("expected non-empty output, got flag=%q subcommand=%q", flagOut, subOut)
	}
	if !strings.Contains(flagOut, buildVersion()) || !strings.Contains(subOut, buildVersion()) {
		t.Errorf("expected both --version (%q) and the version subcommand (%q) to report the same version (%q)",
			flagOut, subOut, buildVersion())
	}
}

// TestVersionSubcommandAndFlag_GoRealStdoutNotStderr is a regression
// test for a bug where the "version" subcommand's payload print used
// cmd.Printf, which resolves via cobra's OutOrStderr() - stdout only
// if something already called SetOut, stderr otherwise. Production
// never calls SetOut, so `promptsmith version` printed to stderr, and
// `V=$(promptsmith version)` captured nothing.
//
// It deliberately does NOT call root.SetOut/SetErr: doing so would
// mask the bug rather than reproduce it (see captureRealStdio's
// comment). Cobra's own --version flag path already writes via
// OutOrStdout() (verified against the vendored cobra source), so it's
// included here as a same-test baseline the subcommand must match.
func TestVersionSubcommandAndFlag_GoRealStdoutNotStderr(t *testing.T) {
	run := func(args []string) (stdout, stderr string) {
		reg := testRegistry(t)
		root := newRootCmd(reg)
		root.SetArgs(args)

		var execErr error
		stdout, stderr = captureRealStdio(t, func() {
			execErr = root.Execute()
		})
		if execErr != nil {
			t.Fatalf("Execute(%v) error = %v, stderr = %s", args, execErr, stderr)
		}
		return stdout, stderr
	}

	subStdout, subStderr := run([]string{"version"})
	flagStdout, flagStderr := run([]string{"--version"})

	want := buildVersion()
	if !strings.Contains(subStdout, want) {
		t.Errorf("`version` real stdout missing %q, got:\n%s", want, subStdout)
	}
	if strings.Contains(subStderr, want) {
		t.Errorf("`version` leaked the version string onto real stderr:\n%s", subStderr)
	}
	if !strings.Contains(flagStdout, want) {
		t.Errorf("`--version` real stdout missing %q, got:\n%s", want, flagStdout)
	}
	if strings.Contains(flagStderr, want) {
		t.Errorf("`--version` leaked the version string onto real stderr:\n%s", flagStderr)
	}
}
