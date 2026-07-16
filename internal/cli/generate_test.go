package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/carlogy/prompt-smith/internal/prompt"
	"github.com/carlogy/prompt-smith/internal/registry"
	"github.com/carlogy/prompt-smith/internal/tui"
)

// stubClipboard substitutes copyToClipboard with fn for the duration of
// the calling test, restoring the original on cleanup.
func stubClipboard(t *testing.T, fn func(string) error) func() {
	t.Helper()
	original := copyToClipboard
	copyToClipboard = fn
	return func() { copyToClipboard = original }
}

// stubInteractive forces isInteractive() to return val for the duration
// of the calling test, restoring the original on cleanup. Used so gate
// tests never depend on whether the test runner's own stdio happens to
// be a terminal.
func stubInteractive(t *testing.T, val bool) func() {
	t.Helper()
	original := isInteractive
	isInteractive = func() bool { return val }
	return func() { isInteractive = original }
}

// stubRunTUI substitutes the tui.Run seam with fn for the duration of
// the calling test, so gate tests never launch a real Bubble Tea program
// (which would block reading real stdin).
func stubRunTUI(t *testing.T, fn func(*registry.Registry, prompt.Inputs) (tui.Result, error)) func() {
	t.Helper()
	original := runTUIFunc
	runTUIFunc = fn
	return func() { runTUIFunc = original }
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

func TestGenerate_QuickAndTUIFlagsParse(t *testing.T) {
	// With --skills given and no --tui override, the gate always skips
	// the picker regardless of -q/interactivity - this just locks that
	// the flags parse cleanly alongside the rest of the surface.
	reg := testRegistry(t)
	root := newRootCmd(reg)

	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"-t", "generic", "-s", "diagnose", "-q", "goal"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "<task>") {
		t.Errorf("expected normal generation to still work, got:\n%s", stdout.String())
	}
}

func TestGenerate_TUI_StdoutAction(t *testing.T) {
	defer stubInteractive(t, true)()
	defer stubRunTUI(t, func(reg *registry.Registry, in prompt.Inputs) (tui.Result, error) {
		in.Skills = []string{"diagnose"}
		return tui.Result{Inputs: in, Action: tui.ActionStdout}, nil
	})()

	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"-t", "generic", "goal"}) // no -s -> interactive + bare -> TUI

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "pass/fail") {
		t.Errorf("expected the TUI's chosen skill to be built into stdout, got:\n%s", stdout.String())
	}
}

func TestGenerate_TUI_CancelProducesNoOutputAndNoError(t *testing.T) {
	defer stubInteractive(t, true)()
	defer stubRunTUI(t, func(reg *registry.Registry, in prompt.Inputs) (tui.Result, error) {
		return tui.Result{Action: tui.ActionCancel}, nil
	})()

	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"-t", "generic", "goal"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, want nil on cancel, stderr = %s", err, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("expected no stdout on cancel, got:\n%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "cancel") {
		t.Errorf("expected a cancellation note on stderr, got:\n%s", stderr.String())
	}
}

func TestGenerate_TUI_CopyAction(t *testing.T) {
	defer stubInteractive(t, true)()
	defer stubRunTUI(t, func(reg *registry.Registry, in prompt.Inputs) (tui.Result, error) {
		in.Skills = []string{"diagnose"}
		return tui.Result{Inputs: in, Action: tui.ActionCopy}, nil
	})()

	var copied string
	defer stubClipboard(t, func(s string) error { copied = s; return nil })()

	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"-t", "generic", "goal"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("expected no stdout when the TUI chose copy, got:\n%s", stdout.String())
	}
	if !strings.Contains(copied, "pass/fail") {
		t.Errorf("expected the built prompt to reach the clipboard, got:\n%s", copied)
	}
}

func TestGenerate_TUI_WriteAction(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "from-tui.txt")

	defer stubInteractive(t, true)()
	defer stubRunTUI(t, func(reg *registry.Registry, in prompt.Inputs) (tui.Result, error) {
		in.Skills = []string{"diagnose"}
		return tui.Result{Inputs: in, Action: tui.ActionWrite, WritePath: outPath}, nil
	})()

	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"-t", "generic", "goal"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
	}

	written, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", outPath, err)
	}
	if !bytes.Contains(written, []byte("pass/fail")) {
		t.Errorf("expected the built prompt in the written file, got:\n%s", written)
	}
	info, err := os.Stat(outPath)
	if err != nil {
		t.Fatalf("Stat(%s) error = %v", outPath, err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("file perms = %o, want 0600 (same guarantee as the flag-only -o path)", perm)
	}
}

