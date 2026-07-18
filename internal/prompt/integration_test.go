package prompt_test

import (
	"strings"
	"testing"

	"github.com/carlogy/prompt-smith/internal/prompt"
	"github.com/carlogy/prompt-smith/internal/registry"
)

// TestBuild_WithRealRegistry proves the engine and the real shipped
// content work together end-to-end for every target, not just against
// the fixture registry used by the rest of this package's tests.
//
// PROMPTSMITH_SKILLS_DIR is pinned to an empty temp directory so this
// stays hermetic regardless of the developer machine's real user
// skills directory.
func TestBuild_WithRealRegistry(t *testing.T) {
	t.Setenv("PROMPTSMITH_SKILLS_DIR", t.TempDir())

	reg, warnings, err := registry.Load()
	if err != nil {
		t.Fatalf("registry.Load() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("registry.Load() warnings = %v, want none", warnings)
	}

	tests := []struct {
		target string
		skill  string
		want   string // substring expected somewhere in the output
	}{
		{"generic", "diagnose", "pass/fail"},                        // inlined methodology
		{"opencode", "diagnose", "Load the `diagnose` skill"},       // reference mode
		{"claude-code", "verify", "Load the `verify-checks` skill"}, // renamed ref
		{"gemini-cli", "diagnose", "Load the `diagnose` skill"},     // reference mode
		{"codex", "diagnose", "Load the `diagnose` skill"},          // reference mode, no tools
	}

	for _, tt := range tests {
		t.Run(tt.target, func(t *testing.T) {
			got, err := prompt.Build(reg, prompt.Inputs{
				Target: tt.target,
				Skills: []string{tt.skill},
				Goal:   "Fix the flaky checkout test.",
			})
			if err != nil {
				t.Fatalf("Build() error = %v", err)
			}
			if !strings.Contains(got, tt.want) {
				t.Errorf("Build() output missing %q:\n%s", tt.want, got)
			}
		})
	}
}
