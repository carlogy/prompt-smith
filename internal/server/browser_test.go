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

package server

import (
	"slices"
	"testing"
)

func TestBrowserCommand(t *testing.T) {
	const url = "http://127.0.0.1:8080"

	cases := []struct {
		goos     string
		wantName string
		wantArgs []string
	}{
		{"darwin", "open", []string{url}},
		{"windows", "rundll32", []string{"url.dll,FileProtocolHandler", url}},
		{"linux", "xdg-open", []string{url}},
		{"freebsd", "xdg-open", []string{url}},
		{"openbsd", "xdg-open", []string{url}},
		{"netbsd", "xdg-open", []string{url}},
	}

	for _, tc := range cases {
		t.Run(tc.goos, func(t *testing.T) {
			name, args, err := browserCommand(tc.goos, url)
			if err != nil {
				t.Fatalf("browserCommand(%q, ...) error = %v", tc.goos, err)
			}
			if name != tc.wantName {
				t.Errorf("name = %q, want %q", name, tc.wantName)
			}
			if !slices.Equal(args, tc.wantArgs) {
				t.Errorf("args = %v, want %v", args, tc.wantArgs)
			}
		})
	}
}

func TestBrowserCommand_UnsupportedOSErrors(t *testing.T) {
	_, _, err := browserCommand("plan9", "http://127.0.0.1:8080")
	if err == nil {
		t.Fatal("browserCommand() error = nil, want an error for an unsupported OS")
	}
}
