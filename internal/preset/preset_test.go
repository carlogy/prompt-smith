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
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
)

// panicFS fails the test immediately if anything ever calls Open on it -
// used to prove that invalid preset names are rejected before the
// filesystem is touched at all.
type panicFS struct{ t *testing.T }

func (p panicFS) Open(name string) (fs.File, error) {
	p.t.Fatalf("panicFS.Open(%q) called: validateName should have rejected the name first", name)
	return nil, nil
}

func TestDir(t *testing.T) {
	t.Run("env override wins outright", func(t *testing.T) {
		t.Setenv("PROMPTSMITH_PRESETS_DIR", "/custom/presets/dir")
		t.Setenv("XDG_CONFIG_HOME", "/should/be/ignored")

		got, err := Dir()
		if err != nil {
			t.Fatalf("Dir() error = %v", err)
		}
		if got != "/custom/presets/dir" {
			t.Errorf("Dir() = %q, want %q", got, "/custom/presets/dir")
		}
	})

	t.Run("XDG_CONFIG_HOME is used when set", func(t *testing.T) {
		t.Setenv("PROMPTSMITH_PRESETS_DIR", "")
		t.Setenv("XDG_CONFIG_HOME", "/xdg/config")

		got, err := Dir()
		if err != nil {
			t.Fatalf("Dir() error = %v", err)
		}
		want := filepath.Join("/xdg/config", "promptsmith", "presets")
		if got != want {
			t.Errorf("Dir() = %q, want %q", got, want)
		}
	})

	t.Run("falls back to ~/.config", func(t *testing.T) {
		t.Setenv("PROMPTSMITH_PRESETS_DIR", "")
		t.Setenv("XDG_CONFIG_HOME", "")

		home, err := os.UserHomeDir()
		if err != nil {
			t.Fatalf("os.UserHomeDir() error = %v", err)
		}

		got, err := Dir()
		if err != nil {
			t.Fatalf("Dir() error = %v", err)
		}
		want := filepath.Join(home, ".config", "promptsmith", "presets")
		if got != want {
			t.Errorf("Dir() = %q, want %q", got, want)
		}
	})
}

func TestValidateName(t *testing.T) {
	cases := []struct {
		name    string
		wantErr bool
	}{
		{"web-review", false},
		{"../evil", true},
		{"a/b", true},
		{"..", true},
		{".", true},
		{"", true},
		{"   ", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateName(tc.name)
			if tc.wantErr && err == nil {
				t.Errorf("validateName(%q) error = nil, want an error", tc.name)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("validateName(%q) error = %v, want nil", tc.name, err)
			}
		})
	}
}

func TestLoadFS_RejectsBadNamesWithoutTouchingDisk(t *testing.T) {
	badNames := []string{"../evil", "a/b", "..", ".", "", "   "}
	for _, name := range badNames {
		t.Run(name, func(t *testing.T) {
			p, warnings, err := LoadFS(panicFS{t: t}, name)
			if err == nil {
				t.Fatalf("LoadFS(%q) error = nil, want an error", name)
			}
			if errors.Is(err, ErrNotFound) {
				t.Errorf("LoadFS(%q) error wraps ErrNotFound, want a plain validation error", name)
			}
			if p != nil {
				t.Errorf("LoadFS(%q) preset = %+v, want nil", name, p)
			}
			if warnings != nil {
				t.Errorf("LoadFS(%q) warnings = %v, want nil", name, warnings)
			}
		})
	}
}

