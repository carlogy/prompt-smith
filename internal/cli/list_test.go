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
	// The canonical category order is planning, research, coding,
	// debugging, testing, review, git, safety, communication, learning;
	// their listed headers must appear in that exact order.
	headers := []string{
		"PLANNING", "RESEARCH", "CODING", "DEBUGGING", "TESTING", "REVIEW",
		"GIT", "SAFETY", "COMMUNICATION", "LEARNING",
	}
	prevIdx := -1
	for _, h := range headers {
		idx := strings.Index(out, h)
		if idx < 0 || idx < prevIdx {
			t.Fatalf("expected category headers in canonical order %v, got:\n%s", headers, out)
		}
		prevIdx = idx
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

func TestList_NoSkillsAtAll_PrintsGuidanceOnStderr(t *testing.T) {
	reg := &registry.Registry{
		Categories: []string{"debugging"},
		Skills:     nil,
		Targets: map[string]registry.TargetConfig{
			"generic": {ID: "generic", SkillMode: "inline"},
		},
	}
	cmd := newListCmd(reg)
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(nil)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if stdout.String() != "" {
		t.Errorf("stdout = %q, want empty (no table content)", stdout.String())
	}
	errOut := stderr.String()
	if !strings.Contains(errOut, "PROMPTSMITH_SKILLS_DIR") {
		t.Errorf("stderr = %q, want mention of PROMPTSMITH_SKILLS_DIR", errOut)
	}
	if !strings.Contains(errOut, "SKILL.md") {
		t.Errorf("stderr = %q, want mention of SKILL.md layout", errOut)
	}
}

func TestList_TargetExcludesAllSkills_PrintsTargetSpecificMessage(t *testing.T) {
	reg := &registry.Registry{
		Categories: []string{"debugging"},
		Skills: []registry.Skill{
			{ID: "agent-only", Category: "debugging"}, // no generic body
		},
		Targets: map[string]registry.TargetConfig{
			"generic":  {ID: "generic", SkillMode: "inline"},
			"opencode": {ID: "opencode", SkillMode: "reference"},
		},
	}
	cmd := newListCmd(reg)
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"-t", "generic"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if stdout.String() != "" {
		t.Errorf("stdout = %q, want empty (no table content)", stdout.String())
	}
	errOut := stderr.String()
	if !strings.Contains(errOut, "generic") {
		t.Errorf("stderr = %q, want mention of the requested target", errOut)
	}
	if !strings.Contains(errOut, "promptsmith list") {
		t.Errorf("stderr = %q, want suggestion to run `promptsmith list` without -t", errOut)
	}
	if strings.Contains(errOut, "PROMPTSMITH_SKILLS_DIR") {
		t.Errorf("stderr = %q, should not suggest configuring a skills directory when skills already exist", errOut)
	}
}

func TestList_NonEmptyUnfiltered_NoEmptyStateMessage(t *testing.T) {
	reg := testRegistry(t)
	cmd := newListCmd(reg)
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(nil)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if stderr.String() != "" {
		t.Errorf("stderr = %q, want empty on the non-empty unfiltered path", stderr.String())
	}
	if !strings.Contains(stdout.String(), "architect") {
		t.Errorf("stdout missing expected skill listing:\n%s", stdout.String())
	}
}
