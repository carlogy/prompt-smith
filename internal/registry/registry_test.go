package registry_test

import (
	"reflect"
	"testing"
	"testing/fstest"

	"github.com/carlogy/prompt-smith/internal/registry"
)

// minimalFS returns a synthetic, in-memory registry filesystem for
// testing LoadFS's parsing behavior without touching real shipped data.
func minimalFS() fstest.MapFS {
	return fstest.MapFS{
		"skills.yaml": &fstest.MapFile{Data: []byte(`
categories:
  - debugging
skills:
  - id: diagnose
    name: Diagnose
    category: debugging
    order: 10
    when_to_use: "Hard bugs."
    body: bodies/debugging/diagnose.md
`)},
		"targets.yaml": &fstest.MapFile{Data: []byte(`
targets:
  - id: generic
    delimiter: xml
    skill_mode: inline
`)},
		"bodies/debugging/diagnose.md": &fstest.MapFile{Data: []byte("Build a feedback loop first.")},
	}
}

func TestLoadFS_ParsesMinimalRegistry(t *testing.T) {
	reg, err := registry.LoadFS(minimalFS())
	if err != nil {
		t.Fatalf("LoadFS() error = %v", err)
	}

	sk, ok := reg.SkillByID("diagnose")
	if !ok {
		t.Fatal(`expected skill "diagnose" to be loaded`)
	}
	if sk.Body != "Build a feedback loop first." {
		t.Errorf("Body = %q", sk.Body)
	}
	if sk.Category != "debugging" || sk.Order != 10 {
		t.Errorf("Category/Order = %q/%d, want debugging/10", sk.Category, sk.Order)
	}
	if sk.WhenToUse != "Hard bugs." {
		t.Errorf("WhenToUse = %q", sk.WhenToUse)
	}

	if _, ok := reg.Targets["generic"]; !ok {
		t.Fatal(`expected target "generic" to be loaded`)
	}
	if len(reg.Categories) != 1 || reg.Categories[0] != "debugging" {
		t.Errorf("Categories = %v", reg.Categories)
	}
}

// TestLoadFS_ParsesTargetName proves targets.yaml's optional name:
// field reaches TargetConfig.Name - the display label rendered in the
// web UI's target <select> (see server/page.go). minimalFS's own
// "generic" target has no name: key, so this checks the field lands
// correctly when present rather than duplicating that omitted-case
// coverage (see TestTargetConfig_DisplayName for the fallback).
func TestLoadFS_ParsesTargetName(t *testing.T) {
	fsys := minimalFS()
	fsys["targets.yaml"] = &fstest.MapFile{Data: []byte(`
targets:
  - id: generic
    name: Generic / Chat
    delimiter: xml
    skill_mode: inline
`)}

	reg, err := registry.LoadFS(fsys)
	if err != nil {
		t.Fatalf("LoadFS() error = %v", err)
	}

	got := reg.Targets["generic"].Name
	if got != "Generic / Chat" {
		t.Errorf(`Targets["generic"].Name = %q, want "Generic / Chat"`, got)
	}
}

