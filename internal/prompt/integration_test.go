package prompt_test

import (
	"strings"
	"testing"

	"github.com/carlogy/prompt-smith/internal/prompt"
	"github.com/carlogy/prompt-smith/internal/registry"
)

// TestBuild_WithRealRegistry proves the engine and the real shipped
// content work together end-to-end for all three targets, not just
// against the fixture registry used by the rest of this package's tests.
func TestBuild_WithRealRegistry(t *testing.T) {
	reg, err := registry.Load()
	if err != nil {
		t.Fatalf("registry.Load() error = %v", err)
	}

	tests := []struct {
		target string
		skill  string
		want   string // substring expected somewhere in the output
	}{
		{"generic", "diagnose", "pass/fail"},                        // inlined methodology
		{"opencode", "diagnose", "Load the `diagnose` skill"},       // reference mode
		{"claude-code", "verify", "Load the `verify-checks` skill"}, // renamed ref
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
