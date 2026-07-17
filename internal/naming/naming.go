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
