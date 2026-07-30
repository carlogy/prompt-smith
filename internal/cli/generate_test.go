// promptsmith - generate portable, skill-aware prompts for any LLM or agent harness.
// Copyright (C) 2026 carlogy
// SPDX-License-Identifier: AGPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/carlogy/prompt-smith/internal/prompt"
	"github.com/carlogy/prompt-smith/internal/registry"
	"github.com/carlogy/prompt-smith/internal/server"
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

// stubRunServer substitutes the server.Serve seam with fn for the
// duration of the calling test, so --ui tests never bind a real port
// or open a browser.
func stubRunServer(t *testing.T, fn func(context.Context, *registry.Registry, server.Options) error) func() {
	t.Helper()
	original := runServerFunc
	runServerFunc = fn
	return func() { runServerFunc = original }
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
	// Windows has no Unix permission bits - any writable file reports
	// 0666 regardless of the mode passed to WriteFile - so this
	// guarantee is only meaningful, and only checked, on Unix.
	if runtime.GOOS != "windows" {
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("file perms = %o, want 0600 (prompt content may be sensitive - see gosec G306)", perm)
		}
	}
}

func TestGenerate_OutExpandsTildeAndCreatesMissingDirs(t *testing.T) {
	// os.UserHomeDir() reads $HOME on Unix but %USERPROFILE% on
	// Windows, so pointing the OS-appropriate var at a temp dir lets
	// this exercise the real expansion path (on every OS) without
	// touching the developer's actual home directory.
	fakeHome := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", fakeHome)
	} else {
		t.Setenv("HOME", fakeHome)
	}

	reg := testRegistry(t)
	root := newRootCmd(reg)

	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"-t", "generic", "-s", "diagnose", "-o", "~/nested/dir/prompt.txt", "goal"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
	}

	wantPath := filepath.Join(fakeHome, "nested", "dir", "prompt.txt")
	written, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v (expected ~ expansion and missing parent dirs to be created)", wantPath, err)
	}
	if !bytes.Contains(written, []byte("pass/fail")) {
		t.Errorf("file contents missing diagnose body, got:\n%s", written)
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
	root.SetArgs([]string{"-t", "generic", "-s", "diagnose", "-y", "goal"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
	}

	if stdout.Len() != 0 {
		t.Errorf("expected stdout to be suppressed when -y is set, got:\n%s", stdout.String())
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
	if strings.Contains(stderr.String(), "later release") {
		t.Errorf("expected the stale 'later release' claim to be gone, got:\n%s", stderr.String())
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
	// See the identical guard + comment in
	// TestGenerate_OutWritesFileAndSuppressesStdout - meaningless on
	// Windows, which has no Unix permission bits.
	if runtime.GOOS != "windows" {
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("file perms = %o, want 0600 (same guarantee as the flag-only -o path)", perm)
		}
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
		{"constraints", "-c", "--constraints", "no new deps", "constraints"},
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

func TestGenerate_UIFlagInvokesServerWithDefaults(t *testing.T) {
	var gotOpts server.Options
	called := false
	defer stubRunServer(t, func(ctx context.Context, reg *registry.Registry, opts server.Options) error {
		called = true
		gotOpts = opts
		return nil
	})()

	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"--ui"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
	}
	if !called {
		t.Fatal("expected --ui to invoke the server seam")
	}
	if gotOpts.Port != 0 {
		t.Errorf("Port = %d, want 0 (OS-assigned) by default", gotOpts.Port)
	}
	if gotOpts.NoBrowser {
		t.Error("NoBrowser = true, want false by default")
	}
	if gotOpts.Stdout != &stdout {
		t.Error("Stdout wasn't wired to cmd.OutOrStdout()")
	}
}

func TestGenerate_UIWithPortAndNoBrowser(t *testing.T) {
	var gotOpts server.Options
	defer stubRunServer(t, func(ctx context.Context, reg *registry.Registry, opts server.Options) error {
		gotOpts = opts
		return nil
	})()

	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"--ui", "--port", "9999", "--no-browser"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
	}
	if gotOpts.Port != 9999 {
		t.Errorf("Port = %d, want 9999", gotOpts.Port)
	}
	if !gotOpts.NoBrowser {
		t.Error("NoBrowser = false, want true")
	}
}

func TestGenerate_UIDoesNotRequireATTY(t *testing.T) {
	// Unlike --tui, --ui has nothing to do with the calling process's
	// own stdio - "open a browser" works the same whether or not
	// promptsmith itself is running interactively.
	defer stubInteractive(t, false)()

	called := false
	defer stubRunServer(t, func(ctx context.Context, reg *registry.Registry, opts server.Options) error {
		called = true
		return nil
	})()

	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"--ui"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
	}
	if !called {
		t.Error("expected --ui to invoke the server seam even with no TTY")
	}
}

func TestGenerate_UIPropagatesServerError(t *testing.T) {
	wantErr := errors.New("listen: address already in use")
	defer stubRunServer(t, func(ctx context.Context, reg *registry.Registry, opts server.Options) error {
		return wantErr
	})()

	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"--ui"})

	err := root.Execute()
	if !errors.Is(err, wantErr) {
		t.Errorf("Execute() error = %v, want %v", err, wantErr)
	}
}

