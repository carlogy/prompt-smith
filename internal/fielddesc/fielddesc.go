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
	OutputFormat: "The shape of the response you want.",
}

// Sentence returns the canonical descriptive sentence for field, or
// "" if field isn't one of the known constants above.
func Sentence(field string) string {
	return sentences[field]
}
