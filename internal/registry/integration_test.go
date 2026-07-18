package registry_test

import (
	"testing"

	"github.com/carlogy/prompt-smith/internal/registry"
)

// TestLoad_RealRegistryIsValid guards the actual shipped, embedded data:
// it must parse and pass Validate(), and must contain what prompt.Build
// depends on for each target's rendering mode. This is what the
// `validate` CLI command runs before a rebuild ships.
//
// PROMPTSMITH_SKILLS_DIR is pinned to an empty temp directory so this
// stays hermetic regardless of the developer machine's real user skills
// directory - this test guards the embedded data specifically, not a
// merge (see userskills_test.go and cli/integration_test.go for that).
func TestLoad_RealRegistryIsValid(t *testing.T) {
	t.Setenv("PROMPTSMITH_SKILLS_DIR", t.TempDir())

	reg, warnings, err := registry.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("Load() warnings = %v, want none", warnings)
	}
	if err := reg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	if len(reg.Skills) != 10 {
		t.Errorf("len(Skills) = %d, want 10", len(reg.Skills))
	}

	for _, target := range []string{"generic", "opencode", "claude-code", "gemini-cli"} {
		if _, ok := reg.Targets[target]; !ok {
			t.Errorf("expected target %q to be defined", target)
		}
	}

	// Every shipped skill must have a non-empty generic body: this
	// registry has no agent-only skills yet, so every skill must render
	// on the "generic" (inline) target.
	for _, sk := range reg.Skills {
		if sk.Body == "" {
			t.Errorf("skill %q has no generic body", sk.ID)
		}
	}

	// "verify" carries the claude-code rename (verify -> verify-checks)
	// this design exists to exercise; guard it explicitly.
	verify, ok := reg.SkillByID("verify")
	if !ok {
		t.Fatal(`expected skill "verify" to be loaded`)
	}
	if verify.Refs["claude-code"] != "verify-checks" {
		t.Errorf(`verify.Refs["claude-code"] = %q, want "verify-checks"`, verify.Refs["claude-code"])
	}
}
