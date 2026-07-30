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
)

// writePreset writes a preset YAML file named name+".yaml" into dir,
// creating dir if needed.
func writePreset(t *testing.T, dir, name, contents string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", dir, err)
	}
	path := filepath.Join(dir, name+".yaml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}

func TestPresets_ListsNamesOnStdoutOneEachLineAndDirOnStderr(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PROMPTSMITH_PRESETS_DIR", dir)
	writePreset(t, dir, "standup", "role: a scrum facilitator\n")
	writePreset(t, dir, "web-review", "role: a reviewer\n")

	cmd := newPresetsCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(nil)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
	}

	wantLines := []string{"standup", "web-review"}
	gotLines := strings.Split(strings.TrimRight(stdout.String(), "\n"), "\n")
	if len(gotLines) != len(wantLines) {
		t.Fatalf("stdout lines = %v, want %v", gotLines, wantLines)
	}
	for i, want := range wantLines {
		if gotLines[i] != want {
			t.Errorf("stdout line %d = %q, want %q", i, gotLines[i], want)
		}
	}

	if strings.Contains(stdout.String(), dir) {
		t.Errorf("expected the presets directory path to stay off stdout, got:\n%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), dir) {
		t.Errorf("expected the presets directory path on stderr, got:\n%s", stderr.String())
	}
}

func TestPresets_NoPresets_EmptyStdoutGuidanceOnStderrNoError(t *testing.T) {
	t.Setenv("PROMPTSMITH_PRESETS_DIR", t.TempDir())

	cmd := newPresetsCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(nil)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, want nil for the empty-presets case", err)
	}
	if stdout.String() != "" {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "PROMPTSMITH_PRESETS_DIR") {
		t.Errorf("stderr = %q, want mention of PROMPTSMITH_PRESETS_DIR", stderr.String())
	}
	if !strings.Contains(stderr.String(), ".yaml") {
		t.Errorf("stderr = %q, want mention of the <name>.yaml layout", stderr.String())
	}
}

func TestPresets_WarningsReachStderrWithPrefix(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PROMPTSMITH_PRESETS_DIR", dir)
	writePreset(t, dir, "good", "role: a reviewer\n")
	// Wrong extension: preset.List warns about this rather than
	// silently ignoring it (see preset.List's doc comment).
	if err := os.WriteFile(filepath.Join(dir, "legacy.yml"), []byte("role: x\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cmd := newPresetsCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(nil)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr.String())
	}
	want := "promptsmith: legacy.yml: ignored - presets must use the .yaml extension"
	if !strings.Contains(stderr.String(), want) {
		t.Errorf("stderr missing %q, got:\n%s", want, stderr.String())
	}
}

func TestUnknownPresetError_ListsAvailableNames(t *testing.T) {
	t.Setenv("PROMPTSMITH_PRESETS_DIR", t.TempDir())

	err := unknownPresetError("web-reveiw", []string{"standup", "web-review"})
	if err == nil {
		t.Fatal("unknownPresetError() = nil, want an error")
	}
	msg := err.Error()
	if !strings.Contains(msg, `"web-reveiw"`) {
		t.Errorf("error = %q, want the bad name quoted", msg)
	}
	if !strings.Contains(msg, "standup") || !strings.Contains(msg, "web-review") {
		t.Errorf("error = %q, want both available names listed", msg)
	}
}

func TestUnknownPresetError_EmptyDirCarriesGuidance(t *testing.T) {
	err := unknownPresetError("mypreset", nil)
	if err == nil {
		t.Fatal("unknownPresetError() = nil, want an error")
	}
	msg := err.Error()
	if !strings.Contains(msg, `"mypreset"`) {
		t.Errorf("error = %q, want the bad name quoted", msg)
	}
	if !strings.Contains(msg, "PROMPTSMITH_PRESETS_DIR") {
		t.Errorf("error = %q, want the create-a-preset guidance", msg)
	}
}
