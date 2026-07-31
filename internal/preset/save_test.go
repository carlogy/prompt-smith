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

package preset

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// TestSave_RoundTripsWithLoadFS_ZeroWarnings is load-bearing: it's the
// single assertion that covers three requirements on Save at once. A
// stray "goal" key, ANY unknown key, or a wrong file extension would
// each independently surface as a warning (or an outright load
// failure) from LoadFS, so "Save then LoadFS produces the same Preset
// back with zero warnings" is a stronger check than asserting on any
// one of those individually - it fails if Save regresses on any of
// them.
func TestSave_RoundTripsWithLoadFS_ZeroWarnings(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PROMPTSMITH_PRESETS_DIR", dir)

	want := &Preset{
		Target:       "opencode",
		Skills:       []string{"diagnose", "verify"},
		Role:         "senior reviewer",
		Context:      "reviewing a web app PR",
		Constraints:  "no new dependencies",
		OutputFormat: "markdown checklist",
		Examples:     []string{"example one", "example two"},
	}

	if _, err := Save("web-review", want, false); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, warnings, err := LoadFS(os.DirFS(dir), "web-review")
	if err != nil {
		t.Fatalf("LoadFS() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("warnings = %v, want none", warnings)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("round-tripped Preset = %+v, want %+v", got, want)
	}
}

// TestSave_FullRoundTrip is the same round-trip property as the
// keystone test above, but with every field populated (including
// multi-element Skills and Examples) to make sure no field is dropped
// or reordered along the way.
func TestSave_FullRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PROMPTSMITH_PRESETS_DIR", dir)

	want := &Preset{
		Target:       "claude-code",
		Skills:       []string{"diagnose", "verify", "architect", "tdd"},
		Role:         "staff engineer",
		Context:      "a large monorepo migration",
		Constraints:  "no breaking changes, no new dependencies",
		OutputFormat: "numbered plan",
		Examples:     []string{"example one", "example two", "example three"},
	}

	if _, err := Save("full", want, false); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, warnings, err := Load("full")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("warnings = %v, want none", warnings)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Load() = %+v, want %+v", got, want)
	}
}

func TestSave_FileAndDirModes(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "presets")
	t.Setenv("PROMPTSMITH_PRESETS_DIR", dir)

	p := &Preset{Target: "opencode"}
	got, err := Save("modes", p, false)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	wantPath := filepath.Join(dir, "modes.yaml")
	if got != wantPath {
		t.Errorf("Save() returned path = %q, want %q (the file it actually wrote)", got, wantPath)
	}

	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("os.Stat(dir) error = %v", err)
	}
	// Windows has no Unix permission bits - any writable directory/file
	// reports 0777/0666 regardless of the mode passed to
	// MkdirAll/OpenFile - so this guarantee is only meaningful, and
	// only checked, on Unix. See the identical guard in
	// internal/cli/generate_test.go's TestGenerate_OutWritesFileAndSuppressesStdout.
	if runtime.GOOS != "windows" {
		if perm := dirInfo.Mode().Perm(); perm != 0o700 {
			t.Errorf("dir mode = %o, want %o", perm, 0o700)
		}
	}

	fileInfo, err := os.Stat(filepath.Join(dir, "modes.yaml"))
	if err != nil {
		t.Fatalf("os.Stat(file) error = %v", err)
	}
	if runtime.GOOS != "windows" {
		if perm := fileInfo.Mode().Perm(); perm != 0o600 {
			t.Errorf("file mode = %o, want %o", perm, 0o600)
		}
	}
}

func TestSave_CreatesDirWhenAbsent(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "does", "not", "exist", "yet")
	t.Setenv("PROMPTSMITH_PRESETS_DIR", dir)

	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("precondition failed: dir %s already exists", dir)
	}

	if _, err := Save("newdir", &Preset{Target: "opencode"}, false); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("os.Stat(dir) error = %v, want dir to have been created", err)
	}
	if !info.IsDir() {
		t.Errorf("%s is not a directory", dir)
	}
}

