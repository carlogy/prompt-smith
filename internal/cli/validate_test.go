package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/carlogy/prompt-smith/internal/registry"
)

func TestValidate_WellFormedRegistryPrintsOK(t *testing.T) {
	reg := testRegistry(t)
	cmd := newValidateCmd(reg)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(stdout.String(), "ok") {
		t.Errorf("expected an ok confirmation, got:\n%s", stdout.String())
	}
}

func TestValidate_InvalidRegistryErrors(t *testing.T) {
	reg := &registry.Registry{
		Categories: []string{"debugging"},
		Skills: []registry.Skill{
			{ID: "diagnose", Category: "nonexistent-category", Body: "text"},
		},
		Targets: map[string]registry.TargetConfig{
			"generic": {ID: "generic", SkillMode: "inline"},
		},
	}
	cmd := newValidateCmd(reg)
	cmd.SetOut(&bytes.Buffer{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want an error for a dangling category reference")
	}
	if !strings.Contains(err.Error(), "nonexistent-category") {
		t.Errorf("expected the error to name the offending category, got: %v", err)
	}
}
