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

	"github.com/carlogy/prompt-smith/internal/registry"
)

// testRegistry loads the real embedded registry for CLI-level tests. CLI
// tests exercise flag plumbing and output routing, not registry content,
// so using the real shipped data (already guarded by its own package's
// tests) avoids a third duplicate fixture.
//
// PROMPTSMITH_SKILLS_DIR is pinned to an empty temp directory so these
// tests stay hermetic regardless of the developer machine's real user
// skills directory (see registry.Load).
func testRegistry(t *testing.T) *registry.Registry {
	t.Helper()
	t.Setenv("PROMPTSMITH_SKILLS_DIR", t.TempDir())

	reg, warnings, err := registry.Load()
	if err != nil {
		t.Fatalf("registry.Load() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("registry.Load() warnings = %v, want none", warnings)
	}
	return reg
}
