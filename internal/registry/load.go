package registry

import (
	"fmt"
	"io/fs"

	"gopkg.in/yaml.v3"
)

// skillsDoc is the on-disk shape of skills.yaml.
type skillsDoc struct {
	Categories []string   `yaml:"categories"`
	Skills     []skillDoc `yaml:"skills"`
}

type skillDoc struct {
	ID        string            `yaml:"id"`
	Name      string            `yaml:"name"`
	Category  string            `yaml:"category"`
	Order     int               `yaml:"order"`
	WhenToUse string            `yaml:"when_to_use"`
	Body      string            `yaml:"body"` // path to the body file, relative to the registry root
	Refs      map[string]string `yaml:"refs"`
}

// targetsDoc is the on-disk shape of targets.yaml.
type targetsDoc struct {
	Targets []targetDoc `yaml:"targets"`
}

type targetDoc struct {
	ID        string            `yaml:"id"`
	Name      string            `yaml:"name"`
	Delimiter string            `yaml:"delimiter"`
	SkillMode string            `yaml:"skill_mode"`
	Tools     map[string]string `yaml:"tools"`
}

// LoadFS parses a registry from fsys, which must contain skills.yaml,
// targets.yaml, and every body file referenced by a skill's body path.
// Load is the production entry point (the embedded registry); LoadFS
// exists so loading/parsing behavior is testable against synthetic
// filesystems.
func LoadFS(fsys fs.FS) (*Registry, error) {
	var sdoc skillsDoc
	if err := readYAML(fsys, "skills.yaml", &sdoc); err != nil {
		return nil, err
	}

	var tdoc targetsDoc
	if err := readYAML(fsys, "targets.yaml", &tdoc); err != nil {
		return nil, err
	}

	targets := make(map[string]TargetConfig, len(tdoc.Targets))
	for _, t := range tdoc.Targets {
		targets[t.ID] = TargetConfig{
			ID:        t.ID,
			Name:      t.Name,
			Delimiter: t.Delimiter,
			SkillMode: t.SkillMode,
			Tools:     t.Tools,
		}
	}

	skills := make([]Skill, 0, len(sdoc.Skills))
	for _, s := range sdoc.Skills {
		var body string
		if s.Body != "" {
			raw, err := fs.ReadFile(fsys, s.Body)
			if err != nil {
				return nil, fmt.Errorf("registry: skill %q: read body %q: %w", s.ID, s.Body, err)
			}
			body = string(raw)
		}
		skills = append(skills, Skill{
			ID:        s.ID,
			Name:      s.Name,
			Category:  s.Category,
			Order:     s.Order,
			WhenToUse: s.WhenToUse,
			Body:      body,
			Refs:      s.Refs,
		})
	}

	return &Registry{
		Categories: sdoc.Categories,
		Skills:     skills,
		Targets:    targets,
	}, nil
}

func readYAML(fsys fs.FS, name string, out any) error {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return fmt.Errorf("registry: read %s: %w", name, err)
	}
	if err := yaml.Unmarshal(data, out); err != nil {
		return fmt.Errorf("registry: parse %s: %w", name, err)
	}
	return nil
}
