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
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/carlogy/prompt-smith/internal/preset"
	"github.com/carlogy/prompt-smith/internal/prompt"
	"github.com/carlogy/prompt-smith/internal/registry"
	"github.com/carlogy/prompt-smith/internal/tui"
)

// TestGenerate_TUI_SavePresetAction_RoundTripsThroughRealLoaderWithZeroWarnings
// mirrors generate_save_preset_test.go's
// TestGenerate_SavePresetRoundTripsThroughRealLoaderWithZeroWarnings one
// surface up: this is the assertion that proves the fromInputs
// direction added to presetFieldSpecs agrees with LoadFS's own
// decoding, exactly as that test proves apply/collect agree for the
// --save-preset flag. Zero warnings is the load-bearing half - a stray
// "goal" key or wrong extension would each independently surface as a
// warning.
func TestGenerate_TUI_SavePresetAction_RoundTripsThroughRealLoaderWithZeroWarnings(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PROMPTSMITH_PRESETS_DIR", dir)

	defer stubInteractive(t, true)()
	defer stubRunTUI(t, func(reg *registry.Registry, in prompt.Inputs, _ tui.Options) (tui.Result, error) {
		return tui.Result{
			Inputs: prompt.Inputs{
				Target:       "opencode",
				Skills:       []string{"diagnose"},
				Role:         "senior reviewer",
				Context:      "reviewing a web app PR",
				Constraints:  "no new dependencies",
				OutputFormat: "markdown checklist",
				Examples:     []string{"example one"},
			},
			Action:     tui.ActionSavePreset,
			PresetName: "roundtrip",
		}, nil
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

	got, warnings, err := preset.Load("roundtrip")
	if err != nil {
		t.Fatalf("preset.Load() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("preset.Load() warnings = %v, want none", warnings)
	}

	if got.Target != "opencode" {
		t.Errorf("Target = %q, want %q", got.Target, "opencode")
	}
	if len(got.Skills) != 1 || got.Skills[0] != "diagnose" {
		t.Errorf("Skills = %v, want [diagnose]", got.Skills)
	}
	if got.Role != "senior reviewer" {
		t.Errorf("Role = %q, want %q", got.Role, "senior reviewer")
	}
	if got.Context != "reviewing a web app PR" {
		t.Errorf("Context = %q, want %q", got.Context, "reviewing a web app PR")
	}
	if got.Constraints != "no new dependencies" {
		t.Errorf("Constraints = %q, want %q", got.Constraints, "no new dependencies")
	}
	if got.OutputFormat != "markdown checklist" {
		t.Errorf("OutputFormat = %q, want %q", got.OutputFormat, "markdown checklist")
	}
	if len(got.Examples) != 1 || got.Examples[0] != "example one" {
		t.Errorf("Examples = %v, want [example one]", got.Examples)
	}
	if !strings.Contains(stderr.String(), "roundtrip") {
		t.Errorf("expected stderr confirmation to name the preset, got:\n%s", stderr.String())
	}
}

// TestGenerate_TUI_SavePresetAction_UsesResultInputsNotOpts is the test
// that proves the fromInputs mapping, not collectPresetFromOpts, is
// what backs ActionSavePreset. opts.role ("flag role") and
// result.Inputs.Role ("edited in picker") are deliberately different -
// if runInteractive ever regressed to reusing collectPresetFromOpts (or
// otherwise reading opts instead of result.Inputs), the saved preset
// would carry the flag's stale value instead of the picker's edit, and
// this test would catch exactly that.
func TestGenerate_TUI_SavePresetAction_UsesResultInputsNotOpts(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PROMPTSMITH_PRESETS_DIR", dir)

	defer stubInteractive(t, true)()
	defer stubRunTUI(t, func(reg *registry.Registry, in prompt.Inputs, _ tui.Options) (tui.Result, error) {
		// Simulate the picker editing role away from what --role seeded.
		edited := in
		edited.Role = "edited in picker"
		return tui.Result{Inputs: edited, Action: tui.ActionSavePreset, PresetName: "edited"}, nil
	})()

	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"-t", "generic", "--role", "flag role", "goal"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
	}

	got, _, err := preset.Load("edited")
	if err != nil {
		t.Fatalf("preset.Load() error = %v", err)
	}
	if got.Role != "edited in picker" {
		t.Errorf("Role = %q, want %q (result.Inputs must win over stale opts)", got.Role, "edited in picker")
	}
}

// TestGenerate_TUI_SavePresetAction_OmitsGoalAndUnsetFields asserts on
// the raw file bytes, mirroring
// TestGenerate_SavePresetOmitsUnsetFields: only role was set on
// result.Inputs, so every other preset key must be entirely absent
// from the written file, and "goal" must never appear - a preset
// describes how to ask, not what to ask.
func TestGenerate_TUI_SavePresetAction_OmitsGoalAndUnsetFields(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PROMPTSMITH_PRESETS_DIR", dir)

	defer stubInteractive(t, true)()
	defer stubRunTUI(t, func(reg *registry.Registry, in prompt.Inputs, _ tui.Options) (tui.Result, error) {
		return tui.Result{
			Inputs:     prompt.Inputs{Goal: "some goal text", Role: "reviewer"},
			Action:     tui.ActionSavePreset,
			PresetName: "onlyrole",
		}, nil
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

	data, err := os.ReadFile(filepath.Join(dir, "onlyrole.yaml"))
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}
	body := string(data)

	for _, key := range []string{"context:", "constraints:", "output_format:", "examples:", "skills:", "target:"} {
		if strings.Contains(body, key) {
			t.Errorf("file contains omitted key %q, body:\n%s", key, body)
		}
	}
	if strings.Contains(body, "goal") {
		t.Errorf("file contains a %q key, which must never be written, body:\n%s", "goal", body)
	}
}

// TestGenerate_TUI_SavePresetAction_WithoutOverwriteRefusesExistingFile
// mirrors TestGenerate_SavePresetWithoutForceRefusesExistingFile:
// OverwritePreset: false against an existing preset must fail and must
// leave the original file's contents untouched.
func TestGenerate_TUI_SavePresetAction_WithoutOverwriteRefusesExistingFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PROMPTSMITH_PRESETS_DIR", dir)
	writePreset(t, dir, "collide", "role: original\n")

	path := filepath.Join(dir, "collide.yaml")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}

	defer stubInteractive(t, true)()
	defer stubRunTUI(t, func(reg *registry.Registry, in prompt.Inputs, _ tui.Options) (tui.Result, error) {
		return tui.Result{
			Inputs:          prompt.Inputs{Role: "attempted overwrite"},
			Action:          tui.ActionSavePreset,
			PresetName:      "collide",
			OverwritePreset: false,
		}, nil
	})()

	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"-t", "generic", "goal"})

	err = root.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want an error for an existing preset without OverwritePreset")
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("error = %v, want it to mention --force (preset.Save's own message)", err)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}
	if string(after) != string(before) {
		t.Errorf("file contents changed after a refused save:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

// TestGenerate_TUI_SavePresetAction_WithOverwriteSucceeds is the
// successful mirror of the test above: OverwritePreset: true against
// the same collision succeeds, replaces the contents, and (per
// preset.Save's force path) leaves the file at 0o600.
func TestGenerate_TUI_SavePresetAction_WithOverwriteSucceeds(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PROMPTSMITH_PRESETS_DIR", dir)
	writePreset(t, dir, "collide", "role: original\n")

	defer stubInteractive(t, true)()
	defer stubRunTUI(t, func(reg *registry.Registry, in prompt.Inputs, _ tui.Options) (tui.Result, error) {
		return tui.Result{
			Inputs:          prompt.Inputs{Role: "overwritten"},
			Action:          tui.ActionSavePreset,
			PresetName:      "collide",
			OverwritePreset: true,
		}, nil
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

	got, _, err := preset.Load("collide")
	if err != nil {
		t.Fatalf("preset.Load() error = %v", err)
	}
	if got.Role != "overwritten" {
		t.Errorf("Role = %q, want %q (OverwritePreset should have overwritten)", got.Role, "overwritten")
	}

	// Meaningless on Windows, which has no Unix permission bits - see
	// the identical guard in TestGenerate_TUI_WriteAction and
	// TestGenerate_OutWritesFileAndSuppressesStdout.
	if runtime.GOOS != "windows" {
		info, err := os.Stat(filepath.Join(dir, "collide.yaml"))
		if err != nil {
			t.Fatalf("os.Stat() error = %v", err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("file perms = %o, want 0600", perm)
		}
	}
}

// TestGenerate_TUI_SavePresetAction_InvalidNameSurfacesValidationError
// confirms an invalid PresetName (one containing a path separator)
// surfaces preset.Save's own name-validation error rather than writing
// anything - runInteractive must not try to detect or pre-validate
// this itself, since the TUI already checks existence before
// submitting and preset.Save is the single source of truth for what
// makes a name valid.
func TestGenerate_TUI_SavePresetAction_InvalidNameSurfacesValidationError(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "presets")
	t.Setenv("PROMPTSMITH_PRESETS_DIR", dir)

	defer stubInteractive(t, true)()
	defer stubRunTUI(t, func(reg *registry.Registry, in prompt.Inputs, _ tui.Options) (tui.Result, error) {
		return tui.Result{
			Inputs:     prompt.Inputs{Role: "x"},
			Action:     tui.ActionSavePreset,
			PresetName: "../../etc/passwd",
		}, nil
	})()

	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"-t", "generic", "goal"})

	if err := root.Execute(); err == nil {
		t.Fatal("Execute() error = nil, want an error for a path-traversal preset name")
	}

	var found []string
	_ = filepath.Walk(parent, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			found = append(found, path)
		}
		return nil
	})
	if len(found) != 0 {
		t.Errorf("save wrote file(s) despite returning an error: %v", found)
	}
}
