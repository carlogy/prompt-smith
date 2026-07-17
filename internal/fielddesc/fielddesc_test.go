package fielddesc

import "testing"

// TestSentence_EveryKnownFieldHasOne guards completeness: every field
// constant must resolve to a non-empty sentence, so neither consuming
// surface (the web hint, the TUI footer) can end up silently rendering
// blank text for a field that exists but was never given copy.
func TestSentence_EveryKnownFieldHasOne(t *testing.T) {
	for _, field := range []string{Target, Goal, Role, Context, Constraints, OutputFormat} {
		if Sentence(field) == "" {
			t.Errorf("Sentence(%q) is empty - every known field constant must have a sentence", field)
		}
	}
}

func TestSentence_UnknownFieldReturnsEmpty(t *testing.T) {
	if got := Sentence("not-a-real-field"); got != "" {
		t.Errorf("Sentence(unknown) = %q, want empty", got)
	}
}
