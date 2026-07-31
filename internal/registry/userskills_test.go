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

package registry

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
)

func TestUserSkillsDir(t *testing.T) {
	t.Run("env override wins outright", func(t *testing.T) {
		t.Setenv("PROMPTSMITH_SKILLS_DIR", "/custom/skills/dir")
		t.Setenv("XDG_CONFIG_HOME", "/should/be/ignored")

		got, err := userSkillsDir()
		if err != nil {
			t.Fatalf("userSkillsDir() error = %v", err)
		}
		if got != "/custom/skills/dir" {
			t.Errorf("userSkillsDir() = %q, want %q", got, "/custom/skills/dir")
		}
	})

	t.Run("XDG_CONFIG_HOME is used when set", func(t *testing.T) {
		t.Setenv("PROMPTSMITH_SKILLS_DIR", "")
		t.Setenv("XDG_CONFIG_HOME", "/xdg/config")

		got, err := userSkillsDir()
		if err != nil {
			t.Fatalf("userSkillsDir() error = %v", err)
		}
		want := filepath.Join("/xdg/config", "promptsmith", "skills")
		if got != want {
			t.Errorf("userSkillsDir() = %q, want %q", got, want)
		}
	})

	t.Run("falls back to ~/.config", func(t *testing.T) {
		t.Setenv("PROMPTSMITH_SKILLS_DIR", "")
		t.Setenv("XDG_CONFIG_HOME", "")

		home, err := os.UserHomeDir()
		if err != nil {
			t.Fatalf("os.UserHomeDir() error = %v", err)
		}

		got, err := userSkillsDir()
		if err != nil {
			t.Fatalf("userSkillsDir() error = %v", err)
		}
		want := filepath.Join(home, ".config", "promptsmith", "skills")
		if got != want {
			t.Errorf("userSkillsDir() = %q, want %q", got, want)
		}
	})
}

func TestParseSkillMD(t *testing.T) {
	cases := []struct {
		name            string
		data            string
		wantID          string
		wantDescription string
		wantBody        string
		wantErr         bool
	}{
		{
			name: "plain scalar description",
			data: "---\n" +
				"name: architect\n" +
				"description: Map before you modify.\n" +
				"---\n" +
				"\n" +
				"# Architect\n\nMap first.\n",
			wantID:          "architect",
			wantDescription: "Map before you modify.",
			wantBody:        "# Architect\n\nMap first.",
		},
		{
			name: "folded block-scalar description",
			data: "---\n" +
				"name: caveman-commit\n" +
				"description: >\n" +
				"  Ultra-compressed commit messages.\n" +
				"  Conventional Commits format.\n" +
				"---\n" +
				"Write terse commit messages.\n",
			wantID:          "caveman-commit",
			wantDescription: "Ultra-compressed commit messages. Conventional Commits format.",
			wantBody:        "Write terse commit messages.",
		},
		{
			name: "no description is not an error",
			data: "---\n" +
				"name: bare\n" +
				"---\n" +
				"Body text.\n",
			wantID:          "bare",
			wantDescription: "",
			wantBody:        "Body text.",
		},
		{
			name: "CRLF line endings are normalized",
			data: "---\r\n" +
				"name: windows-skill\r\n" +
				"description: Authored on Windows.\r\n" +
				"---\r\n" +
				"Line one.\r\n" +
				"Line two.\r\n",
			wantID:          "windows-skill",
			wantDescription: "Authored on Windows.",
			wantBody:        "Line one.\nLine two.",
		},
		{
			name:    "missing opening delimiter errors",
			data:    "name: architect\ndescription: x\n---\nbody\n",
			wantErr: true,
		},
		{
			name:    "missing closing delimiter errors",
			data:    "---\nname: architect\ndescription: x\nbody\n",
			wantErr: true,
		},
		{
			name:    "missing name field errors",
			data:    "---\ndescription: x\n---\nbody\n",
			wantErr: true,
		},
		{
			name:    "malformed frontmatter YAML errors",
			data:    "---\nname: [unterminated\n---\nbody\n",
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id, description, body, err := parseSkillMD([]byte(tc.data))
			if tc.wantErr {
				if err == nil {
					t.Fatal("parseSkillMD() error = nil, want an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseSkillMD() error = %v", err)
			}
			if id != tc.wantID {
				t.Errorf("id = %q, want %q", id, tc.wantID)
			}
			if description != tc.wantDescription {
				t.Errorf("description = %q, want %q", description, tc.wantDescription)
			}
			if body != tc.wantBody {
				t.Errorf("body = %q, want %q", body, tc.wantBody)
			}
		})
	}
}