func TestGenerate_UIConflictingFlagsError(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"--ui and --tui", []string{"--ui", "--tui"}},
		{"--ui and --quick", []string{"--ui", "--quick"}},
		{"--ui and --copy", []string{"--ui", "--copy"}},
		{"--ui and --out", []string{"--ui", "--out", "x.txt"}},
		{"--port without --ui", []string{"--port", "9999", "-s", "diagnose", "goal"}},
		{"--no-browser without --ui", []string{"--no-browser", "-s", "diagnose", "goal"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			defer stubRunServer(t, func(ctx context.Context, reg *registry.Registry, opts server.Options) error {
				called = true
				return nil
			})()

			reg := testRegistry(t)
			root := newRootCmd(reg)
			var stdout, stderr bytes.Buffer
			root.SetOut(&stdout)
			root.SetErr(&stderr)
			root.SetArgs(tc.args)

			if err := root.Execute(); err == nil {
				t.Fatal("Execute() error = nil, want an error for conflicting flags")
			}
			if called {
				t.Error("expected the server seam to never be invoked when flags conflict")
			}
		})
	}
}

func TestGenerate_UISeedsInitialInputsFromFlagsAndArgs(t *testing.T) {
	// --ui seeds the page's form exactly like --tui pre-populates the
	// picker, from the same flags/goal.
	var gotOpts server.Options
	defer stubRunServer(t, func(ctx context.Context, reg *registry.Registry, opts server.Options) error {
		gotOpts = opts
		return nil
	})()

	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{
		"--ui",
		"-t", "opencode",
		"-s", "diagnose,verify",
		"--role", "a seeded role",
		"--context", "seeded context",
		"--constraints", "seeded constraints",
		"--output-format", "seeded output format",
		"my seeded goal",
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
	}

	want := prompt.Inputs{
		Target:       "opencode",
		Skills:       []string{"diagnose", "verify"},
		Goal:         "my seeded goal",
		Role:         "a seeded role",
		Context:      "seeded context",
		Constraints:  "seeded constraints",
		OutputFormat: "seeded output format",
	}
	got := gotOpts.Initial
	if !slices.Equal(got.Skills, want.Skills) {
		t.Errorf("Initial.Skills = %v, want %v", got.Skills, want.Skills)
	}
	if got.Target != want.Target || got.Goal != want.Goal || got.Role != want.Role ||
		got.Context != want.Context || got.Constraints != want.Constraints || got.OutputFormat != want.OutputFormat {
		t.Errorf("Initial = %+v, want %+v", got, want)
	}
}