func TestLoadFS_HappyPath(t *testing.T) {
	fsys := fstest.MapFS{
		"web-review.yaml": &fstest.MapFile{Data: []byte(`
target: opencode
skills:
  - diagnose
  - verify
role: senior reviewer
context: reviewing a web app PR
constraints: no new dependencies
output_format: markdown checklist
examples:
  - "example one"
  - "example two"
`)},
	}

	p, warnings, err := LoadFS(fsys, "web-review")
	if err != nil {
		t.Fatalf("LoadFS() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("warnings = %v, want none", warnings)
	}

	want := &Preset{
		Target:       "opencode",
		Skills:       []string{"diagnose", "verify"},
		Role:         "senior reviewer",
		Context:      "reviewing a web app PR",
		Constraints:  "no new dependencies",
		OutputFormat: "markdown checklist",
		Examples:     []string{"example one", "example two"},
	}
	if !reflect.DeepEqual(p, want) {
		t.Errorf("LoadFS() = %+v, want %+v", p, want)
	}
}

func TestLoadFS_NotFound(t *testing.T) {
	fsys := fstest.MapFS{}

	_, _, err := LoadFS(fsys, "missing")
	if err == nil {
		t.Fatal("LoadFS() error = nil, want an error")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("LoadFS() error = %v, want it to wrap ErrNotFound", err)
	}
}

func TestLoadFS_MalformedIsNotErrNotFound(t *testing.T) {
	fsys := fstest.MapFS{
		"broken.yaml": &fstest.MapFile{Data: []byte("role: [unterminated\n")},
	}

	_, _, err := LoadFS(fsys, "broken")
	if err == nil {
		t.Fatal("LoadFS() error = nil, want an error")
	}
	if errors.Is(err, ErrNotFound) {
		t.Errorf("LoadFS() error = %v, want it NOT to wrap ErrNotFound", err)
	}
}

func TestLoadFS_TypeErrorKeepsPartialValues(t *testing.T) {
	fsys := fstest.MapFS{
		"partial.yaml": &fstest.MapFile{Data: []byte(`
target: claude-code
role: reviewer
examples: "just one"
`)},
	}

	p, warnings, err := LoadFS(fsys, "partial")
	if err != nil {
		t.Fatalf("LoadFS() error = %v", err)
	}
	if len(warnings) == 0 {
		t.Fatal("warnings = none, want at least one for the examples type mismatch")
	}
	if p.Target != "claude-code" {
		t.Errorf("Target = %q, want %q (should survive the examples type error)", p.Target, "claude-code")
	}
	if p.Role != "reviewer" {
		t.Errorf("Role = %q, want %q (should survive the examples type error)", p.Role, "reviewer")
	}
	if len(p.Examples) != 0 {
		t.Errorf("Examples = %v, want empty (the field that failed to decode)", p.Examples)
	}
}

func TestLoadFS_SyntaxErrorIsFatal(t *testing.T) {
	fsys := fstest.MapFS{
		"syntax.yaml": &fstest.MapFile{Data: []byte("role: [unterminated\n")},
	}

	p, _, err := LoadFS(fsys, "syntax")
	if err == nil {
		t.Fatal("LoadFS() error = nil, want an error")
	}
	if p != nil {
		t.Errorf("LoadFS() preset = %+v, want nil", p)
	}
}

func TestLoadFS_UnknownKeyWarnsButStillLoads(t *testing.T) {
	fsys := fstest.MapFS{
		"typo.yaml": &fstest.MapFile{Data: []byte(`
target: opencode
rol: reviewer
`)},
	}

	p, warnings, err := LoadFS(fsys, "typo")
	if err != nil {
		t.Fatalf("LoadFS() error = %v", err)
	}
	if p.Target != "opencode" {
		t.Errorf("Target = %q, want %q", p.Target, "opencode")
	}
	found := false
	for _, w := range warnings {
		if strings.Contains(w, `unknown key "rol"`) {
			found = true
		}
	}
	if !found {
		t.Errorf("warnings = %v, want one naming the unknown key %q", warnings, "rol")
	}
}

func TestLoadFS_GoalKeyGetsDedicatedWarning(t *testing.T) {
	fsys := fstest.MapFS{
		"withgoal.yaml": &fstest.MapFile{Data: []byte(`
target: opencode
goal: fix the flaky test
`)},
	}

	p, warnings, err := LoadFS(fsys, "withgoal")
	if err != nil {
		t.Fatalf("LoadFS() error = %v", err)
	}
	if p.Target != "opencode" {
		t.Errorf("Target = %q, want %q", p.Target, "opencode")
	}

	found := false
	for _, w := range warnings {
		if strings.Contains(w, "how to ask") && strings.Contains(w, "not what to ask") {
			found = true
		}
		// Must not also produce the generic unknown-key wording for "goal".
		if strings.Contains(w, `unknown key "goal"`) {
			t.Errorf("goal produced the generic unknown-key warning, want the dedicated one: %v", warnings)
		}
	}
	if !found {
		t.Errorf("warnings = %v, want the dedicated goal warning", warnings)
	}
}

func TestList(t *testing.T) {
	fsys := fstest.MapFS{
		"web-review.yaml":    &fstest.MapFile{Data: []byte("target: opencode\n")},
		"api-design.yaml":    &fstest.MapFile{Data: []byte("target: opencode\n")},
		"legacy.yml":         &fstest.MapFile{Data: []byte("target: opencode\n")}, // wrong extension
		"notes.txt":          &fstest.MapFile{Data: []byte("just notes")},         // not a preset at all
		"subdir/nested.yaml": &fstest.MapFile{Data: []byte("target: opencode\n")}, // ignored: not at root
	}

	names, warnings, err := List(fsys)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	wantNames := []string{"api-design", "web-review"}
	if !reflect.DeepEqual(names, wantNames) {
		t.Errorf("names = %v, want %v", names, wantNames)
	}

	if len(warnings) != 2 {
		t.Fatalf("warnings = %v, want exactly 2 (legacy.yml and notes.txt)", warnings)
	}
	joined := strings.Join(warnings, "\n")
	if !strings.Contains(joined, "legacy.yml") {
		t.Errorf("warnings = %v, want one mentioning legacy.yml", warnings)
	}
	if !strings.Contains(joined, "notes.txt") {
		t.Errorf("warnings = %v, want one mentioning notes.txt", warnings)
	}
}

func TestList_Empty(t *testing.T) {
	names, warnings, err := List(fstest.MapFS{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(names) != 0 || len(warnings) != 0 {
		t.Errorf("List() = %v, %v, want both empty", names, warnings)
	}
}
