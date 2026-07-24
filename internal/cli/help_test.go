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
)

func TestHelp_RootIncludesExamples(t *testing.T) {
	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetArgs([]string{"--help"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "Examples:") {
		t.Errorf("expected --help to include an Examples: section, got:\n%s", got)
	}
	if !strings.Contains(got, `promptsmith "fix`) {
		t.Errorf("expected a sample goal invocation, got:\n%s", got)
	}
}

func TestHelp_ListIncludesExamples(t *testing.T) {
	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetArgs([]string{"list", "--help"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "Examples:") {
		t.Errorf("expected list --help to include an Examples: section, got:\n%s", got)
	}
}

func TestHelp_ValidateIncludesExamples(t *testing.T) {
	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetArgs([]string{"validate", "--help"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "Examples:") {
		t.Errorf("expected validate --help to include an Examples: section, got:\n%s", got)
	}
}