// TestGenerate_DefaultDeliveryGoesToRealStdoutNotStderr is a regression
// test for a bug where the flag-only delivery path's fallback print
// (deliver's "nothing else requested, print the prompt") used
// cmd.Println, which resolves via cobra's OutOrStderr() - stdout only
// if something already called SetOut, stderr otherwise. Production
// never calls SetOut, so every generated prompt went to stderr,
// silently breaking `promptsmith "goal" > file` and `| pbcopy`.
//
// It deliberately does NOT call root.SetOut/SetErr: doing so would
// mask the bug rather than reproduce it (see captureRealStdio's
// comment). Instead it runs Execute() exactly as production's
// Execute() does and inspects the real os.Stdout/os.Stderr streams.
func TestGenerate_DefaultDeliveryGoesToRealStdoutNotStderr(t *testing.T) {
	reg := testRegistry(t)
	root := newRootCmd(reg)
	root.SetArgs([]string{"-t", "generic", "-s", "diagnose", "fix the flaky checkout test"})

	var execErr error
	stdout, stderr := captureRealStdio(t, func() {
		execErr = root.Execute()
	})

	if execErr != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", execErr, stderr)
	}

	want := "<task>\nfix the flaky checkout test\n</task>"
	if !strings.Contains(stdout, want) {
		t.Errorf("real stdout missing %q, got:\n%s", want, stdout)
	}
	if strings.Contains(stderr, want) {
		t.Errorf("the generated prompt leaked onto real stderr:\n%s", stderr)
	}
}

// TestGenerate_TUIStdoutActionGoesToRealStdoutNotStderr is the same
// regression, but for the TUI's ActionStdout delivery path (the other
// cmd.Println(out) call site). See
// TestGenerate_DefaultDeliveryGoesToRealStdoutNotStderr for why this
// uses captureRealStdio instead of SetOut/SetErr.
func TestGenerate_TUIStdoutActionGoesToRealStdoutNotStderr(t *testing.T) {
	defer stubInteractive(t, true)()
	defer stubRunTUI(t, func(reg *registry.Registry, in prompt.Inputs) (tui.Result, error) {
		in.Skills = []string{"diagnose"}
		return tui.Result{Inputs: in, Action: tui.ActionStdout}, nil
	})()

	reg := testRegistry(t)
	root := newRootCmd(reg)
	root.SetArgs([]string{"-t", "generic", "goal"}) // no -s -> interactive + bare -> TUI

	var execErr error
	stdout, stderr := captureRealStdio(t, func() {
		execErr = root.Execute()
	})

	if execErr != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", execErr, stderr)
	}
	if !strings.Contains(stdout, "pass/fail") {
		t.Errorf("real stdout missing the TUI's chosen skill body, got:\n%s", stdout)
	}
	if strings.Contains(stderr, "pass/fail") {
		t.Errorf("the generated prompt leaked onto real stderr:\n%s", stderr)
	}
}

func TestResolveGoal(t *testing.T) {
	cases := []struct {
		name     string
		flagGoal string
		args     []string
		want     string
		wantErr  error
	}{
		{"flag only", "fix the flaky checkout test", nil, "fix the flaky checkout test", nil},
		{"positional only", "", []string{"fix", "the", "flaky", "checkout", "test"}, "fix the flaky checkout test", nil},
		{"multi-word positional joined with single spaces", "", []string{"fix", "the", "bug"}, "fix the bug", nil},
		{"both set -> conflict", "fix the bug", []string{"fix", "the", "bug"}, "", errGoalConflict},
		{"neither set -> empty, no error", "", nil, "", nil},
		{"whitespace-only flag -> empty, no error", "   ", nil, "", nil},
		{"whitespace-only flag plus positional -> positional wins, no conflict", "   ", []string{"fix", "the", "bug"}, "fix the bug", nil},
		{"flag with surrounding whitespace is trimmed", "  fix the bug  ", nil, "fix the bug", nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveGoal(tc.flagGoal, tc.args)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("resolveGoal() error = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveGoal() error = %v, want nil", err)
			}
			if got != tc.want {
				t.Errorf("resolveGoal() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestGenerate_GoalFlagMatchesPositionalGoal(t *testing.T) {
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

	short := run([]string{"-t", "generic", "-s", "diagnose", "-g", "fix the flaky checkout test"})
	long := run([]string{"-t", "generic", "-s", "diagnose", "--goal", "fix the flaky checkout test"})
	positional := run([]string{"-t", "generic", "-s", "diagnose", "fix the flaky checkout test"})

	if short != long || short != positional {
		t.Errorf("-g, --goal, and positional produced different output:\n-g:         %q\n--goal:     %q\npositional: %q", short, long, positional)
	}
}

func TestGenerate_GoalFlagAndPositionalTogetherError(t *testing.T) {
	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"-t", "generic", "-s", "diagnose", "-g", "fix the bug", "fix the bug"})

	err := root.Execute()
	if !errors.Is(err, errGoalConflict) {
		t.Errorf("Execute() error = %v, want errGoalConflict", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("expected no stdout on a goal conflict, got:\n%s", stdout.String())
	}
}

func TestGenerate_GoalFlagWhitespaceOnlyErrors(t *testing.T) {
	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"-t", "generic", "-s", "diagnose", "-g", "   "})

	err := root.Execute()
	if !errors.Is(err, errEmptyGoal) {
		t.Errorf("Execute() error = %v, want errEmptyGoal", err)
	}
}

