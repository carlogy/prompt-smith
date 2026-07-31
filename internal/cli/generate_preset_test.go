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
	"strings"
	"testing"

	"github.com/carlogy/prompt-smith/internal/prompt"
	"github.com/carlogy/prompt-smith/internal/registry"
	"github.com/carlogy/prompt-smith/internal/server"
	"github.com/carlogy/prompt-smith/internal/tui"
)

// sharedTestPreset is used by most of the mapping tests below: every
// field is set to a distinct, greppable "preset ..." value so a test
// can assert on exactly one field at a time without the others'
// values colliding in a substring check. target/skills use "opencode"
// + "diagnose" specifically so their effect is observable via the
// reference-mode "Load the `diagnose` skill" signal (see
// prompt/build.go), which is otherwise untestable via a plain tag.
const sharedTestPreset = `
target: opencode
skills:
  - diagnose
role: preset role value
context: preset context value
constraints: preset constraints value
output_format: preset output format value
examples:
  - preset example value
`

func TestGenerate_PresetFieldsAppliedWhenFlagsAbsent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PROMPTSMITH_PRESETS_DIR", dir)
	writePreset(t, dir, "mypreset", sharedTestPreset)

	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"-p", "mypreset", "goal"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
	}

	out := stdout.String()
	// target=opencode + skills=[diagnose] together produce the
	// reference-mode signal - the only way to observe target/skills
	// precedence without a dedicated tag.
	if !strings.Contains(out, "Load the `diagnose` skill") {
		t.Errorf("expected the preset's target=opencode + skills=[diagnose] to produce reference-mode output, got:\n%s", out)
	}
	if !strings.Contains(out, "<role>\npreset role value\n</role>") {
		t.Errorf("expected the preset's role to flow through, got:\n%s", out)
	}
}

// TestGenerate_ExplicitTargetGenericBeatsPresetTarget is the exact test
// an empty-check implementation of applyPreset would fail:
// --target defaults to "generic" (see addGenerateFlags), so an
// empty-value check can't distinguish "the flag was never passed" from
// "the flag was explicitly passed as generic" - it would let the
// preset's target win either way. Only gating on
// cmd.Flags().Changed("target") gets this right.
func TestGenerate_ExplicitTargetGenericBeatsPresetTarget(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PROMPTSMITH_PRESETS_DIR", dir)
	writePreset(t, dir, "mypreset", "target: opencode\nskills:\n  - diagnose\n")

	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"-p", "mypreset", "--target", "generic", "goal"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
	}

	out := stdout.String()
	if strings.Contains(out, "Load the `diagnose` skill") {
		t.Errorf("expected an explicit --target generic to beat the preset's target=opencode, got:\n%s", out)
	}
	if !strings.Contains(out, "pass/fail") {
		t.Errorf("expected generic's inlined diagnose body, got:\n%s", out)
	}
}