func TestTargetConfig_DisplayName(t *testing.T) {
	cases := []struct {
		name string
		cfg  registry.TargetConfig
		want string
	}{
		{"explicit name wins", registry.TargetConfig{ID: "claude-code", Name: "Claude Code"}, "Claude Code"},
		{"empty name falls back to id", registry.TargetConfig{ID: "opencode", Name: ""}, "opencode"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.cfg.DisplayName(); got != tc.want {
				t.Errorf("DisplayName() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRegistry_SortSkills(t *testing.T) {
	reg := &registry.Registry{Categories: []string{"debugging", "testing"}}
	skills := []registry.Skill{
		{ID: "verify", Category: "testing", Order: 20},
		{ID: "diagnose", Category: "debugging", Order: 10},
		{ID: "audit", Category: "testing", Order: 10},
		{ID: "check", Category: "testing", Order: 10},
	}

	reg.SortSkills(skills)

	var ids []string
	for _, sk := range skills {
		ids = append(ids, sk.ID)
	}
	// diagnose: category order wins (debugging < testing).
	// audit, check: same category+order, id tiebreak (audit < check).
	// verify: same category, higher order, sorts last.
	want := []string{"diagnose", "audit", "check", "verify"}
	if !reflect.DeepEqual(ids, want) {
		t.Errorf("SortSkills order = %v, want %v", ids, want)
	}
}

func TestRegistry_SupportsTarget(t *testing.T) {
	reg := &registry.Registry{
		Categories: []string{"debugging"},
		Skills: []registry.Skill{
			{ID: "diagnose", Category: "debugging", Body: "inline text"},
			{ID: "agent-only", Category: "debugging"}, // no Body
		},
		Targets: map[string]registry.TargetConfig{
			"generic":  {ID: "generic", SkillMode: "inline"},
			"opencode": {ID: "opencode", SkillMode: "reference"},
		},
	}

	cases := []struct {
		skillID, target string
		want            bool
	}{
		{"diagnose", "generic", true},
		{"agent-only", "generic", false}, // no body -> unsupported on inline target
		{"diagnose", "opencode", true},
		{"agent-only", "opencode", true},      // reference-mode targets: always supported
		{"diagnose", "does-not-exist", false}, // unknown target
	}

	for _, tc := range cases {
		sk, ok := reg.SkillByID(tc.skillID)
		if !ok {
			t.Fatalf("fixture skill %q not found", tc.skillID)
		}
		if got := reg.SupportsTarget(sk, tc.target); got != tc.want {
			t.Errorf("SupportsTarget(%s, %s) = %v, want %v", tc.skillID, tc.target, got, tc.want)
		}
	}
}

func TestLoadFS_MissingBodyFileErrors(t *testing.T) {
	fsys := minimalFS()
	delete(fsys, "bodies/debugging/diagnose.md")

	if _, err := registry.LoadFS(fsys); err == nil {
		t.Fatal("LoadFS() error = nil, want an error for a missing body file")
	}
}

func TestLoadFS_MalformedYAMLErrors(t *testing.T) {
	fsys := minimalFS()
	fsys["skills.yaml"] = &fstest.MapFile{Data: []byte("not: [valid: yaml")}

	if _, err := registry.LoadFS(fsys); err == nil {
		t.Fatal("LoadFS() error = nil, want an error for malformed YAML")
	}
}

func TestRegistry_Validate(t *testing.T) {
	t.Run("well-formed registry passes", func(t *testing.T) {
		reg, err := registry.LoadFS(minimalFS())
		if err != nil {
			t.Fatalf("LoadFS() error = %v", err)
		}
		if err := reg.Validate(); err != nil {
			t.Errorf("Validate() error = %v, want nil", err)
		}
	})

	t.Run("duplicate skill id", func(t *testing.T) {
		fsys := minimalFS()
		fsys["skills.yaml"] = &fstest.MapFile{Data: []byte(`
categories:
  - debugging
skills:
  - id: diagnose
    name: Diagnose
    category: debugging
    body: bodies/debugging/diagnose.md
  - id: diagnose
    name: Diagnose Again
    category: debugging
    body: bodies/debugging/diagnose.md
`)}
		reg, err := registry.LoadFS(fsys)
		if err != nil {
			t.Fatalf("LoadFS() error = %v", err)
		}
		if err := reg.Validate(); err == nil {
			t.Error("Validate() error = nil, want an error for a duplicate skill id")
		}
	})

	t.Run("dangling category reference", func(t *testing.T) {
		fsys := minimalFS()
		fsys["skills.yaml"] = &fstest.MapFile{Data: []byte(`
categories:
  - debugging
skills:
  - id: diagnose
    name: Diagnose
    category: nonexistent-category
    body: bodies/debugging/diagnose.md
`)}
		reg, err := registry.LoadFS(fsys)
		if err != nil {
			t.Fatalf("LoadFS() error = %v", err)
		}
		if err := reg.Validate(); err == nil {
			t.Error("Validate() error = nil, want an error for a dangling category reference")
		}
	})

	t.Run("dangling ref target", func(t *testing.T) {
		fsys := minimalFS()
		fsys["skills.yaml"] = &fstest.MapFile{Data: []byte(`
categories:
  - debugging
skills:
  - id: diagnose
    name: Diagnose
    category: debugging
    body: bodies/debugging/diagnose.md
    refs:
      nonexistent-target: some-ref
`)}
		reg, err := registry.LoadFS(fsys)
		if err != nil {
			t.Fatalf("LoadFS() error = %v", err)
		}
		if err := reg.Validate(); err == nil {
			t.Error("Validate() error = nil, want an error for a ref pointing at an unknown target")
		}
	})
}