func TestGenerate_RemovedShorthandsError(t *testing.T) {
	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"-t", "generic", "-s", "diagnose", "-C", "no new deps", "goal"})

	err := root.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want an error for the removed -C shorthand")
	}
	if !strings.Contains(err.Error(), "unknown shorthand flag") {
		t.Errorf("Execute() error = %v, want it to mention 'unknown shorthand flag'", err)
	}
}

func TestGenerate_ConstraintsShorthandNoLongerCopies(t *testing.T) {
	clipboardCalled := false
	defer stubClipboard(t, func(s string) error {
		clipboardCalled = true
		return nil
	})()

	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"-t", "generic", "-s", "diagnose", "-c", "no new deps", "goal"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
	}

	want := "<constraints>\nno new deps\n</constraints>"
	if !strings.Contains(stdout.String(), want) {
		t.Errorf("stdout missing %q, got:\n%s", want, stdout.String())
	}
	if clipboardCalled {
		t.Error("expected -c to set constraints, not invoke the clipboard seam")
	}
}

func TestGenerate_SkillsWithSpacesAfterCommasResolve(t *testing.T) {
	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"-t", "generic", "-s", "diagnose, verify", "goal"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "pass/fail") {
		t.Errorf("stdout missing diagnose body, got:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "meaningful change") {
		t.Errorf("stdout missing verify body, got:\n%s", stdout.String())
	}
}

// TestGenerate_WarnsOnSkillNamesParsedAsGoal pins the shell-splitting
// failure mode that motivated warnStraySkillArgs: an unquoted, spaced
// --skills list (`-s a, b, c`) gets CSV-split into "a" by --skills
// while the shell hands "b," and "c" to promptsmith as ordinary
// positional args, which used to silently join the goal text. The old
// behavior hard-errored on the accompanying blank id ("unknown skill
// \"\""); this pins that the command now succeeds, warns on stderr
// naming the stray skill-shaped words, and builds a prompt using only
// the skill(s) that actually landed in --skills.
func TestGenerate_WarnsOnSkillNamesParsedAsGoal(t *testing.T) {
	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"-t", "opencode", "-s", "diagnose,", "verify,", "tdd", "refactor the parser"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, want nil (the old unknown-skill abort must be gone), stderr = %s", err, stderr.String())
	}

	if !strings.Contains(stderr.String(), "verify") || !strings.Contains(stderr.String(), "tdd") {
		t.Errorf("expected stderr to name both stray skill-shaped args, got:\n%s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "no spaces") {
		t.Errorf("expected stderr to include the corrective hint about no spaces, got:\n%s", stderr.String())
	}

	// Isolate the <approach> section: the goal text here (itself the
	// stray positional args, e.g. "verify, tdd refactor the parser")
	// legitimately contains "verify"/"tdd" inside <task>, so asserting
	// their absence has to be scoped to <approach>, not the whole
	// output.
	approach, ok := betweenTags(stdout.String(), "approach")
	if !ok {
		t.Fatalf("stdout missing an <approach> section, got:\n%s", stdout.String())
	}
	if !strings.Contains(approach, "Load the `diagnose` skill") {
		t.Errorf("<approach> missing diagnose's reference content, got:\n%s", approach)
	}
	if strings.Contains(approach, "verify") || strings.Contains(approach, "tdd") {
		t.Errorf("expected <approach> to have no verify/tdd content (they were never in --skills), got:\n%s", approach)
	}
}

