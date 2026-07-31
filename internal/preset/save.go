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
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Save writes p to the presets directory (Dir) under name+".yaml" and
// returns the full path written. Save is the production entry point;
// saveDir exists underneath it so the file-writing behavior is
// testable against an arbitrary directory rather than only whatever
// Dir() resolves to - the same split LoadFS gives Load and List gives
// ListDir, just on the write side. Unlike those read paths, Save can't
// take an fs.FS: the standard library's fs.FS is read-only by design,
// so a directory string is the smallest abstraction that still lets
// tests avoid depending on Dir()'s environment-variable resolution
// directly.
//
// The returned path exists specifically so a caller confirming the
// save (e.g. the CLI's --save-preset) can report the full location
// without recomputing the "dir/name+.yaml" join itself - that join,
// and the ".yaml" extension in particular, is this package's
// invariant to own (see the path construction below), not something a
// second call site should have to restate correctly.
//
// force controls what happens when name already has a preset on disk:
// false refuses and returns an error naming the full path, true
// overwrites. The default is "refuse" because hand-authoring a preset
// file is currently the only way one comes to exist at all - there's no
// "list of presets that used to exist" to recover from, so silently
// clobbering one destroys the only copy.
func Save(name string, p *Preset, force bool) (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", fmt.Errorf("preset %q: %w", name, err)
	}
	return saveDir(dir, name, p, force)
}

// saveDir does the actual work of Save against an explicit directory.
// See Save's doc comment for why this split exists.
func saveDir(dir, name string, p *Preset, force bool) (string, error) {
	if err := validateName(name); err != nil {
		return "", err
	}

	// presetDoc has no Goal field - see the Preset doc comment - so
	// there is structurally no way for this assignment to produce a
	// "goal" key in the marshaled output; it isn't a check that could
	// be forgotten. That matters here specifically because LoadFS
	// gives a "goal" key its own dedicated warning (see
	// TestLoadFS_GoalKeyGetsDedicatedWarning), so a saver that emitted
	// one would make every preset it wrote warn on the very next load.
	doc := presetDoc{
		Target:       p.Target,
		Skills:       p.Skills,
		Role:         p.Role,
		Context:      p.Context,
		Constraints:  p.Constraints,
		OutputFormat: p.OutputFormat,
		Examples:     p.Examples,
	}

	// presetDoc's yaml tags all carry "omitempty" (see preset.go), so a
	// zero-value field here - "" or a nil slice - is left out of data
	// entirely rather than marshaled as an empty string. That
	// distinction matters because LoadFS/Load have no way to tell "the
	// YAML omitted this key" apart from "the key was present but
	// empty" - both just decode to the zero value - so writing blanks
	// for unset fields would make a preset silently blank out fields
	// on load that the caller never intended to touch.
	data, err := yaml.Marshal(&doc)
	if err != nil {
		return "", fmt.Errorf("preset %q: encode: %w", name, err)
	}

	// The directory is created if absent, but never chmod'd if it
	// already exists - Dir() only resolves a path, it never guarantees
	// the path exists, and MkdirAll is a no-op when it does.
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("preset %q: create presets directory: %w", name, err)
	}

	// Always ".yaml", never ".yml": List warns about and ignores any
	// "*.yml" file it finds in the presets directory (see List's doc
	// comment), so a saver that emitted one would write a preset the
	// loader refuses to read back.
	path := filepath.Join(dir, name+".yaml")

	if force {
		// os.WriteFile alone is not enough here: it only applies mode
		// 0o600 when it CREATES the file - if path already exists (the
		// whole reason force is needed at all), WriteFile opens it
		// with O_TRUNC and leaves its existing mode untouched. A
		// pre-existing preset saved before this fix, or one someone
		// hand-authored with looser permissions, would stay
		// world-readable after a --force overwrite, defeating the
		// entire reason preset files are 0o600 in the first place: a
		// preset's context/constraints text can carry proprietary or
		// otherwise sensitive detail. Chmod'ing explicitly after the
		// write guarantees 0o600 regardless of what the file's mode
		// was going in.
		// #nosec G304 -- path is dir (from Dir(), never user text) joined
		// with name+".yaml", and validateName above has already rejected
		// any name containing a path separator or equal to "."/"..", so
		// path can never resolve outside dir. G304 flags any non-literal
		// OpenFile path unconditionally; this is the intended resolution
		// for a save path that legitimately needs a caller-chosen name.
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
		if err != nil {
			return "", fmt.Errorf("preset %q: write %s: %w", name, path, err)
		}
		defer f.Close()
		if _, err := f.Write(data); err != nil {
			return "", fmt.Errorf("preset %q: write %s: %w", name, path, err)
		}
		if err := f.Chmod(0o600); err != nil {
			return "", fmt.Errorf("preset %q: chmod %s: %w", name, path, err)
		}
		return path, nil
	}

	// O_CREATE|O_EXCL over stat-then-write: stat-then-write leaves a
	// TOCTOU window between the check and the write where a
	// concurrently created file could be silently overwritten anyway,
	// defeating the whole point of the force flag. O_EXCL makes
	// "create only if absent" a single atomic syscall instead, and
	// avoids the race gosec flags a bare os.Stat+os.Create pair for.
	// #nosec G304 -- see the identical justification on the force branch's
	// OpenFile above: path is validated-name-derived, never arbitrary
	// user text, and cannot resolve outside dir.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if os.IsExist(err) {
			return "", fmt.Errorf("preset %q: %s already exists, use --force to overwrite", name, path)
		}
		return "", fmt.Errorf("preset %q: create %s: %w", name, path, err)
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return "", fmt.Errorf("preset %q: write %s: %w", name, path, err)
	}
	return path, nil
}
