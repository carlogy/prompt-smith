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

package prompt_test

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

// update regenerates .golden fixtures from current output. Run via
// `make update-golden` after an intentional behavior change.
var update = flag.Bool("update", false, "update .golden files")

// assertGolden compares got against testdata/<name>.golden, rewriting the
// fixture first when -update is passed.
func assertGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")

	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("update golden %s: %v", path, err)
		}
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run `make update-golden` if this is a new case)", path, err)
	}

	if got != string(want) {
		t.Errorf("%s mismatch:\n got:  %q\nwant:  %q", name, got, string(want))
	}
}