// betweenTags returns the text strictly between "<tag>\n" and "\n</tag>"
// in s, and whether the tag was found at all.
func betweenTags(s, tag string) (string, bool) {
	open := "<" + tag + ">\n"
	closeTag := "\n</" + tag + ">"
	start := strings.Index(s, open)
	if start == -1 {
		return "", false
	}
	start += len(open)
	end := strings.Index(s[start:], closeTag)
	if end == -1 {
		return "", false
	}
	return s[start : start+end], true
}

func TestGenerate_NoStrayWarningWhenGoalNamesASelectedSkill(t *testing.T) {
	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"-t", "generic", "-s", "verify", "add a verify step to CI"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
	}
	if strings.Contains(stderr.String(), "warning:") {
		t.Errorf("expected no stray-skill warning when the goal merely names an already-selected skill, got:\n%s", stderr.String())
	}
}

func TestGenerate_NoStrayWarningWithoutSkillsFlag(t *testing.T) {
	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"-t", "generic", "run the verify skill"}) // no -s

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
	}
	if strings.Contains(stderr.String(), "warning:") {
		t.Errorf("expected no stray-skill warning with no --skills given (stderr will still have the no-skills note), got:\n%s", stderr.String())
	}
}

func TestGenerate_TUISeedsGoalFromGoalFlag(t *testing.T) {
	defer stubInteractive(t, true)()

	var gotGoal string
	defer stubRunTUI(t, func(reg *registry.Registry, in prompt.Inputs) (tui.Result, error) {
		gotGoal = in.Goal
		in.Skills = []string{"diagnose"}
		return tui.Result{Inputs: in, Action: tui.ActionStdout}, nil
	})()

	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"--tui", "-g", "fix the flaky checkout test"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
	}
	if gotGoal != "fix the flaky checkout test" {
		t.Errorf("prompt.Inputs.Goal = %q, want %q", gotGoal, "fix the flaky checkout test")
	}
}

func TestGenerate_UISeedsGoalFromGoalFlag(t *testing.T) {
	var gotOpts server.Options
	defer stubRunServer(t, func(ctx context.Context, reg *registry.Registry, opts server.Options) error {
		gotOpts = opts
		return nil
	})()

	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"--ui", "-g", "fix the flaky checkout test"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
	}
	if gotOpts.Initial.Goal != "fix the flaky checkout test" {
		t.Errorf("server.Options.Initial.Goal = %q, want %q", gotOpts.Initial.Goal, "fix the flaky checkout test")
	}
}

func TestGenerate_ExampleFlagRepeatable(t *testing.T) {
	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{
		"-t", "generic", "-s", "diagnose",
		"-e", "input: 1 -> output: one",
		"-e", "input: 2 -> output: two",
		"goal",
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
	}

	for _, want := range []string{
		"<example>\ninput: 1 -> output: one\n</example>",
		"<example>\ninput: 2 -> output: two\n</example>",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("stdout missing %q, got:\n%s", want, stdout.String())
		}
	}
	if n := strings.Count(stdout.String(), "<example>"); n != 2 {
		t.Errorf("expected exactly 2 <example> blocks, got %d in:\n%s", n, stdout.String())
	}
}

