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
	"io"
	"os"
	"testing"

	"github.com/carlogy/prompt-smith/internal/registry"
)

// testRegistry loads the real embedded registry for CLI-level tests. CLI
// tests exercise flag plumbing and output routing, not registry content,
// so using the real shipped data (already guarded by its own package's
// tests) avoids a third duplicate fixture.
//
// PROMPTSMITH_SKILLS_DIR is pinned to an empty temp directory so these
// tests stay hermetic regardless of the developer machine's real user
// skills directory (see registry.Load).
func testRegistry(t *testing.T) *registry.Registry {
	t.Helper()
	t.Setenv("PROMPTSMITH_SKILLS_DIR", t.TempDir())

	reg, warnings, err := registry.Load()
	if err != nil {
		t.Fatalf("registry.Load() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("registry.Load() warnings = %v, want none", warnings)
	}
	return reg
}

// captureRealStdio redirects the process's actual os.Stdout/os.Stderr for
// the duration of fn and returns what each received.
//
// This is deliberately NOT the same as cmd.SetOut/cmd.SetErr: cobra's
// Command.Print/Printf/Println route through OutOrStderr(), which
// returns the out-writer if one was set at all - regardless of which
// writer the call "means" to target - and only falls back to the
// default it was given (stdout for OutOrStdout, stderr for
// OutOrStderr) when no out-writer is set. So a test harness that calls
// both SetOut(&stdout) and SetErr(&stderr) can't actually distinguish
// a cmd.Println payload call from a correct fmt.Fprintln(cmd.OutOrStdout(), ...)
// one: both land in the SetOut buffer either way. Production never
// calls Set(Out|Err) at all, so the only way to reproduce - and pin -
// the real stdout/stderr split is to exercise the command exactly like
// production does and observe where bytes actually land on the real
// streams. No test in this package calls t.Parallel(), so swapping the
// package-global os.Stdout/os.Stderr here is safe.
func captureRealStdio(t *testing.T, fn func()) (stdout, stderr string) {
	t.Helper()

	origStdout, origStderr := os.Stdout, os.Stderr
	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stdout, os.Stderr = wOut, wErr
	defer func() { os.Stdout, os.Stderr = origStdout, origStderr }()

	outCh := make(chan string, 1)
	errCh := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, rOut)
		outCh <- buf.String()
	}()
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, rErr)
		errCh <- buf.String()
	}()

	fn()

	_ = wOut.Close()
	_ = wErr.Close()
	stdout, stderr = <-outCh, <-errCh
	_ = rOut.Close()
	_ = rErr.Close()
	return stdout, stderr
}
