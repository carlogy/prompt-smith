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
	"testing"

	"github.com/carlogy/prompt-smith/internal/prompt"
	"github.com/carlogy/prompt-smith/internal/registry"
	"github.com/carlogy/prompt-smith/internal/tui"
)

func TestDecideUseTUI(t *testing.T) {
	cases := []struct {
		name        string
		interactive bool
		quick       bool
		forceTUI    bool
		numSkills   int
		goalEmpty   bool
		want        bool
		wantErr     bool
	}{
		{"non-tty, bare -> skip", false, false, false, 0, false, false, false},
		{"tty, quick, bare -> skip (quick wins)", true, true, false, 0, false, false, false},
		{"tty, bare -> TUI", true, false, false, 0, false, true, false},
		{"tty, skills given, no force -> skip", true, false, false, 2, false, false, false},
		{"tty, skills given, forced -> TUI (pre-selected)", true, false, true, 2, false, true, false},
		{"quick + tui together -> error", true, true, true, 0, false, false, true},
		{"tui on non-tty -> error", false, false, true, 0, false, false, true},
		{"quick+tui error takes priority over the tty error", false, true, true, 0, false, false, true},
		{"tty, skills given, goal empty -> TUI (missing goal)", true, false, false, 2, true, true, false},
		{"tty, skills given, goal empty, quick -> skip (quick wins)", true, true, false, 2, true, false, false},
		{"non-tty, skills given, goal empty -> skip", false, false, false, 2, true, false, false},
		{"tty, no skills, goal empty -> TUI", true, false, false, 0, true, true, false},
		{"tty, skills given, goal present -> skip", true, false, false, 2, false, false, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := decideUseTUI(tc.interactive, tc.quick, tc.forceTUI, tc.numSkills, tc.goalEmpty)
			if tc.wantErr {
				if err == nil {
					t.Fatal("decideUseTUI() error = nil, want an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("decideUseTUI() error = %v, want nil", err)
			}
			if got != tc.want {
				t.Errorf("decideUseTUI() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDecideUseTUI_ErrorMessages(t *testing.T) {
	_, err := decideUseTUI(true, true, true, 0, false)
	if err == nil {
		t.Fatal("expected an error for --quick + --tui")
	}
}

// TestGenerate_PresetWithNoGoalLaunchesTUI covers decideUseTUI's third
// reason to open the picker (goalEmpty) end-to-end: a preset supplies
// skills, so numSkills != 0, isolating goalEmpty as the only reason the
// picker fires here rather than the pre-existing "no skills" reason.
func TestGenerate_PresetWithNoGoalLaunchesTUI(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PROMPTSMITH_PRESETS_DIR", dir)
	writePreset(t, dir, "mypreset", "skills:\n  - diagnose\n")

	defer stubInteractive(t, true)()
	called := false
	defer stubRunTUI(t, func(reg *registry.Registry, in prompt.Inputs, _ []string) (tui.Result, error) {
		called = true
		if in.Goal != "" {
			t.Errorf("initial goal passed to the TUI = %q, want empty", in.Goal)
		}
		return tui.Result{Inputs: in, Action: tui.ActionCancel}, nil
	})()

	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"-p", "mypreset"}) // no goal arg

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
	}
	if !called {
		t.Fatal("expected a preset with no goal to launch the TUI (runTUIFunc was not called)")
	}
}
