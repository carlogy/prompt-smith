package registry_test

import (
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