// TestGenerate_AllSevenPresetMappingsFire is table-driven so a mistyped
// flagName in generate.go's presetFieldSpecs is caught mechanically:
// pflag's FlagSet.Changed returns false for a name that doesn't match
// any registered flag (see spf13/pflag's Changed doc), so a typo makes
// !cmd.Flags().Changed(typo) always true - the preset value would keep
// winning even when the override flag below IS passed. Each case's
// "override" assertion catches exactly that failure mode for its field.
func TestGenerate_AllSevenPresetMappingsFire(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PROMPTSMITH_PRESETS_DIR", dir)
	writePreset(t, dir, "mypreset", sharedTestPreset)

	run := func(t *testing.T, extraArgs ...string) string {
		t.Helper()
		reg := testRegistry(t)
		root := newRootCmd(reg)
		var stdout, stderr bytes.Buffer
		root.SetOut(&stdout)
		root.SetErr(&stderr)
		args := append([]string{"-p", "mypreset"}, extraArgs...)
		args = append(args, "goal")
		root.SetArgs(args)
		if err := root.Execute(); err != nil {
			t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
		}
		return stdout.String()
	}

	cases := []struct {
		name            string
		overrideArgs    []string
		presetSignal    string
		overrideSignal  string
		skipPresetCheck bool // target/skills share one combined signal, checked once
	}{
		{
			name:           "target",
			overrideArgs:   []string{"--target", "generic"},
			presetSignal:   "Load the `diagnose` skill",
			overrideSignal: "pass/fail",
		},
		{
			name:           "skills",
			overrideArgs:   []string{"--skills", "verify"},
			presetSignal:   "Load the `diagnose` skill",
			overrideSignal: "Load the `verify` skill",
		},
		{
			name:           "role",
			overrideArgs:   []string{"--role", "explicit role value"},
			presetSignal:   "<role>\npreset role value\n</role>",
			overrideSignal: "<role>\nexplicit role value\n</role>",
		},
		{
			name:           "context",
			overrideArgs:   []string{"--context", "explicit context value"},
			presetSignal:   "<context>\npreset context value\n</context>",
			overrideSignal: "<context>\nexplicit context value\n</context>",
		},
		{
			name:           "constraints",
			overrideArgs:   []string{"--constraints", "explicit constraints value"},
			presetSignal:   "<constraints>\npreset constraints value\n</constraints>",
			overrideSignal: "<constraints>\nexplicit constraints value\n</constraints>",
		},
		{
			name:           "output-format",
			overrideArgs:   []string{"--output-format", "explicit output format value"},
			presetSignal:   "<output_format>\npreset output format value\n</output_format>",
			overrideSignal: "<output_format>\nexplicit output format value\n</output_format>",
		},
		{
			name:           "example",
			overrideArgs:   []string{"--example", "explicit example value"},
			presetSignal:   "<example>\npreset example value\n</example>",
			overrideSignal: "<example>\nexplicit example value\n</example>",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withoutOverride := run(t)
			if !strings.Contains(withoutOverride, tc.presetSignal) {
				t.Errorf("without an explicit flag, expected preset signal %q, got:\n%s", tc.presetSignal, withoutOverride)
			}

			withOverride := run(t, tc.overrideArgs...)
			if !strings.Contains(withOverride, tc.overrideSignal) {
				t.Errorf("with %v, expected override signal %q, got:\n%s", tc.overrideArgs, tc.overrideSignal, withOverride)
			}
			if strings.Contains(withOverride, tc.presetSignal) {
				t.Errorf("with %v, expected the preset signal %q to be gone (explicit flag should win), got:\n%s", tc.overrideArgs, tc.presetSignal, withOverride)
			}
		})
	}
}

func TestGenerate_PresetSkillsSuppressNoSkillsStderrNote(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PROMPTSMITH_PRESETS_DIR", dir)
	writePreset(t, dir, "mypreset", "skills:\n  - diagnose\n")

	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"-t", "generic", "-p", "mypreset", "goal"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
	}
	if strings.Contains(stderr.String(), "--skills") {
		t.Errorf("expected the preset's skills to suppress the no-skills stderr note, got:\n%s", stderr.String())
	}
	if !strings.Contains(stdout.String(), "pass/fail") {
		t.Errorf("expected the preset's diagnose skill to be built into stdout, got:\n%s", stdout.String())
	}
}

func TestGenerate_UnknownPresetErrorListsAvailable(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PROMPTSMITH_PRESETS_DIR", dir)
	writePreset(t, dir, "standup", "role: x\n")
	writePreset(t, dir, "web-review", "role: x\n")

	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"-p", "web-reveiw", "goal"}) // typo'd name

	err := root.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want an error for an unknown preset")
	}
	msg := err.Error()
	if !strings.Contains(msg, `"web-reveiw"`) {
		t.Errorf("error = %q, want the bad name quoted", msg)
	}
	if !strings.Contains(msg, "standup") || !strings.Contains(msg, "web-review") {
		t.Errorf("error = %q, want the available names listed", msg)
	}
}

