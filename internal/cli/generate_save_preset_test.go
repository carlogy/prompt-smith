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
	"strings"
	"testing"

	"github.com/carlogy/prompt-smith/internal/preset"
	"github.com/carlogy/prompt-smith/internal/prompt"
	"github.com/carlogy/prompt-smith/internal/registry"
	"github.com/carlogy/prompt-smith/internal/tui"
)

// TestGenerate_SavePresetWithGoalIsAdditive pins the core additive
// guarantee: --save-preset alongside a goal does BOTH things, not
// either/or - the preset lands on disk AND the generated prompt still
// reaches stdout exactly like it would without --save-preset at all.
func TestGenerate_SavePresetWithGoalIsAdditive(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PROMPTSMITH_PRESETS_DIR", dir)

	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"-t", "generic", "-s", "diagnose", "--save-preset", "mine", "fix the flaky checkout test"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
	}

	if _, err := os.Stat(filepath.Join(dir, "mine.yaml")); err != nil {
		t.Errorf("preset file was not written: %v", err)
	}
	if !strings.Contains(stdout.String(), "fix the flaky checkout test") {
		t.Errorf("expected the generated prompt on stdout, got:\n%s", stdout.String())
	}
}

// TestGenerate_SavePresetWithNoGoalSavesAndExitsCleanly is THE pinned
// guarantee behind this whole feature: --save-preset with no goal at
// all must save the preset and return exit 0 WITHOUT falling into
// decideUseTUI's goalEmpty branch. Asserting runTUIFunc was never
// called is the only way to actually observe that: if the early
// return in runGenerate were missing (or too narrow), this exact
// invocation would instead open the interactive skill picker - an
// absurd side effect of asking to save a preset. See
// TestGenerate_PresetWithNoGoalLaunchesTUI (interactive_test.go) for
// the mirror-image regression canary: the SAME "no goal, no --tui"
// shape without --save-preset must still open the picker, so the
// early return this test exercises has to be scoped to --save-preset
// specifically, not to "no goal" in general.
func TestGenerate_SavePresetWithNoGoalSavesAndExitsCleanly(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PROMPTSMITH_PRESETS_DIR", dir)

	defer stubInteractive(t, true)()
	called := false
	defer stubRunTUI(t, func(reg *registry.Registry, in prompt.Inputs, _ []string) (tui.Result, error) {
		called = true
		return tui.Result{Inputs: in, Action: tui.ActionCancel}, nil
	})()

	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"--save-preset", "mine"}) // no goal at all

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "mine.yaml")); err != nil {
		t.Errorf("preset file was not written: %v", err)
	}
	if called {
		t.Error("expected runTUIFunc NOT to be called for --save-preset with no goal")
	}
}

// TestGenerate_SavePresetWithNoGoalAndExplicitTUIStillOpensPicker
// checks the other half of the same guard: an EXPLICIT --tui is the
// user directly asking for the picker, so it must still open even
// though no goal was given and --save-preset was also passed. Only the
// IMPLICIT goalEmpty reason for opening the picker is meant to be
// suppressed by --save-preset, not an explicit request.
func TestGenerate_SavePresetWithNoGoalAndExplicitTUIStillOpensPicker(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PROMPTSMITH_PRESETS_DIR", dir)

	defer stubInteractive(t, true)()
	called := false
	defer stubRunTUI(t, func(reg *registry.Registry, in prompt.Inputs, _ []string) (tui.Result, error) {
		called = true
		return tui.Result{Inputs: in, Action: tui.ActionCancel}, nil
	})()

	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"--save-preset", "mine", "--tui"}) // no goal, explicit --tui

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "mine.yaml")); err != nil {
		t.Errorf("preset file was not written: %v", err)
	}
	if !called {
		t.Error("expected an explicit --tui to still open the picker, even with --save-preset and no goal")
	}
}

// TestGenerate_SavePresetRoundTripsThroughRealLoaderWithZeroWarnings
// exercises every flag presetFieldSpecs maps, then loads the saved
// file back through the real preset.Load (not a hand-rolled
// assertion), asserting both that every field came back correct AND
// that there were zero warnings. Zero warnings is the load-bearing
// half of this assertion - a stray "goal" key, a wrong file
// extension, or any unrecognized key would each independently surface
// as a warning from LoadFS (see save_test.go's own
// TestSave_RoundTripsWithLoadFS_ZeroWarnings for the same reasoning
// one layer down), so this is a stronger check than comparing fields
// alone.
func TestGenerate_SavePresetRoundTripsThroughRealLoaderWithZeroWarnings(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PROMPTSMITH_PRESETS_DIR", dir)

	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{
		"--save-preset", "roundtrip",
		"-r", "senior reviewer",
		"-c", "no new dependencies",
		"-x", "reviewing a web app PR",
		"-f", "markdown checklist",
		"-e", "example one",
		"-s", "diagnose",
		"-t", "opencode",
		"fix the flaky checkout test",
	})

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
}

