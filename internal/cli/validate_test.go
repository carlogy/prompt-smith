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
	"strings"
	"testing"

	"github.com/carlogy/prompt-smith/internal/registry"
)

func TestValidate_WellFormedRegistryPrintsOK(t *testing.T) {
	reg := testRegistry(t)
	cmd := newValidateCmd(reg)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(stdout.String(), "ok") {
		t.Errorf("expected an ok confirmation, got:\n%s", stdout.String())
	}
}

func TestValidate_InvalidRegistryErrors(t *testing.T) {
	reg := &registry.Registry{
		Categories: []string{"debugging"},
		Skills: []registry.Skill{
			{ID: "diagnose", Category: "nonexistent-category", Body: "text"},
		},
		Targets: map[string]registry.TargetConfig{
			"generic": {ID: "generic", SkillMode: "inline"},
		},
	}
	cmd := newValidateCmd(reg)
	cmd.SetOut(&bytes.Buffer{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want an error for a dangling category reference")
	}
	if !strings.Contains(err.Error(), "nonexistent-category") {
		t.Errorf("expected the error to name the offending category, got: %v", err)
	}
}

// TestValidate_OKGoesToRealStdoutNotStderr is a regression test for a
// bug where the "ok" confirmation used cmd.Println, which resolves via
// cobra's OutOrStderr() - stdout only if something already called
// SetOut, stderr otherwise. Production never calls SetOut, so
// `promptsmith validate` printed its success confirmation to stderr.
//
// It deliberately does NOT call cmd.SetOut/SetErr: doing so would mask
// the bug rather than reproduce it (see captureRealStdio's comment).
func TestValidate_OKGoesToRealStdoutNotStderr(t *testing.T) {
	reg := testRegistry(t)
	cmd := newValidateCmd(reg)

	var execErr error
	stdout, stderr := captureRealStdio(t, func() {
		execErr = cmd.Execute()
	})

	if execErr != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", execErr, stderr)
	}
	if !strings.Contains(stdout, "ok") {
		t.Errorf("real stdout missing the ok confirmation, got:\n%s", stdout)
	}
	if strings.Contains(stderr, "ok") {
		t.Errorf("the ok confirmation leaked onto real stderr:\n%s", stderr)
	}
}

// TestValidate_NoWarningsInContextPrintsOK pins the no-warnings branch
// explicitly (TestValidate_WellFormedRegistryPrintsOK above covers the
// same thing implicitly, since it never sets a context at all - here
// the context is set via withWarnings with an empty slice, the same
// way run() sets it when registry.Load reported nothing).
func TestValidate_NoWarningsInContextPrintsOK(t *testing.T) {
	reg := testRegistry(t)
	cmd := newValidateCmd(reg)
	cmd.SetContext(withWarnings(context.Background(), nil))
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(stdout.String(), "ok") {
		t.Errorf("expected an ok confirmation, got:\n%s", stdout.String())
	}
}

// TestValidate_WarningsInContextErrors is the unit-level counterpart to
// TestRun_ValidateFailsOnDroppedUserSkill below: it drives newValidateCmd
// directly with warnings injected via withWarnings (mirroring exactly
// what run() does before Execute), rather than through a real
// registry.Load + PROMPTSMITH_SKILLS_DIR round trip.
func TestValidate_WarningsInContextErrors(t *testing.T) {
	reg := testRegistry(t)
	cmd := newValidateCmd(reg)
	cmd.SetContext(withWarnings(context.Background(), []string{
		"skip testing/broken/SKILL.md: missing frontmatter",
		"skip testing/dup/SKILL.md: duplicate user skill id \"standalone\"",
	}))
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want an error when load warnings are present")
	}
	if strings.Contains(stdout.String(), "registry ok") {
		t.Errorf("stdout = %q, want no ok confirmation when warnings are present", stdout.String())
	}
	if !strings.Contains(err.Error(), "2") {
		t.Errorf("error = %v, want it to mention the warning count (2)", err)
	}
}

// TestRun_ValidateFailsOnDroppedUserSkill is the end-to-end counterpart
// to TestRun_WarningsPrintAfterCommandSucceeds (root_test.go): it drives
// the real run() entry point - registry.Load, not a test fixture -
// through to "validate" with a genuinely malformed user skill on disk,
// confirming the exit code, that the warning reaches real stderr, and
// that "registry ok" never reaches stdout.
func TestRun_ValidateFailsOnDroppedUserSkill(t *testing.T) {
	t.Setenv("PROMPTSMITH_SKILLS_DIR", t.TempDir())
	dir := os.Getenv("PROMPTSMITH_SKILLS_DIR")
	writeMalformedUserSkill(t, dir)

	var stdout, stderr bytes.Buffer
	code := run(&stdout, &stderr, []string{"validate"})

	if code != 1 {
		t.Fatalf("run() = %d, want 1 (a dropped user skill should fail validate), stderr = %s", code, stderr.String())
	}
	const wantWarning = `skip testing/broken/SKILL.md: missing frontmatter: expected the file to start with "---"`
	if !strings.Contains(stderr.String(), wantWarning) {
		t.Errorf("stderr = %q, want it to contain the malformed-skill warning %q", stderr.String(), wantWarning)
	}
	if strings.Contains(stdout.String(), "registry ok") {
		t.Errorf("stdout = %q, want no ok confirmation when a user skill was dropped", stdout.String())
	}
}
