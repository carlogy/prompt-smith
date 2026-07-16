// Package tui implements the interactive skill-picker + live-preview
// terminal UI (see Run in tui.go). This file holds SuggestFilename, a
// pure function with no Bubble Tea dependency so it's trivially unit
// tested.
package tui

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
