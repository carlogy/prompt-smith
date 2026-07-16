package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stubClipboard substitutes copyToClipboard with fn for the duration of
// the calling test, restoring the original on cleanup.
func stubClipboard(t *testing.T, fn func(string) error) func() {
	t.Helper()
	original := copyToClipboard
	copyToClipboard = fn
	return func() { copyToClipboard = original }
}

func TestGenerate_TracerBullet(t *testing.T) {
	reg := testRegistry(t)
	root := newRootCmd(reg)

	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"-t", "generic", "-s", "diagnose", "fix the flaky checkout test"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
	}

	want := "<task>\nfix the flaky checkout test\n</task>"
	if !bytes.Contains(stdout.Bytes(), []byte(want)) {
		t.Errorf("stdout missing %q, got:\n%s", want, stdout.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("pass/fail")) {
		t.Errorf("stdout missing diagnose body, got:\n%s", stdout.String())
	}
}

func TestGenerate_OptionalFieldsFlowThrough(t *testing.T) {
	reg := testRegistry(t)
	root := newRootCmd(reg)

	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{
		"-t", "generic",
		"-s", "diagnose",
		"--role", "You are a senior Go engineer.",
		"--context", "checkout_test.go:42 is flaky.",
		"--constraints", "Don't change assertions.",
		"--output-format", "Return a unified diff.",
		"fix the flaky checkout test",
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
	}

	for _, want := range []string{
		"<role>\nYou are a senior Go engineer.\n</role>",
		"<context>\ncheckout_test.go:42 is flaky.\n</context>",
		"<constraints>\nDon't change assertions.\n</constraints>",
		"<output_format>\nReturn a unified diff.\n</output_format>",
	} {
		if !bytes.Contains(stdout.Bytes(), []byte(want)) {
			t.Errorf("stdout missing %q, got:\n%s", want, stdout.String())
		}
	}
}

func TestGenerate_SkillsCommaAndRepeatedResolveIdentically(t *testing.T) {
	run := func(args []string) string {
		reg := testRegistry(t)
		root := newRootCmd(reg)
		var stdout, stderr bytes.Buffer
		root.SetOut(&stdout)
		root.SetErr(&stderr)
		root.SetArgs(args)
		if err := root.Execute(); err != nil {
			t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
		}
		return stdout.String()
	}

	comma := run([]string{"-t", "generic", "-s", "diagnose,verify", "goal"})
	repeated := run([]string{"-t", "generic", "-s", "diagnose", "-s", "verify", "goal"})

	if comma != repeated {
		t.Errorf("comma and repeated -s produced different output:\ncomma:    %q\nrepeated: %q", comma, repeated)
	}
}

func TestGenerate_EmptyGoalErrors(t *testing.T) {
	reg := testRegistry(t)
	root := newRootCmd(reg)

	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"-t", "generic", "-s", "diagnose"}) // no goal arg

	if err := root.Execute(); err == nil {
		t.Fatal("Execute() error = nil, want an error for an empty goal")
	}
}

func TestGenerate_UnknownTargetOrSkillErrors(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"unknown target", []string{"-t", "does-not-exist", "-s", "diagnose", "goal"}},
		{"unknown skill", []string{"-t", "generic", "-s", "does-not-exist", "goal"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reg := testRegistry(t)
			root := newRootCmd(reg)
			var stdout, stderr bytes.Buffer
			root.SetOut(&stdout)
			root.SetErr(&stderr)
			root.SetArgs(tc.args)

			if err := root.Execute(); err == nil {
				t.Fatal("Execute() error = nil, want an error")
			}
		})
	}
}

func TestGenerate_OutWritesFileAndSuppressesStdout(t *testing.T) {
	reg := testRegistry(t)
	root := newRootCmd(reg)

	outPath := filepath.Join(t.TempDir(), "prompt.txt")
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"-t", "generic", "-s", "diagnose", "-o", outPath, "goal"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
	}

	if stdout.Len() != 0 {
		t.Errorf("expected stdout to be suppressed when -o is set, got:\n%s", stdout.String())
	}

	written, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", outPath, err)
	}
	if !bytes.Contains(written, []byte("pass/fail")) {
		t.Errorf("file contents missing diagnose body, got:\n%s", written)
	}

	info, err := os.Stat(outPath)
	if err != nil {
		t.Fatalf("Stat(%s) error = %v", outPath, err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("file perms = %o, want 0600 (prompt content may be sensitive - see gosec G306)", perm)
	}
}

func TestGenerate_CopyUsesClipboardSeamAndSuppressesStdout(t *testing.T) {
	var copied string
	restore := stubClipboard(t, func(s string) error {
		copied = s
		return nil
	})
	defer restore()

	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"-t", "generic", "-s", "diagnose", "-c", "goal"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
	}

	if stdout.Len() != 0 {
		t.Errorf("expected stdout to be suppressed when -c is set, got:\n%s", stdout.String())
	}
	if !strings.Contains(copied, "pass/fail") {
		t.Errorf("expected clipboard content to contain the diagnose body, got:\n%s", copied)
	}
	if !strings.Contains(stderr.String(), "copied to clipboard") {
		t.Errorf("expected a clipboard confirmation on stderr, got:\n%s", stderr.String())
	}
}

func TestGenerate_NoSkillsProducesGoalOnlyPromptWithStderrNote(t *testing.T) {
	reg := testRegistry(t)
	root := newRootCmd(reg)

	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"-t", "generic", "fix the flaky checkout test"}) // no -s

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
	}

	want := "<task>\nfix the flaky checkout test\n</task>"
	if !bytes.Contains(stdout.Bytes(), []byte(want)) {
		t.Errorf("stdout missing %q, got:\n%s", want, stdout.String())
	}
	if bytes.Contains(stdout.Bytes(), []byte("<approach>")) {
		t.Errorf("expected no <approach> section with no skills selected, got:\n%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "--skills") {
		t.Errorf("expected a stderr note mentioning --skills, got:\n%s", stderr.String())
	}
}

func TestGenerate_UnknownTargetWithNoSkills_ErrorsWithoutGoalOnlyNote(t *testing.T) {
	reg := testRegistry(t)
	root := newRootCmd(reg)

	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"-t", "does-not-exist", "goal"}) // no -s, and target is invalid

	if err := root.Execute(); err == nil {
		t.Fatal("Execute() error = nil, want an error for an unknown target")
	}
	if strings.Contains(stderr.String(), "--skills") {
		t.Errorf("expected no goal-only note when generation fails outright, got:\n%s", stderr.String())
	}
}
