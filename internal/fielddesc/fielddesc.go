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

// Package fielddesc holds the one canonical descriptive sentence per
// prompt-input field, shared by the two surfaces where showing this
// exact text matters: the web UI's field hints and the TUI's footer
// descriptor. Deliberately minimal - the CLI's flag help and the
// TUI's inline placeholders stay their own, terser, local strings
// (see internal/cli/generate.go and internal/tui/model.go), since
// those have different space budgets and voices; only the one
// sentence that appears verbatim in two places lives here.
package fielddesc

// Field name constants. Keys match the JSON/form field names already
// used elsewhere (e.g. server's r.FormValue("outputFormat")).
const (
	Target       = "target"
	Goal         = "goal"
	Role         = "role"
	Context      = "context"
	Constraints  = "constraints"
	OutputFormat = "outputFormat"
)

// sentences holds the canonical sentence for each known field.
var sentences = map[string]string{
	Target:       "Which agent or harness the prompt is tuned for.",
	Goal:         "What you want the model to do.",
	Role:         "The persona the model should adopt.",
	Context:      "Background the model should know.",
	Constraints:  "Rules the solution must respect.",
	OutputFormat: "How the model should shape its response (e.g. a diff, JSON, bullets).",
}

// Sentence returns the canonical descriptive sentence for field, or
// "" if field isn't one of the known constants above.
func Sentence(field string) string {
	return sentences[field]
}
