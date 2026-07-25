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
