package registry

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// customCategory is where a loose user skill (no category subdirectory)
// lands - see loadUserSkills.
const customCategory = "custom"

// userSkillsDir returns the directory Load merges user-provided skills
// from. $PROMPTSMITH_SKILLS_DIR, if set, wins outright. Otherwise it's
// $XDG_CONFIG_HOME/promptsmith/skills, falling back to
// ~/.config/promptsmith/skills per the XDG Base Directory spec. It's not
// an error for the directory not to exist - that's the common case, and
// Load treats it as "no user skills" rather than a failure.
func userSkillsDir() (string, error) {
	if dir := os.Getenv("PROMPTSMITH_SKILLS_DIR"); dir != "" {
		return dir, nil
	}

	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		configHome = filepath.Join(home, ".config")
	}
	return filepath.Join(configHome, "promptsmith", "skills"), nil
}

// skillFrontmatter is the YAML frontmatter shape shared by Claude/opencode
// SKILL.md files, so a user's existing skill file drops in unmodified.
type skillFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// parseSkillMD parses a SKILL.md file: a "---"-delimited YAML frontmatter
// block (name, description) followed by a markdown body. name becomes
// the skill's id, description its WhenToUse, and the body is used
// verbatim - unlike the embedded registry's hand-curated bodies (see
// data/bodies), there's no distillation step, so a drop-in doesn't
// require separate authoring.
func parseSkillMD(data []byte) (name, description, body string, err error) {
	// Normalize CRLF/CR to LF up front - the one choke point both the
	// frontmatter and body pass through - so a SKILL.md authored or
	// edited on Windows parses identically to a Unix one. Embedded
	// bodies are covered by .gitattributes (LF on every checkout); this
	// covers a user's own file, which is outside git's reach.
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", "", "", errors.New(`missing frontmatter: expected the file to start with "---"`)
	}

	closeAt := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			closeAt = i
			break
		}
	}
	if closeAt == -1 {
		return "", "", "", errors.New(`missing frontmatter: no closing "---" found`)
	}

	var fm skillFrontmatter
	fmBlock := strings.Join(lines[1:closeAt], "\n")
	if err := yaml.Unmarshal([]byte(fmBlock), &fm); err != nil {
		return "", "", "", fmt.Errorf("parse frontmatter: %w", err)
	}
	if fm.Name == "" {
		return "", "", "", errors.New(`frontmatter missing required "name" field`)
	}

	body = strings.TrimSpace(strings.Join(lines[closeAt+1:], "\n"))
	return fm.Name, fm.Description, body, nil
}

// displayName derives a human-readable display name from a kebab-case
// skill id (e.g. "caveman-commit" -> "Caveman Commit"), since SKILL.md's
// frontmatter has no separate display-name field.
func displayName(id string) string {
	words := strings.Split(id, "-")
	for i, w := range words {
		if w == "" {
			continue
		}
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

// loadUserSkills walks fsys for user-provided skills, laid out as:
//
//	<category>/<skill-id>/SKILL.md   -> explicit category
//	<skill-id>/SKILL.md              -> loose skill, category "custom"
//
// This mirrors how Claude/opencode skills are conventionally organized
// on disk, so no separate import step is needed for an existing skill
// set. Malformed skills and skill ids that collide with another *user*
// skill are reported as warnings and skipped rather than failing the
// whole load - a single bad drop-in shouldn't take down the registry.
// (Overriding an *embedded* skill by id is the intended override
// mechanism - see mergeUserSkills - and never warns.)
func loadUserSkills(fsys fs.FS) (skills []Skill, categoriesInOrder []string, warnings []string) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, nil, []string{fmt.Sprintf("read user skills directory: %v", err)}
	}

	seenCategory := make(map[string]bool)
	seenID := make(map[string]bool)

	addCategory := func(cat string) {
		if !seenCategory[cat] {
			seenCategory[cat] = true
			categoriesInOrder = append(categoriesInOrder, cat)
		}
	}

	load := func(dir, category string) {
		mdPath := path.Join(dir, "SKILL.md")
		data, err := fs.ReadFile(fsys, mdPath)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("skip %s: %v", dir, err))
			return
		}
		name, description, body, err := parseSkillMD(data)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("skip %s: %v", mdPath, err))
			return
		}
		if seenID[name] {
			warnings = append(warnings, fmt.Sprintf("skip %s: duplicate user skill id %q", mdPath, name))
			return
		}
		seenID[name] = true
		addCategory(category)
		skills = append(skills, Skill{
			ID:        name,
			Name:      displayName(name),
			Category:  category,
			WhenToUse: description,
			Body:      body,
		})
	}

	for _, top := range entries {
		if !top.IsDir() {
			continue // stray files at the root are ignored
		}
		name := top.Name()

		if hasSkillMD(fsys, name) {
			load(name, customCategory) // loose skill dir: no category subdir
			continue
		}

		// Not a skill dir itself, so treat it as a category and look
		// one level deeper.
		subEntries, err := fs.ReadDir(fsys, name)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("skip %s: %v", name, err))
			continue
		}
		for _, sub := range subEntries {
			if !sub.IsDir() {
				continue
			}
			skillDir := path.Join(name, sub.Name())
			if !hasSkillMD(fsys, skillDir) {
				warnings = append(warnings, fmt.Sprintf("skip %s: no SKILL.md found", skillDir))
				continue
			}
			load(skillDir, name)
		}
	}

	return skills, categoriesInOrder, warnings
}

func hasSkillMD(fsys fs.FS, dir string) bool {
	_, err := fs.Stat(fsys, path.Join(dir, "SKILL.md"))
	return err == nil
}

// mergeUserSkills combines base (the embedded registry) with
// user-provided skills: a user skill whose id matches an existing one
// overrides it outright (the whole record, not a field-by-field patch);
// any other user skill is appended. newCategories are appended after
// base's canonical categories, in first-seen order, skipping any that
// already exist - so an embedded category is never duplicated or
// reordered.
func mergeUserSkills(base *Registry, userSkills []Skill, newCategories []string) *Registry {
	merged := &Registry{
		Categories: append([]string(nil), base.Categories...),
		Skills:     append([]Skill(nil), base.Skills...),
		Targets:    base.Targets,
	}

	existingCategory := make(map[string]bool, len(merged.Categories))
	for _, c := range merged.Categories {
		existingCategory[c] = true
	}
	for _, c := range newCategories {
		if !existingCategory[c] {
			existingCategory[c] = true
			merged.Categories = append(merged.Categories, c)
		}
	}

	indexByID := make(map[string]int, len(merged.Skills))
	for i, sk := range merged.Skills {
		indexByID[sk.ID] = i
	}
	for _, sk := range userSkills {
		if i, ok := indexByID[sk.ID]; ok {
			merged.Skills[i] = sk
			continue
		}
		indexByID[sk.ID] = len(merged.Skills)
		merged.Skills = append(merged.Skills, sk)
	}

	return merged
}
