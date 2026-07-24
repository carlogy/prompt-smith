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

// Package naming builds a suggested filename for a saved/downloaded
// prompt, from a goal and a timestamp. Shared by the TUI's save-file
// prompt (internal/tui) and the web UI's Download button
// (internal/server), so both surfaces agree on what "obvious default
// name" means for the same goal.
package naming

import (
	"strings"
	"time"
)

const (
	maxSlugWords = 6
	maxSlugLen   = 50
)

// SuggestFilename builds a default save-file name from a goal and a
// timestamp: promptsmith-<UTC-timestamp>-<goal-slug>.txt. The slug is
// lowercased, non-alphanumeric runs collapse to a single "-", and it's
// capped at maxSlugWords words / maxSlugLen characters. An empty (or
// whitespace-only) goal falls back to the timestamp alone.
func SuggestFilename(goal string, t time.Time) string {
	ts := t.UTC().Format("20060102T150405Z")

	slug := slugify(goal)
	if slug == "" {
		return "promptsmith-" + ts + ".txt"
	}
	return "promptsmith-" + ts + "-" + slug + ".txt"
}

func slugify(s string) string {
	lower := strings.ToLower(s)

	var b strings.Builder
	for _, r := range lower {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteRune(' ')
		}
	}

	words := strings.Fields(b.String())
	if len(words) > maxSlugWords {
		words = words[:maxSlugWords]
	}

	slug := strings.Join(words, "-")
	if len(slug) > maxSlugLen {
		slug = strings.TrimRight(slug[:maxSlugLen], "-")
	}
	return slug
}