func TestGenerate_TUI_LaunchesWithEmptyGoalWhenBare(t *testing.T) {
	// As of P3c, the picker collects the goal inline (focused on the
	// goal field by default) - bare promptsmith no longer errors.
	var receivedGoal string
	called := false
	defer stubInteractive(t, true)()
	defer stubRunTUI(t, func(reg *registry.Registry, in prompt.Inputs) (tui.Result, error) {
		called = true
		receivedGoal = in.Goal
		// Simulate the picker collecting a goal before confirming.
		in.Goal = "typed in the picker"
		in.Skills = []string{"diagnose"}
		return tui.Result{Inputs: in, Action: tui.ActionStdout}, nil
	})()

	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"-t", "generic"}) // no goal, TTY, bare -> TUI

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
	}
	if !called {
		t.Fatal("expected runTUIFunc to be called even with no goal argument")
	}
	if receivedGoal != "" {
		t.Errorf("initial goal passed to the TUI = %q, want empty (the picker collects it)", receivedGoal)
	}
	if !strings.Contains(stdout.String(), "typed in the picker") {
		t.Errorf("expected the goal collected in the picker to reach the built prompt, got:\n%s", stdout.String())
	}
}

func TestGenerate_QuickSkipsTUIEvenWhenInteractive(t *testing.T) {
	defer stubInteractive(t, true)()
	defer stubRunTUI(t, func(reg *registry.Registry, in prompt.Inputs) (tui.Result, error) {
		t.Fatal("runTUIFunc should not be called when --quick is set")
		return tui.Result{}, nil
	})()

	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"-t", "generic", "-q", "goal"}) // interactive, but --quick

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "<task>") {
		t.Errorf("expected the flag path (goal-only) to run, got:\n%s", stdout.String())
	}
}

func TestGenerate_TUIFlagForcesPickerEvenWithSkills(t *testing.T) {
	called := false
	defer stubInteractive(t, true)()
	defer stubRunTUI(t, func(reg *registry.Registry, in prompt.Inputs) (tui.Result, error) {
		called = true
		if len(in.Skills) != 1 || in.Skills[0] != "diagnose" {
			t.Errorf("expected --skills to pre-populate the TUI's initial Inputs, got %v", in.Skills)
		}
		return tui.Result{Inputs: in, Action: tui.ActionStdout}, nil
	})()

	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"-t", "generic", "-s", "diagnose", "--tui", "goal"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
	}
	if !called {
		t.Error("expected --tui to force runTUIFunc to be called even with --skills given")
	}
}

func TestGenerate_TUIAndQuickTogetherErrors(t *testing.T) {
	defer stubInteractive(t, true)()
	defer stubRunTUI(t, func(reg *registry.Registry, in prompt.Inputs) (tui.Result, error) {
		t.Fatal("runTUIFunc should not be called when --tui and --quick conflict")
		return tui.Result{}, nil
	})()

	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"-t", "generic", "-s", "diagnose", "--tui", "-q", "goal"})

	if err := root.Execute(); err == nil {
		t.Fatal("Execute() error = nil, want an error: --tui and --quick are mutually exclusive")
	}
}

func TestGenerate_ShortAliasesMatchLongForms(t *testing.T) {
	cases := []struct {
		name        string
		short       string
		long        string
		value       string
		wantSection string
	}{
		{"role", "-r", "--role", "a senior engineer", "role"},
		{"output-format", "-f", "--output-format", "a diff", "output_format"},
		{"context", "-x", "--context", "some context", "context"},
		{"constraints", "-C", "--constraints", "no new deps", "constraints"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			run := func(flag string) string {
				reg := testRegistry(t)
				root := newRootCmd(reg)
				var stdout, stderr bytes.Buffer
				root.SetOut(&stdout)
				root.SetErr(&stderr)
				root.SetArgs([]string{"-t", "generic", "-s", "diagnose", flag, tc.value, "goal"})
				if err := root.Execute(); err != nil {
					t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
				}
				return stdout.String()
			}

			short := run(tc.short)
			long := run(tc.long)
			if short != long {
				t.Errorf("%s: short-flag output != long-flag output\nshort: %q\nlong:  %q", tc.name, short, long)
			}

			wantTag := "<" + tc.wantSection + ">"
			if !strings.Contains(short, wantTag) {
				t.Errorf("%s: expected output to contain %q, got:\n%s", tc.name, wantTag, short)
			}
		})
	}
}
