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

package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/carlogy/prompt-smith/internal/prompt"
)

// TestPreview_BuildErrorShowsBannerAndNoHints pins requirements 1 and 3:
// an unknown target makes prompt.Build fail, and the viewport content
// shows ONLY the styled error banner - no promptlint findings
// alongside it, mirroring the web UI's preview.html (its {{if .Error}}
// branch owns the whole pane; Findings is never populated when
// buildErr != nil - see internal/server/preview.go's handlePreview).
func TestPreview_BuildErrorShowsBannerAndNoHints(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "does-not-exist", Goal: "g"})

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m2 := updated.(model)

	got := stripANSI(m2.previewVP.View())
	if !strings.Contains(got, `Error: prompt: unknown target "does-not-exist"`) {
		t.Errorf("expected the build error rendered as a banner, got:\n%s", got)
	}
	if strings.Contains(got, "Suggestions") {
		t.Errorf("expected no hints block alongside a build error, got:\n%s", got)
	}
}

// TestPreview_SuccessfulBuildRendersHintsAboveThePrompt pins
// requirement 2: on a successful build, promptlint's findings render
// as a styled "Suggestions" block ABOVE the built prompt inside the
// same viewport content. The goal "g" is both selected as a skill's
// output would confirm the prompt built, and short enough
// (< minGoalChars) to guarantee at least one concrete finding fires.
func TestPreview_SuccessfulBuildRendersHintsAboveThePrompt(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g", Skills: []string{"diagnose"}})

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m2 := updated.(model)

	got := stripANSI(m2.previewVP.View())
	suggestionsIdx := strings.Index(got, "Suggestions")
	if suggestionsIdx == -1 {
		t.Fatalf("expected a Suggestions block for a hint-worthy prompt, got:\n%s", got)
	}
	promptIdx := strings.Index(got, "diagnose body")
	if promptIdx == -1 {
		t.Fatalf("expected the built prompt to still render, got:\n%s", got)
	}
	if suggestionsIdx >= promptIdx {
		t.Errorf("expected the hints block ABOVE the prompt, got:\n%s", got)
	}
	if !strings.Contains(got, "The goal is only 1 characters") {
		t.Errorf("expected the short-goal finding's message, got:\n%s", got)
	}
}

// TestPreview_WellFormedPromptHasNoHintsBlock is the mirror of
// TestPreview_SuccessfulBuildRendersHintsAboveThePrompt: a prompt with
// nothing left for promptlint to flag renders no Suggestions block at
// all, matching renderHints' "" return for an empty findings slice.
func TestPreview_WellFormedPromptHasNoHintsBlock(t *testing.T) {
	reg := fixtureRegistry()
	m := newModel(reg, prompt.Inputs{
		Target:       "generic",
		Goal:         "fix the flaky checkout test end to end",
		Skills:       []string{"diagnose", "verify"},
		Role:         "a senior engineer",
		OutputFormat: "a unified diff",
		Examples:     []string{"in -> out", "in2 -> out2", "in3 -> out3"},
	})

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m2 := updated.(model)

	got := stripANSI(m2.previewVP.View())
	if strings.Contains(got, "Suggestions") {
		t.Errorf("expected no hints block for a well-formed prompt, got:\n%s", got)
	}
}

// TestPreview_BannerAndHintsVisibleWithoutScrolling pins requirement 4:
// GotoTop already runs on every recomputePreview, so both the error
// banner and the hints block - now part of the viewport's own content
// rather than separate chrome - must still land at YOffset 0 and
// appear in the viewport's rendered View() with no scrolling needed.
func TestPreview_BannerAndHintsVisibleWithoutScrolling(t *testing.T) {
	reg := fixtureRegistry()

	t.Run("error banner", func(t *testing.T) {
		m := newModel(reg, prompt.Inputs{Target: "does-not-exist", Goal: "g"})
		updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
		m2 := updated.(model)

		if m2.previewVP.YOffset != 0 {
			t.Fatalf("expected scroll to reset to top on recompute, got YOffset=%d", m2.previewVP.YOffset)
		}
		if !strings.Contains(stripANSI(m2.previewVP.View()), "Error:") {
			t.Errorf("expected the banner visible at the top without scrolling, got:\n%s", stripANSI(m2.previewVP.View()))
		}
	})

	t.Run("hints block", func(t *testing.T) {
		m := newModel(reg, prompt.Inputs{Target: "generic", Goal: "g", Skills: []string{"diagnose"}})
		updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
		m2 := updated.(model)

		if m2.previewVP.YOffset != 0 {
			t.Fatalf("expected scroll to reset to top on recompute, got YOffset=%d", m2.previewVP.YOffset)
		}
		if !strings.Contains(stripANSI(m2.previewVP.View()), "Suggestions") {
			t.Errorf("expected the hints block visible at the top without scrolling, got:\n%s", stripANSI(m2.previewVP.View()))
		}
	})
}
