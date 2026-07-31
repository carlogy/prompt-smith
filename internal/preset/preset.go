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

// Package preset loads saved bundles of prompt-generation defaults - a
// preset describes HOW to ask (target, skills, role, context,
// constraints, output format, examples), never WHAT to ask. That's why
// Preset has no Goal field: the goal is supplied fresh on every
// invocation, while a preset is the reusable part around it.
package preset

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Preset holds the prompt defaults a preset file supplies. There is
// deliberately no Goal field: a preset is HOW to ask, not WHAT to ask.
type Preset struct {
	Target       string
	Skills       []string
	Role         string
	Context      string
	Constraints  string
	OutputFormat string
	Examples     []string
}

// ErrNotFound is returned (wrapped) when the named preset file does not
// exist, so callers can distinguish "no such preset" from "preset is
// broken" and print a single clean message for the former.
var ErrNotFound = errors.New("preset not found")

// presetDoc is the on-disk shape of a preset file. "goal" is
// intentionally absent - see the Preset doc comment - and LoadFS gives
// it a dedicated warning rather than treating it as an ordinary unknown
// key, since it's the one omission a user is likely to hit by mistake.
//
// Every tag also carries "omitempty". That's read by Save (save.go),
// not by LoadFS: yaml.v3's omitempty affects marshaling only and is
// inert on decode, so LoadFS's behavior here is completely unchanged.
// Save reuses this exact struct rather than defining a second
// write-only one, because omitempty then gives it "leave unset fields
// out of the file" as a property of the struct's tags instead of seven
// hand-written "if this field is empty, skip it" checks at the call
// site.
type presetDoc struct {
	Target       string   `yaml:"target,omitempty"`
	Skills       []string `yaml:"skills,omitempty"`
	Role         string   `yaml:"role,omitempty"`
	Context      string   `yaml:"context,omitempty"`
	Constraints  string   `yaml:"constraints,omitempty"`
	OutputFormat string   `yaml:"output_format,omitempty"`
	Examples     []string `yaml:"examples,omitempty"`
}

// knownKeys is presetDoc's yaml tag set, used by the pass-2 unknown-key
// scan in LoadFS.
var knownKeys = map[string]bool{
	"target":        true,
	"skills":        true,
	"role":          true,
	"context":       true,
	"constraints":   true,
	"output_format": true,
	"examples":      true,
}

// Dir returns the directory presets are read from.
// $PROMPTSMITH_PRESETS_DIR, if non-empty, wins outright. Otherwise it's
// $XDG_CONFIG_HOME/promptsmith/presets, falling back to
// ~/.config/promptsmith/presets per the XDG Base Directory spec. It's
// not an error for the directory not to exist - ListDir treats that as
// "no presets" rather than a failure.
func Dir() (string, error) {
	if dir := os.Getenv("PROMPTSMITH_PRESETS_DIR"); dir != "" {
		return dir, nil
	}

	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		configHome = filepath.Join(home, ".config")
	}
	return filepath.Join(configHome, "promptsmith", "presets"), nil
}

// validateName rejects any preset name that isn't a bare, single-segment
// file stem, before anything touches the filesystem. This is the
// path-traversal guard: without it, a name like "../../../etc/passwd"
// would resolve outside the presets directory once ".yaml" is appended
// and the result is joined against fsys. Because both separators are
// rejected outright, a name can never contain more than one path
// element, so checking the whole name against "." and ".." also covers
// "a .. path element" - there's nothing left for a separator to split.
func validateName(name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("preset name must not be empty")
	}
	if strings.ContainsRune(name, '/') || strings.ContainsRune(name, os.PathSeparator) {
		return fmt.Errorf("preset name %q must not contain a path separator", name)
	}
	if name == "." || name == ".." {
		return fmt.Errorf("preset name %q is not a valid preset name", name)
	}
	return nil
}