// TestGenerate_ExampleWithCommaIsNotSplit is the key regression test for
// StringArrayVarP over StringSliceVarP: StringSlice treats its value as
// CSV, so a single -e containing commas would silently fragment into
// multiple broken examples with no error. StringArray appends the raw
// value untouched, so one -e stays one <example>, comma and all.
func TestGenerate_ExampleWithCommaIsNotSplit(t *testing.T) {
	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{
		"-t", "generic", "-s", "diagnose",
		"-e", "input: a, b, c -> output: 3",
		"goal",
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
	}

	if n := strings.Count(stdout.String(), "<example>"); n != 1 {
		t.Errorf("expected exactly 1 <example> block (comma must not split it), got %d in:\n%s", n, stdout.String())
	}
	want := "<example>\ninput: a, b, c -> output: 3\n</example>"
	if !strings.Contains(stdout.String(), want) {
		t.Errorf("stdout missing the full comma-bearing example %q, got:\n%s", want, stdout.String())
	}
}

func TestGenerate_ExampleShortAndLongFormsMatch(t *testing.T) {
	run := func(flag string) string {
		reg := testRegistry(t)
		root := newRootCmd(reg)
		var stdout, stderr bytes.Buffer
		root.SetOut(&stdout)
		root.SetErr(&stderr)
		root.SetArgs([]string{"-t", "generic", "-s", "diagnose", flag, "input: 1 -> output: one", "goal"})
		if err := root.Execute(); err != nil {
			t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
		}
		return stdout.String()
	}

	short := run("-e")
	long := run("--example")
	if short != long {
		t.Errorf("-e and --example produced different output:\n-e:       %q\n--example: %q", short, long)
	}
}

func TestGenerate_NoExamplesOmitsSection(t *testing.T) {
	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"-t", "generic", "-s", "diagnose", "goal"}) // no -e

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
	}
	if strings.Contains(stdout.String(), "<examples>") {
		t.Errorf("expected no <examples> section with no -e given, got:\n%s", stdout.String())
	}
}

func TestGenerate_ExamplesSeedTUI(t *testing.T) {
	defer stubInteractive(t, true)()

	var gotExamples []string
	defer stubRunTUI(t, func(reg *registry.Registry, in prompt.Inputs) (tui.Result, error) {
		gotExamples = in.Examples
		in.Skills = []string{"diagnose"}
		return tui.Result{Inputs: in, Action: tui.ActionStdout}, nil
	})()

	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{
		"--tui",
		"-e", "input: 1 -> output: one",
		"-e", "input: 2 -> output: two",
		"-g", "fix the flaky checkout test",
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
	}

	want := []string{"input: 1 -> output: one", "input: 2 -> output: two"}
	if !slices.Equal(gotExamples, want) {
		t.Errorf("TUI's initial Inputs.Examples = %v, want %v", gotExamples, want)
	}
}

func TestGenerate_ExamplesSeedUI(t *testing.T) {
	// Mirrors TestGenerate_UISeedsInitialInputsFromFlagsAndArgs, scoped
	// to Examples.
	var gotOpts server.Options
	defer stubRunServer(t, func(ctx context.Context, reg *registry.Registry, opts server.Options) error {
		gotOpts = opts
		return nil
	})()

	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"--ui", "-e", "input: 1 -> output: one"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
	}

	want := []string{"input: 1 -> output: one"}
	if !slices.Equal(gotOpts.Initial.Examples, want) {
		t.Errorf("server.Options.Initial.Examples = %v, want %v", gotOpts.Initial.Examples, want)
	}
}

func TestGenerate_WhitespaceOnlyExampleDropped(t *testing.T) {
	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{
		"-t", "generic", "-s", "diagnose",
		"-e", "   ",
		"-e", "input: 1 -> output: one",
		"goal",
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
	}
	if n := strings.Count(stdout.String(), "<example>"); n != 1 {
		t.Errorf("expected exactly 1 <example> block (the whitespace-only one dropped), got %d in:\n%s", n, stdout.String())
	}
}

func TestGenerate_SkillsAllEmptyFallsBackToGoalOnly(t *testing.T) {
	defer stubInteractive(t, false)() // picker must not launch

	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"-t", "generic", "-s", ",", "goal"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
	}
	if strings.Contains(stdout.String(), "<approach>") {
		t.Errorf("expected no <approach> section when --skills normalizes to empty, got:\n%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "--skills") {
		t.Errorf("expected the no-skills note on stderr, got:\n%s", stderr.String())
	}
}
