package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestHelp_RootIncludesExamples(t *testing.T) {
	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetArgs([]string{"--help"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "Examples:") {
		t.Errorf("expected --help to include an Examples: section, got:\n%s", got)
	}
	if !strings.Contains(got, `promptsmith "fix`) {
		t.Errorf("expected a sample goal invocation, got:\n%s", got)
	}
}

func TestHelp_ListIncludesExamples(t *testing.T) {
	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetArgs([]string{"list", "--help"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "Examples:") {
		t.Errorf("expected list --help to include an Examples: section, got:\n%s", got)
	}
}

func TestHelp_ValidateIncludesExamples(t *testing.T) {
	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetArgs([]string{"validate", "--help"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "Examples:") {
		t.Errorf("expected validate --help to include an Examples: section, got:\n%s", got)
	}
}