// TestSave_OmitsUnsetFields asserts directly on the raw file bytes -
// not just on a re-Load - so it also catches the failure mode where an
// omitted field round-trips fine (because it decodes back to the same
// zero value either way) but was actually written out as an explicit
// blank.
func TestSave_OmitsUnsetFields(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PROMPTSMITH_PRESETS_DIR", dir)

	p := &Preset{
		Target: "opencode",
		Role:   "reviewer",
		// Context, Constraints, OutputFormat, Skills, Examples all
		// deliberately left unset.
	}
	if _, err := Save("partial", p, false); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "partial.yaml"))
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}
	body := string(data)

	for _, key := range []string{"context", "constraints", "output_format", "skills", "examples"} {
		if strings.Contains(body, key) {
			t.Errorf("file contains omitted key %q, body:\n%s", key, body)
		}
	}
	if !strings.Contains(body, "target") || !strings.Contains(body, "role") {
		t.Errorf("file is missing a field that WAS set, body:\n%s", body)
	}
	if strings.Contains(body, "goal") {
		t.Errorf("file contains a %q key, which must never be written, body:\n%s", "goal", body)
	}
}

func TestSave_ExistingFileWithoutForce(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PROMPTSMITH_PRESETS_DIR", dir)

	original := &Preset{Target: "original"}
	if _, err := Save("collide", original, false); err != nil {
		t.Fatalf("Save() first call error = %v", err)
	}

	path := filepath.Join(dir, "collide.yaml")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}

	_, err = Save("collide", &Preset{Target: "replacement"}, false)
	if err == nil {
		t.Fatal("Save() error = nil, want an error for an existing file without force")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error = %v, want it to contain the full path %q", err, path)
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("error = %v, want it to name the --force flag", err)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}
	if string(after) != string(before) {
		t.Errorf("file contents changed after a rejected Save:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestSave_ForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PROMPTSMITH_PRESETS_DIR", dir)

	if _, err := Save("collide", &Preset{Target: "original"}, false); err != nil {
		t.Fatalf("Save() first call error = %v", err)
	}

	path, err := Save("collide", &Preset{Target: "replacement"}, true)
	if err != nil {
		t.Fatalf("Save() with force error = %v", err)
	}
	wantPath := filepath.Join(dir, "collide.yaml")
	if path != wantPath {
		t.Errorf("Save() with force returned path = %q, want %q", path, wantPath)
	}

	got, _, err := Load("collide")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Target != "replacement" {
		t.Errorf("Target = %q, want %q (force should have overwritten)", got.Target, "replacement")
	}
}

// TestSave_ForceFixesLooseModeOnPreexistingFile is the regression test
// for the os.WriteFile bug this package used to have: os.WriteFile
// only applies its mode argument when it CREATES a file, so a --force
// overwrite of a preset that already existed with looser permissions
// (0644 here, standing in for a hand-authored file or one written
// before this fix) used to leave it world-readable instead of
// restoring the 0o600 every other preset file gets. Pre-creating the
// file at 0644 and asserting 0600 afterward is the only way to catch
// that regression - TestSave_ForceOverwrites's own file was always
// 0600 already (Save created it), so it can't distinguish "Chmod'd"
// from "already had the right mode and nothing touched it".
func TestSave_ForceFixesLooseModeOnPreexistingFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PROMPTSMITH_PRESETS_DIR", dir)

	path := filepath.Join(dir, "loose.yaml")
	if err := os.WriteFile(path, []byte("target: original\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() precondition error = %v", err)
	}

	if _, err := Save("loose", &Preset{Target: "replacement"}, true); err != nil {
		t.Fatalf("Save() with force error = %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("os.Stat() error = %v", err)
	}
	// Meaningless on Windows - see the identical guard in
	// TestSave_FileAndDirModes above.
	if runtime.GOOS != "windows" {
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("file mode after --force over a pre-existing 0644 file = %o, want %o", perm, 0o600)
		}
	}
}

// TestSave_RejectsInvalidNames covers both the "obviously invalid" name
// shapes and path-traversal attempts. For the traversal cases it also
// walks up from the target directory to confirm nothing was written
// outside it - a name like "../../etc/passwd" that merely failed to
// produce a file INSIDE dir wouldn't be enough; it must not have
// written one anywhere else either.
func TestSave_RejectsInvalidNames(t *testing.T) {
	badNames := []string{"", ".", "..", "../../etc/passwd", "a/b"}

	for _, name := range badNames {
		t.Run(name, func(t *testing.T) {
			parent := t.TempDir()
			dir := filepath.Join(parent, "presets")
			t.Setenv("PROMPTSMITH_PRESETS_DIR", dir)

			_, err := Save(name, &Preset{Target: "opencode"}, false)
			if err == nil {
				t.Fatalf("Save(%q) error = nil, want an error", name)
			}

			// Nothing should have been written anywhere under parent,
			// including outside dir itself (dir may not even exist,
			// since validateName must run before the directory is
			// created).
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
				t.Errorf("Save(%q) wrote file(s) despite returning an error: %v", name, found)
			}
		})
	}
}