// LoadFS reads and decodes the preset named name (i.e. "<name>.yaml")
// from fsys. Load is the production entry point; LoadFS exists so
// loading/parsing behavior is testable against synthetic filesystems.
//
// Decoding happens in two passes over the same bytes:
//
//  1. Values, leniently. gopkg.in/yaml.v3's Unmarshal guarantees that
//     when one or more fields have the wrong YAML type, decoding
//     "continues partially until the end of the YAML content" and
//     returns a *yaml.TypeError describing every mismatch, while the
//     destination struct is "still unmarshaled partially" (see
//     yaml.go's Unmarshal doc and the TypeError doc). We rely on that
//     explicitly: one wrong-typed field (say, `examples: "just one"`
//     instead of a sequence) produces a warning for that field but must
//     not discard every other field that decoded fine. A plain
//     (non-TypeError) decode error means nothing usable came back -
//     almost certainly a YAML syntax error - and that IS fatal for this
//     preset.
//
//  2. Unknown keys, via a second decode into map[string]any. We
//     deliberately do NOT use yaml.Decoder.KnownFields(true) for this:
//     that option fails the *entire* decode on the first unrecognized
//     key, which would throw away an otherwise-usable preset over a
//     single typo. Warning-and-continuing is the whole point of this
//     package's error handling, so unknown keys get the same treatment
//     as type mismatches. A "goal" key gets its own dedicated warning
//     instead of the generic one, since "goal" is the one field a user
//     is likely to expect and its absence is deliberate (see the Preset
//     doc comment).
func LoadFS(fsys fs.FS, name string) (*Preset, []string, error) {
	if err := validateName(name); err != nil {
		return nil, nil, err
	}

	filename := name + ".yaml"
	data, err := fs.ReadFile(fsys, filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, fmt.Errorf("preset %q: %w", name, ErrNotFound)
		}
		return nil, nil, fmt.Errorf("preset %q: %w", name, err)
	}

	var warnings []string

	var doc presetDoc
	err = yaml.Unmarshal(data, &doc)
	var terr *yaml.TypeError
	if errors.As(err, &terr) {
		for _, e := range terr.Errors {
			warnings = append(warnings, fmt.Sprintf("preset %q: %s", name, e))
		}
	} else if err != nil {
		return nil, warnings, fmt.Errorf("preset %q: parse: %w", name, err)
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		// The pass-1 decode above already succeeded (or returned a
		// TypeError, which still yields a usable node tree), so a
		// map[string]any decode of the same bytes failing here would
		// be unexpected. Treat it the same as any other parse warning
		// rather than failing a preset pass 1 already accepted.
		warnings = append(warnings, fmt.Sprintf("preset %q: scan for unknown keys: %v", name, err))
	}
	for key := range raw {
		if knownKeys[key] {
			continue
		}
		if key == "goal" {
			warnings = append(warnings, fmt.Sprintf(
				`preset %q: "goal" is not a preset field - a preset describes how to ask, not what to ask`, name))
			continue
		}
		warnings = append(warnings, fmt.Sprintf("preset %q: unknown key %q ignored", name, key))
	}

	return &Preset{
		Target:       doc.Target,
		Skills:       doc.Skills,
		Role:         doc.Role,
		Context:      doc.Context,
		Constraints:  doc.Constraints,
		OutputFormat: doc.OutputFormat,
		Examples:     doc.Examples,
	}, warnings, nil
}

// Load resolves the presets directory (Dir) and loads the named preset
// from it. A missing presets directory surfaces as the same
// ErrNotFound-wrapping error as a missing file, so a caller can print a
// single clean "no such preset" message regardless of which is missing.
func Load(name string) (*Preset, []string, error) {
	dir, err := Dir()
	if err != nil {
		return nil, nil, fmt.Errorf("preset %q: %w", name, err)
	}

	if info, statErr := os.Stat(dir); statErr != nil {
		if os.IsNotExist(statErr) {
			return nil, nil, fmt.Errorf("preset %q: %w", name, ErrNotFound)
		}
		return nil, nil, fmt.Errorf("preset %q: %v", name, statErr)
	} else if !info.IsDir() {
		return nil, nil, fmt.Errorf("preset %q: presets path %s is not a directory", name, dir)
	}

	return LoadFS(os.DirFS(dir), name)
}

// List reads the root of fsys and returns the sorted stems of every
// "*.yaml" file found there (e.g. "web-review.yaml" -> "web-review").
// Subdirectories are skipped silently. Any other regular file - one
// that does NOT end in ".yaml" - produces a warning that it's being
// ignored: unlike registry.loadUserSkills (which silently skips stray
// files, since a skill is a whole directory and one extra file at the
// root is meaningless), a preset file IS the unit of meaning here, so a
// stray "foo.yml" sitting in the presets directory is almost certainly a
// typo'd extension the user expected to work. Silently doing nothing is
// exactly the failure this feature exists to prevent.
func List(fsys fs.FS) ([]string, []string, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, nil, fmt.Errorf("list presets: %w", err)
	}

	var names, warnings []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		fname := entry.Name()
		if ext := filepath.Ext(fname); ext == ".yaml" {
			names = append(names, strings.TrimSuffix(fname, ext))
			continue
		}
		warnings = append(warnings, fmt.Sprintf(
			"%s: ignored - presets must use the .yaml extension", fname))
	}

	sort.Strings(names)
	return names, warnings, nil
}

// ListDir resolves the presets directory (Dir) and lists the presets in
// it. Mirroring registry.Load's handling of its user-skills directory: a
// missing presets directory is NOT an error here (the common case - no
// presets have been created yet), returning no names and no warnings. A
// path that exists but isn't a directory produces a warning instead.
func ListDir() ([]string, []string, error) {
	dir, err := Dir()
	if err != nil {
		return nil, []string{fmt.Sprintf("resolve presets directory: %v", err)}, nil
	}

	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return nil, nil, nil // most common case: no presets directory at all
	}
	if err != nil {
		return nil, []string{fmt.Sprintf("presets directory %s: %v", dir, err)}, nil
	}
	if !info.IsDir() {
		return nil, []string{fmt.Sprintf("presets path %s is not a directory", dir)}, nil
	}

	return List(os.DirFS(dir))
}
