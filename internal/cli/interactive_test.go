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
	"testing"
)

func TestDecideUseTUI(t *testing.T) {
	cases := []struct {
		name        string
		interactive bool
		quick       bool
		forceTUI    bool
		numSkills   int
		want        bool
		wantErr     bool
	}{
		{"non-tty, bare -> skip", false, false, false, 0, false, false},
		{"tty, quick, bare -> skip (quick wins)", true, true, false, 0, false, false},
		{"tty, bare -> TUI", true, false, false, 0, true, false},
		{"tty, skills given, no force -> skip", true, false, false, 2, false, false},
		{"tty, skills given, forced -> TUI (pre-selected)", true, false, true, 2, true, false},
		{"quick + tui together -> error", true, true, true, 0, false, true},
		{"tui on non-tty -> error", false, false, true, 0, false, true},
		{"quick+tui error takes priority over the tty error", false, true, true, 0, false, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := decideUseTUI(tc.interactive, tc.quick, tc.forceTUI, tc.numSkills)
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
	_, err := decideUseTUI(true, true, true, 0)
	if err == nil {
		t.Fatal("expected an error for --quick + --tui")
	}
}