func TestDisplayName(t *testing.T) {
	cases := []struct{ id, want string }{
		{"architect", "Architect"},
		{"caveman-commit", "Caveman Commit"},
		{"a-b-c", "A B C"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := displayName(tc.id); got != tc.want {
			t.Errorf("displayName(%q) = %q, want %q", tc.id, got, tc.want)
		}
	}
}

// userSkillMD builds a minimal, valid SKILL.md body for test fixtures.
func userSkillMD(name, description, body string) *fstest.MapFile {
	return &fstest.MapFile{Data: []byte(
		"---\nname: " + name + "\ndescription: " + description + "\n---\n" + body + "\n",
	)}
}

func TestLoadUserSkills(t *testing.T) {
	t.Run("empty directory yields nothing", func(t *testing.T) {
		skills, cats, warnings := loadUserSkills(fstest.MapFS{})
		if len(skills) != 0 || len(cats) != 0 || len(warnings) != 0 {
			t.Errorf("loadUserSkills() = %v, %v, %v, want all empty", skills, cats, warnings)
		}
	})

	t.Run("categorized and loose skills, malformed and duplicate skipped with warnings", func(t *testing.T) {
		fsys := fstest.MapFS{
			// Explicit category.
			"testing/my-checklist/SKILL.md": userSkillMD("my-checklist", "A pre-flight checklist.", "Check everything."),
			// Loose: falls into "custom".
			"standalone/SKILL.md": userSkillMD("standalone", "No category subdir.", "Standalone body."),
			// Malformed frontmatter: skipped with a warning, not fatal.
			"testing/broken/SKILL.md": &fstest.MapFile{Data: []byte("not frontmatter at all")},
			// Category dir whose subdir has no SKILL.md at all.
			"testing/empty-dir/placeholder.txt": &fstest.MapFile{Data: []byte("x")},
			// Stray file at the root: ignored, not a directory.
			"README.txt": &fstest.MapFile{Data: []byte("x")},
			// Duplicate id: a second skill also named "standalone".
			"testing/dup/SKILL.md": userSkillMD("standalone", "Collides with the loose one above.", "Dup body."),
		}

		skills, cats, warnings := loadUserSkills(fsys)

		byID := make(map[string]Skill, len(skills))
		for _, sk := range skills {
			byID[sk.ID] = sk
		}

		checklist, ok := byID["my-checklist"]
		if !ok {
			t.Fatal(`expected "my-checklist" to be loaded`)
		}
		if checklist.Category != "testing" {
			t.Errorf("my-checklist.Category = %q, want %q", checklist.Category, "testing")
		}
		if checklist.Name != "My Checklist" {
			t.Errorf("my-checklist.Name = %q, want %q", checklist.Name, "My Checklist")
		}

		standalone, ok := byID["standalone"]
		if !ok {
			t.Fatal(`expected "standalone" to be loaded`)
		}
		if standalone.Category != customCategory {
			t.Errorf("standalone.Category = %q, want %q", standalone.Category, customCategory)
		}
		// The duplicate ("testing/dup") must have been skipped, so the
		// first-seen body wins.
		if standalone.Body != "Standalone body." {
			t.Errorf("standalone.Body = %q, want the first-seen body", standalone.Body)
		}

		if len(skills) != 2 {
			t.Errorf("len(skills) = %d, want 2 (broken and duplicate skipped)", len(skills))
		}

		wantCats := []string{customCategory, "testing"} // fs.ReadDir sorts by name: "standalone" < "testing"
		if !reflect.DeepEqual(cats, wantCats) {
			t.Errorf("categories = %v, want %v", cats, wantCats)
		}

		if len(warnings) != 3 {
			t.Errorf("len(warnings) = %d, want 3 (broken frontmatter, empty-dir, duplicate), got: %v", len(warnings), warnings)
		}
	})

	t.Run("dot-prefixed directories are skipped silently at both levels", func(t *testing.T) {
		fsys := fstest.MapFS{
			// A valid loose skill, so the walk has something to find.
			"myskill/SKILL.md": userSkillMD("myskill", "A valid skill.", "Body."),
			// A dot-prefixed top-level dir, shaped like a real .git
			// tree: not a skill dir itself, so without the dot-skip
			// it would be treated as a category and every subdir
			// below would warn "no SKILL.md found".
			"testing/.git/objects/placeholder.txt": &fstest.MapFile{Data: []byte("x")},
			"testing/.git/refs/placeholder.txt":    &fstest.MapFile{Data: []byte("x")},
			// A dot-prefixed subdir one level deeper, alongside a
			// real skill in the same category - the sub-level skip.
			"testing/real-skill/SKILL.md":     userSkillMD("real-skill", "Real.", "Body."),
			"testing/.hidden/placeholder.txt": &fstest.MapFile{Data: []byte("x")},
		}

		skills, _, warnings := loadUserSkills(fsys)

		if len(warnings) != 0 {
			t.Errorf("warnings = %v, want none - dot-prefixed dirs must be skipped silently", warnings)
		}
		byID := make(map[string]Skill, len(skills))
		for _, sk := range skills {
			byID[sk.ID] = sk
		}
		if _, ok := byID["myskill"]; !ok {
			t.Error(`expected "myskill" to be loaded`)
		}
		if _, ok := byID["real-skill"]; !ok {
			t.Error(`expected "real-skill" to be loaded`)
		}
		if len(skills) != 2 {
			t.Errorf("len(skills) = %d, want 2 (.git and .hidden must not surface as skills or warnings)", len(skills))
		}
	})

	t.Run("SKILL.md directly at the root warns, unlike other stray files", func(t *testing.T) {
		fsys := fstest.MapFS{
			"SKILL.md":  userSkillMD("orphan", "Never nested in its own dir.", "Body."),
			".DS_Store": &fstest.MapFile{Data: []byte("x")},
			"README.md": &fstest.MapFile{Data: []byte("x")},
			"LICENSE":   &fstest.MapFile{Data: []byte("x")},
		}

		skills, _, warnings := loadUserSkills(fsys)

		if len(skills) != 0 {
			t.Errorf("skills = %v, want none - a root-level SKILL.md is never loaded as a skill", skills)
		}
		if len(warnings) != 1 {
			t.Fatalf("warnings = %v, want exactly 1 (SKILL.md only, not .DS_Store/README.md/LICENSE)", warnings)
		}
		if !strings.Contains(warnings[0], "SKILL.md") {
			t.Errorf("warnings[0] = %q, want it to mention SKILL.md", warnings[0])
		}
	})

	t.Run("top-level directory yielding zero skills warns exactly once", func(t *testing.T) {
		fsys := fstest.MapFS{
			// A category dir with no subdirectories at all - not even
			// a broken one, so nothing else would ever warn about it.
			"empty-category/placeholder.txt": &fstest.MapFile{Data: []byte("x")},
		}

		_, _, warnings := loadUserSkills(fsys)

		if len(warnings) != 1 {
			t.Fatalf("warnings = %v, want exactly 1", warnings)
		}
		if !strings.Contains(warnings[0], "empty-category") {
			t.Errorf("warnings[0] = %q, want it to name %q", warnings[0], "empty-category")
		}
	})

	t.Run("one good and one bad subdir warns only once, not twice", func(t *testing.T) {
		fsys := fstest.MapFS{
			"mixed/good/SKILL.md": userSkillMD("good", "Loads fine.", "Body."),
			// No SKILL.md here - already produces its own warning.
			"mixed/bad/placeholder.txt": &fstest.MapFile{Data: []byte("x")},
		}

		skills, _, warnings := loadUserSkills(fsys)

		if len(skills) != 1 {
			t.Errorf("len(skills) = %d, want 1", len(skills))
		}
		if len(warnings) != 1 {
			t.Errorf("warnings = %v, want exactly 1 (the existing \"no SKILL.md found\", not also a zero-skills warning)", warnings)
		}
	})
}

func TestMergeUserSkills(t *testing.T) {
	base := &Registry{
		Categories: []string{"debugging", "testing"},
		Skills: []Skill{
			{ID: "diagnose", Name: "Diagnose", Category: "debugging", Body: "embedded diagnose body"},
			{ID: "verify", Name: "Verify", Category: "testing", Body: "embedded verify body"},
		},
		Targets: map[string]TargetConfig{"generic": {ID: "generic"}},
	}

	userSkills := []Skill{
		// Overrides the embedded "diagnose" entirely, including category.
		{ID: "diagnose", Name: "Diagnose", Category: customCategory, Body: "user diagnose body"},
		// Brand new skill.
		{ID: "brand-new", Name: "Brand New", Category: customCategory, Body: "new body"},
	}
	newCategories := []string{"debugging", customCategory} // "debugging" already exists

	merged := mergeUserSkills(base, userSkills, newCategories)

	wantCats := []string{"debugging", "testing", customCategory}
	if !reflect.DeepEqual(merged.Categories, wantCats) {
		t.Errorf("Categories = %v, want %v", merged.Categories, wantCats)
	}

	if len(merged.Skills) != 3 {
		t.Fatalf("len(Skills) = %d, want 3 (override in place + one append)", len(merged.Skills))
	}

	diagnose, ok := merged.SkillByID("diagnose")
	if !ok {
		t.Fatal(`expected "diagnose" to still be present`)
	}
	if diagnose.Body != "user diagnose body" || diagnose.Category != customCategory {
		t.Errorf("diagnose = %+v, want the user override (whole record replaced)", diagnose)
	}

	verify, ok := merged.SkillByID("verify")
	if !ok || verify.Body != "embedded verify body" {
		t.Errorf("verify = %+v, want the untouched embedded record", verify)
	}

	brandNew, ok := merged.SkillByID("brand-new")
	if !ok || brandNew.Body != "new body" {
		t.Errorf("brand-new = %+v, want the appended user skill", brandNew)
	}

	// base itself must be untouched (merge copies, never mutates in place).
	if len(base.Skills) != 2 || len(base.Categories) != 2 {
		t.Errorf("base was mutated: Skills=%v Categories=%v", base.Skills, base.Categories)
	}
}
