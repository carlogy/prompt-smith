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

//go:build !empty

package registry

import (
	"embed"
	"io/fs"
)

// embeddedRaw is the registry data compiled into this binary: the
// canonical skill set by default. Built with `-tags empty` (see
// embed_empty.go), it's an empty scaffold instead - same categories and
// targets, no skills - for users who only want their own, via
// PROMPTSMITH_SKILLS_DIR (see userskills.go).
//
//go:embed data
var embeddedRaw embed.FS

// embeddedData returns the embedded registry's root as an fs.FS, ready
// for LoadFS. Load calls this; it's the one symbol embed_default.go and
// embed_empty.go must each provide, so Load itself never needs to know
// which build tag is active.
func embeddedData() (fs.FS, error) {
	return fs.Sub(embeddedRaw, "data")
}
