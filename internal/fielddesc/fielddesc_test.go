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

package fielddesc

import "testing"

// TestSentence_EveryKnownFieldHasOne guards completeness: every field
// constant must resolve to a non-empty sentence, so neither consuming
// surface (the web hint, the TUI footer) can end up silently rendering
// blank text for a field that exists but was never given copy.
func TestSentence_EveryKnownFieldHasOne(t *testing.T) {
	for _, field := range []string{Target, Goal, Role, Context, Constraints, OutputFormat} {
		if Sentence(field) == "" {
			t.Errorf("Sentence(%q) is empty - every known field constant must have a sentence", field)
		}
	}
}

func TestSentence_UnknownFieldReturnsEmpty(t *testing.T) {
	if got := Sentence("not-a-real-field"); got != "" {
		t.Errorf("Sentence(unknown) = %q, want empty", got)
	}
}
