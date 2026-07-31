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
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/carlogy/prompt-smith/internal/registry"
	"github.com/carlogy/prompt-smith/internal/server"
)

func TestNewRootCmd_HasExpectedSubcommands(t *testing.T) {
	reg := testRegistry(t)
	root := newRootCmd(reg)

	want := map[string]bool{"list": false, "presets": false, "validate": false}
	for _, c := range root.Commands() {
		if _, ok := want[c.Name()]; ok {
			want[c.Name()] = true
		}
	}

	for name, found := range want {
		if !found {
			t.Errorf("expected subcommand %q to be registered", name)
		}
	}
}

// writeMalformedUserSkill drops a single malformed SKILL.md into dir,
// laid out the same way TestLoadUserSkills's "malformed frontmatter"
// case exercises loadUserSkills directly (see
// internal/registry/userskills_test.go) - a file that doesn't even
// start with the "---" frontmatter delimiter. loadUserSkills treats
// this as a non-fatal, reportable warning rather than a load failure,
// which is exactly the case run's warnings-after-Execute reordering
// (see root.go) needs a real, end-to-end reproduction of.
func writeMalformedUserSkill(t *testing.T, dir string) {
	t.Helper()
	skillDir := filepath.Join(dir, "testing", "broken")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", skillDir, err)
	}
	path := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(path, []byte("not frontmatter at all"), 0o600); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}

// TestRun_WarningsPrintAfterCommandSucceeds pins the actual behavior
// change this unit makes: a registry.Load warning reaches stderr, but
// only once the command tree has already finished running - not before
// it starts, which is what let the interactive picker's alt-screen
// session swallow it in the original ordering (see run's doc comment
// in root.go). "list" is used as the driven subcommand rather than a
// generation because it's a plain, always-succeeds, no-TTY-required
// path that doesn't itself write anything resembling the warning text
// this test looks for, so there's no risk of a false match.
func TestRun_WarningsPrintAfterCommandSucceeds(t *testing.T) {
	t.Setenv("PROMPTSMITH_SKILLS_DIR", t.TempDir())
	dir := os.Getenv("PROMPTSMITH_SKILLS_DIR")
	writeMalformedUserSkill(t, dir)

	var stdout, stderr bytes.Buffer
	code := run(&stdout, &stderr, []string{"list"})

	if code != 0 {
		t.Fatalf("run() = %d, want 0 (the command itself should still succeed), stderr = %s", code, stderr.String())
	}
	const wantWarning = `promptsmith: skip testing/broken/SKILL.md: missing frontmatter: expected the file to start with "---"`
	if !strings.Contains(stderr.String(), wantWarning) {
		t.Errorf("stderr = %q, want it to contain the malformed-skill warning %q", stderr.String(), wantWarning)
	}
}

// TestRun_WarningsPrintBeforeTerminalError covers the other gotcha
// this unit's spec calls out explicitly: the original code's
// os.Exit(1) on a command failure would have skipped a warnings loop
// placed after it, so warnings have to print on the ERROR exit path
// too, not just the success one above - and specifically BEFORE the
// terminal error, so the more urgent, more actionable message ends up
// last, closest to the user's cursor (see run's doc comment).
func TestRun_WarningsPrintBeforeTerminalError(t *testing.T) {
	t.Setenv("PROMPTSMITH_SKILLS_DIR", t.TempDir())
	dir := os.Getenv("PROMPTSMITH_SKILLS_DIR")
	writeMalformedUserSkill(t, dir)

	var stdout, stderr bytes.Buffer
	// An unknown target fails deterministically without a TTY or any
	// other seam to stub - see TestGenerate_UnknownTargetWithNoSkills_
	// ErrorsWithoutGoalOnlyNote in generate_test.go for the same
	// invocation shape used as a plain non-interactive failure.
	code := run(&stdout, &stderr, []string{"-t", "does-not-exist", "goal"})

	if code != 1 {
		t.Fatalf("run() = %d, want 1 (the command should fail on the unknown target), stderr = %s", code, stderr.String())
	}

	const wantWarning = `promptsmith: skip testing/broken/SKILL.md`
	const wantError = `unknown target "does-not-exist"`
	warningAt := strings.Index(stderr.String(), wantWarning)
	errorAt := strings.Index(stderr.String(), wantError)
	if warningAt == -1 {
		t.Fatalf("stderr = %q, want it to contain the malformed-skill warning", stderr.String())
	}
	if errorAt == -1 {
		t.Fatalf("stderr = %q, want it to contain the unknown-target error", stderr.String())
	}
	if warningAt > errorAt {
		t.Errorf("warning printed at byte %d, error at byte %d - want the warning BEFORE the terminal error", warningAt, errorAt)
	}
}

// TestRun_UIThreadsWarningsIntoServerOptions is the end-to-end
// counterpart to TestRun_WarningsPrintAfterCommandSucceeds above: it
// drives the real run() entry point (registry.Load, not a test
// fixture) through to --ui and confirms the same malformed-skill
// warning that reaches stderr on every other path also reaches
// server.Options.Warnings here - proving the full chain run ->
// withWarnings -> newRootCmd(reg).Execute() -> runGenerate -> runUI ->
// warningsFromContext -> server.Options, not just one hop of it.
func TestRun_UIThreadsWarningsIntoServerOptions(t *testing.T) {
	t.Setenv("PROMPTSMITH_SKILLS_DIR", t.TempDir())
	dir := os.Getenv("PROMPTSMITH_SKILLS_DIR")
	writeMalformedUserSkill(t, dir)

	var gotOpts server.Options
	defer stubRunServer(t, func(ctx context.Context, reg *registry.Registry, opts server.Options) error {
		gotOpts = opts
		return nil
	})()

	var stdout, stderr bytes.Buffer
	code := run(&stdout, &stderr, []string{"--ui"})

	if code != 0 {
		t.Fatalf("run() = %d, want 0, stderr = %s", code, stderr.String())
	}
	const wantWarning = `skip testing/broken/SKILL.md: missing frontmatter: expected the file to start with "---"`
	found := false
	for _, w := range gotOpts.Warnings {
		if strings.Contains(w, wantWarning) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("server.Options.Warnings = %v, want one containing %q", gotOpts.Warnings, wantWarning)
	}
}