// TestGenerate_SavePresetOmitsUnsetFields asserts on the raw file
// bytes, not just a re-Load, mirroring save_test.go's own
// TestSave_OmitsUnsetFields one layer down: a field that round-trips
// fine (because it decodes to the same zero value either way) could
// still have been written out as an explicit blank, and only reading
// the bytes directly catches that. Saved with no goal, so the early
// return applies and generation never runs.
func TestGenerate_SavePresetOmitsUnsetFields(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PROMPTSMITH_PRESETS_DIR", dir)

	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"--save-preset", "onlyrole", "-r", "reviewer"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
	}

	data, err := os.ReadFile(filepath.Join(dir, "onlyrole.yaml"))
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}
	body := string(data)

	for _, key := range []string{"context:", "constraints:", "output_format:", "examples:", "skills:"} {
		if strings.Contains(body, key) {
			t.Errorf("file contains omitted key %q, body:\n%s", key, body)
		}
	}
	// target: is deliberately NOT in this omission list - --target
	// defaults to "generic" and is never empty, so it's always written
	// (see presetFieldSpecs's collect-direction doc comment).
	if strings.Contains(body, "goal") {
		t.Errorf("file contains a %q key, which must never be written, body:\n%s", "goal", body)
	}
}

// TestGenerate_SavePresetWithoutForceRefusesExistingFile mirrors
// save_test.go's TestSave_ExistingFileWithoutForce one layer up: the
// CLI surface's error, not just the package's, must name the full
// path and mention --force, and the first file's contents must be
// left untouched by the rejected second save.
func TestGenerate_SavePresetWithoutForceRefusesExistingFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PROMPTSMITH_PRESETS_DIR", dir)

	run := func(extraArgs ...string) error {
		reg := testRegistry(t)
		root := newRootCmd(reg)
		var stdout, stderr bytes.Buffer
		root.SetOut(&stdout)
		root.SetErr(&stderr)
		args := append([]string{"--save-preset", "collide", "-r", "first"}, extraArgs...)
		root.SetArgs(args)
		return root.Execute()
	}

	if err := run(); err != nil {
		t.Fatalf("first save error = %v", err)
	}

	path := filepath.Join(dir, "collide.yaml")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}

	err = run("-r", "second")
	if err == nil {
		t.Fatal("second save error = nil, want an error for an existing preset without --force")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error = %v, want it to contain the full path %q", err, path)
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("error = %v, want it to mention --force", err)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}
	if string(after) != string(before) {
		t.Errorf("file contents changed after a rejected save:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

// TestGenerate_SaveWithForceOverwrites is the successful mirror of the
// test above: the same collision, but with --force, succeeds and
// replaces the contents.
func TestGenerate_SaveWithForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PROMPTSMITH_PRESETS_DIR", dir)

	run := func(extraArgs ...string) error {
		reg := testRegistry(t)
		root := newRootCmd(reg)
		var stdout, stderr bytes.Buffer
		root.SetOut(&stdout)
		root.SetErr(&stderr)
		args := append([]string{"--save-preset", "collide"}, extraArgs...)
		root.SetArgs(args)
		return root.Execute()
	}

	if err := run("-r", "first"); err != nil {
		t.Fatalf("first save error = %v", err)
	}
	if err := run("-r", "second", "--force"); err != nil {
		t.Fatalf("forced save error = %v", err)
	}

	got, _, err := preset.Load("collide")
	if err != nil {
		t.Fatalf("preset.Load() error = %v", err)
	}
	if got.Role != "second" {
		t.Errorf("Role = %q, want %q (force should have overwritten)", got.Role, "second")
	}
}

// TestGenerate_ForceWithoutSavePresetErrors covers section B: --force
// alone, without --save-preset, is a user error - overwriting a
// preset that was never named doesn't mean anything.
func TestGenerate_ForceWithoutSavePresetErrors(t *testing.T) {
	t.Setenv("PROMPTSMITH_PRESETS_DIR", t.TempDir())

	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"--force", "goal"})

	err := root.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want an error for --force without --save-preset")
	}
	if !strings.Contains(err.Error(), "--force") || !strings.Contains(err.Error(), "--save-preset") {
		t.Errorf("error = %q, want it to name both flags", err.Error())
	}
}

