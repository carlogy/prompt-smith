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
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

// expandPath resolves a leading "~" (the calling user's home directory)
// or "~name" (that user's home directory) in path, the same shorthand a
// shell would expand. It's a no-op for any path that doesn't start with
// "~" (absolute, relative, or empty). Shared by the flag-only (--out)
// and TUI save-path delivery paths so both accept the same shorthand.
func expandPath(path string) (string, error) {
	if path == "" || path[0] != '~' {
		return path, nil
	}

	rest := path[1:]
	if rest == "" || rest[0] == '/' {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("promptsmith: resolve home directory: %w", err)
		}
		return filepath.Join(home, rest), nil
	}

	name, tail, _ := strings.Cut(rest, "/")
	u, err := user.Lookup(name)
	if err != nil {
		return "", fmt.Errorf("promptsmith: resolve home directory for %q: %w", name, err)
	}
	return filepath.Join(u.HomeDir, tail), nil
}
