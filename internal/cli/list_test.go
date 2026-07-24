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

func TestList_GroupsByCategoryInCanonicalOrder(t *testing.T) {
	reg := testRegistry(t)
	cmd := newListCmd(reg)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs(nil)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	out := stdout.String()
	// "planning" precedes "debugging" in the canonical category order;
	// their listed headers must appear in the same order.
	pi, di := strings.Index(out, "PLANNING"), strings.Index(out, "DEBUGGING")
	if pi < 0 || di < 0 || pi > di {
		t.Errorf("expected PLANNING before DEBUGGING, got:\n%s", out)
	}
	if !strings.Contains(out, "architect") {
		t.Errorf("expected architect to be listed, got:\n%s", out)
	}
}

func TestList_TargetFlagFiltersUnsupportedSkills(t *testing.T) {
	reg := &registry.Registry{
		Categories: []string{"debugging"},
		Skills: []registry.Skill{
			{ID: "diagnose", Category: "debugging", Body: "inline text"},
			{ID: "agent-only", Category: "debugging"}, // no generic body
		},
		Targets: map[string]registry.TargetConfig{
			"generic":  {ID: "generic", SkillMode: "inline"},
			"opencode": {ID: "opencode", SkillMode: "reference"},
		},
	}
	cmd := newListCmd(reg)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"-t", "generic"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "diagnose") {
		t.Errorf("expected diagnose (supported on generic) to be listed, got:\n%s", out)
	}
	if strings.Contains(out, "agent-only") {
		t.Errorf("expected agent-only to be filtered out for generic, got:\n%s", out)
	}
}

func TestList_UnknownTargetErrors(t *testing.T) {
	reg := testRegistry(t)
	cmd := newListCmd(reg)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"-t", "does-not-exist"})

	if err := cmd.Execute(); err == nil {
		t.Fatal("Execute() error = nil, want an error for an unknown target")
	}
}