// TestGenerate_EmptySavePresetNameErrors confirms --save-preset ""
// still reaches preset.Save's own name-validation error instead of
// being silently treated as "no save requested" - the same
// Changed()-not-empty-check reasoning applyPreset already relies on
// for -p/--preset.
func TestGenerate_EmptySavePresetNameErrors(t *testing.T) {
	t.Setenv("PROMPTSMITH_PRESETS_DIR", t.TempDir())

	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"--save-preset", "", "goal"})

	if err := root.Execute(); err == nil {
		t.Fatal("Execute() error = nil, want an error for --save-preset \"\"")
	}
}

// TestGenerate_SavePresetPathTraversalNameErrors confirms the CLI
// surface inherits preset.Save's path-traversal guard end-to-end, and
// that nothing gets written anywhere under the presets directory's
// parent - not just that no file landed INSIDE the presets dir.
func TestGenerate_SavePresetPathTraversalNameErrors(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "presets")
	t.Setenv("PROMPTSMITH_PRESETS_DIR", dir)

	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"--save-preset", "../../etc/passwd", "goal"})

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

// TestGenerate_SavePresetInheritsLoadedPresetValues is the pinned
// inheritance case: `-p base --save-preset derived` must copy base's
// already-merged values into derived, INCLUDING target - a
// Changed()-gated collect direction would only capture flags typed on
// this exact command line and would silently drop base's contribution.
func TestGenerate_SavePresetInheritsLoadedPresetValues(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PROMPTSMITH_PRESETS_DIR", dir)
	writePreset(t, dir, "base", "target: opencode\nskills:\n  - diagnose\nrole: base role\n")

	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"-p", "base", "--save-preset", "derived"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
	}

	got, warnings, err := preset.Load("derived")
	if err != nil {
		t.Fatalf("preset.Load() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("preset.Load() warnings = %v, want none", warnings)
	}
	if got.Target != "opencode" {
		t.Errorf("Target = %q, want %q (inherited from base)", got.Target, "opencode")
	}
	if len(got.Skills) != 1 || got.Skills[0] != "diagnose" {
		t.Errorf("Skills = %v, want [diagnose] (inherited from base)", got.Skills)
	}
	if got.Role != "base role" {
		t.Errorf("Role = %q, want %q (inherited from base)", got.Role, "base role")
	}
}

// TestGenerate_SavePresetSkippedOnGoalConflict pins C2's ordering
// requirement: a malformed invocation (--goal plus a positional goal)
// must error out via resolveGoal's errGoalConflict BEFORE the save
// runs. Never write a file for a command that's about to fail.
func TestGenerate_SavePresetSkippedOnGoalConflict(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PROMPTSMITH_PRESETS_DIR", dir)

	reg := testRegistry(t)
	root := newRootCmd(reg)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"-g", "x", "--save-preset", "shouldnotexist", "positional"})

	if err := root.Execute(); err == nil {
		t.Fatal("Execute() error = nil, want errGoalConflict")
	}
	if _, err := os.Stat(filepath.Join(dir, "shouldnotexist.yaml")); !os.IsNotExist(err) {
		t.Errorf("expected no preset file to be written on a goal conflict, os.Stat() error = %v", err)
	}
}

// TestPresetFieldSpecs_EveryEntryHasBothFuncs is a mechanical guard in
// the same class as generate_preset_test.go's mistyped-flagName tests:
// a nil apply or collect func in any entry would panic at runtime the
// first time that entry's direction is exercised (applyPreset for
// apply, collectPresetFromOpts for collect), rather than failing
// loudly at compile time - table literals don't enforce "every field
// must be set".
func TestPresetFieldSpecs_EveryEntryHasBothFuncs(t *testing.T) {
	for _, spec := range presetFieldSpecs {
		if spec.apply == nil {
			t.Errorf("presetFieldSpecs[%q].apply is nil", spec.flagName)
		}
		if spec.collect == nil {
			t.Errorf("presetFieldSpecs[%q].collect is nil", spec.flagName)
		}
	}
}
