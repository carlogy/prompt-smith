package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/carlogy/prompt-smith/internal/registry"
)

// writeUserSkillMD writes a minimal, valid SKILL.md at
// <dir>/<id>/SKILL.md - a loose skill dir, the same flat layout real
// Claude/opencode skill directories use (no category subdirectory), so
// it lands in the "custom" category. See registry.loadUserSkills.
func writeUserSkillMD(t *testing.T, dir, id, description, body string) {
	t.Helper()
	skillDir := filepath.Join(dir, id)
	if err := os.MkdirAll(skillDir, 0o700); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", skillDir, err)
	}
	content := "---\nname: " + id + "\ndescription: " + description + "\n---\n" + body + "\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(SKILL.md) error = %v", err)
	}
}

// TestIntegration_UserSkillFromEnvVarAppearsInListAndGenerate proves the
// full path end-to-end: a real SKILL.md dropped into a directory named
// by PROMPTSMITH_SKILLS_DIR is picked up by the real registry.Load (not
// the hermetic testRegistry helper the rest of this package's tests
// use), shows up in `list` under the "custom" category, and its body
// renders when selected via `-s` in generate - exactly as if it had
// shipped in the embedded registry.
func TestIntegration_UserSkillFromEnvVarAppearsInListAndGenerate(t *testing.T) {
	skillsDir := t.TempDir()
	t.Setenv("PROMPTSMITH_SKILLS_DIR", skillsDir)
	writeUserSkillMD(t, skillsDir, "pair-programming",
		"Pairing on a hard problem.",
		"Talk through the approach out loud before writing any code.")

	reg, warnings, err := registry.Load()
	if err != nil {
		t.Fatalf("registry.Load() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("registry.Load() warnings = %v, want none", warnings)
	}

	t.Run("list", func(t *testing.T) {
		cmd := newListCmd(reg)
		var stdout bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetArgs(nil)

		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		out := stdout.String()
		if !strings.Contains(out, "CUSTOM") {
			t.Errorf("expected a CUSTOM category header, got:\n%s", out)
		}
		if !strings.Contains(out, "pair-programming") {
			t.Errorf("expected pair-programming to be listed, got:\n%s", out)
		}
	})

	t.Run("generate", func(t *testing.T) {
		root := newRootCmd(reg)
		var stdout, stderr bytes.Buffer
		root.SetOut(&stdout)
		root.SetErr(&stderr)
		root.SetArgs([]string{"-t", "generic", "-s", "pair-programming", "solve this bug together"})

		if err := root.Execute(); err != nil {
			t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
		}

		want := "Talk through the approach out loud before writing any code."
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("stdout missing the user skill's body %q, got:\n%s", want, stdout.String())
		}
	})
}

// TestIntegration_MalformedUserSkillWarnsButDoesNotBreakLoad proves a
// bad drop-in degrades to a warning rather than taking down the whole
// registry - the embedded skills must still load and work normally.
func TestIntegration_MalformedUserSkillWarnsButDoesNotBreakLoad(t *testing.T) {
	skillsDir := t.TempDir()
	t.Setenv("PROMPTSMITH_SKILLS_DIR", skillsDir)

	brokenDir := filepath.Join(skillsDir, "broken")
	if err := os.MkdirAll(brokenDir, 0o700); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", brokenDir, err)
	}
	if err := os.WriteFile(filepath.Join(brokenDir, "SKILL.md"), []byte("not valid frontmatter"), 0o600); err != nil {
		t.Fatalf("WriteFile(SKILL.md) error = %v", err)
	}

	reg, warnings, err := registry.Load()
	if err != nil {
		t.Fatalf("registry.Load() error = %v", err)
	}
	if len(warnings) != 1 {
		t.Fatalf("registry.Load() warnings = %v, want exactly one", warnings)
	}
	if !strings.Contains(warnings[0], "broken") {
		t.Errorf("warning = %q, want it to mention the offending skill dir", warnings[0])
	}

	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"-t", "generic", "-s", "diagnose", "goal"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "pass/fail") {
		t.Errorf("expected the embedded diagnose skill to still work, got:\n%s", stdout.String())
	}
}