func TestGenerate_UnknownPresetWithEmptyDirCarriesGuidance(t *testing.T) {
	t.Setenv("PROMPTSMITH_PRESETS_DIR", t.TempDir())

	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"-p", "mypreset", "goal"})

	err := root.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want an error when no presets exist at all")
	}
	msg := err.Error()
	if !strings.Contains(msg, "PROMPTSMITH_PRESETS_DIR") {
		t.Errorf("error = %q, want the create-a-preset guidance", msg)
	}
	if !strings.Contains(msg, ".yaml") {
		t.Errorf("error = %q, want mention of the <name>.yaml layout", msg)
	}
}

func TestGenerate_PresetWarningsReachStderrWithPrefix(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PROMPTSMITH_PRESETS_DIR", dir)
	writePreset(t, dir, "mypreset", "skills:\n  - diagnose\nrole: x\ngoal: this is not a preset field\n")

	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"-p", "mypreset", "goal"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
	}
	want := `promptsmith: preset "mypreset": "goal" is not a preset field - a preset describes how to ask, not what to ask`
	if !strings.Contains(stderr.String(), want) {
		t.Errorf("stderr missing %q, got:\n%s", want, stderr.String())
	}
}

func TestGenerate_PresetSeedsTUIInitialInputs(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PROMPTSMITH_PRESETS_DIR", dir)
	writePreset(t, dir, "mypreset", "role: preset role value\nskills:\n  - diagnose\n")

	defer stubInteractive(t, true)()
	var gotInputs prompt.Inputs
	defer stubRunTUI(t, func(reg *registry.Registry, in prompt.Inputs, _ tui.Options) (tui.Result, error) {
		gotInputs = in
		return tui.Result{Inputs: in, Action: tui.ActionCancel}, nil
	})()

	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"-p", "mypreset", "--tui", "goal"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
	}
	if gotInputs.Role != "preset role value" {
		t.Errorf("TUI's initial Inputs.Role = %q, want %q", gotInputs.Role, "preset role value")
	}
	if len(gotInputs.Skills) != 1 || gotInputs.Skills[0] != "diagnose" {
		t.Errorf("TUI's initial Inputs.Skills = %v, want [diagnose]", gotInputs.Skills)
	}
}

// TestPresetFieldSpecs_EveryEntryHasAllThreeFuncs supersedes
// generate_save_preset_test.go's now-removed
// TestPresetFieldSpecs_EveryEntryHasBothFuncs, covering the third
// (fromInputs) direction added for tui.ActionSavePreset alongside the
// original apply/collect pair. A nil apply or collect func in any
// entry would panic at runtime the first time that entry's direction
// is exercised (applyPreset for apply, collectPresetFromOpts for
// collect); a nil fromInputs func would likewise panic the first time
// collectPresetFromInputs reaches it (i.e. the first real
// ActionSavePreset save) - rather than failing loudly at compile time,
// since table literals don't enforce "every field must be set".
func TestPresetFieldSpecs_EveryEntryHasAllThreeFuncs(t *testing.T) {
	for _, spec := range presetFieldSpecs {
		if spec.apply == nil {
			t.Errorf("presetFieldSpecs[%q].apply is nil", spec.flagName)
		}
		if spec.collect == nil {
			t.Errorf("presetFieldSpecs[%q].collect is nil", spec.flagName)
		}
		if spec.fromInputs == nil {
			t.Errorf("presetFieldSpecs[%q].fromInputs is nil", spec.flagName)
		}
	}
}

func TestGenerate_PresetSeedsUIInitialInputs(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PROMPTSMITH_PRESETS_DIR", dir)
	writePreset(t, dir, "mypreset", "role: preset role value\nskills:\n  - diagnose\n")

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
	root.SetArgs([]string{"-p", "mypreset", "--ui"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
	}
	if gotOpts.Initial.Role != "preset role value" {
		t.Errorf("server.Options.Initial.Role = %q, want %q", gotOpts.Initial.Role, "preset role value")
	}
	if len(gotOpts.Initial.Skills) != 1 || gotOpts.Initial.Skills[0] != "diagnose" {
		t.Errorf("server.Options.Initial.Skills = %v, want [diagnose]", gotOpts.Initial.Skills)
	}
}
