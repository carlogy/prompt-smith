package prompthl

import "testing"

func TestClassify(t *testing.T) {
	cases := []struct {
		line string
		want Kind
	}{
		{"<task>", OpenTag},
		{"</task>", CloseTag},
		{"<output_format>", OpenTag},
		{"</output_format>", CloseTag},
		{"Fix the bug", Body},
		{"", Body},
		{"<not a valid tag", Body},
		{"find: glob", Body},
		{"Load the `diagnose` skill:", Body},
		{"<bad-tag>", Body}, // hyphen isn't in [a-z_] - the builder never emits one anyway
		{"< task>", Body},   // a space breaks the match
		{"<TASK>", Body},    // uppercase breaks the match - the builder only emits lowercase tags
		{"<task", Body},     // missing closing >
		{"task>", Body},     // missing opening <
	}

	for _, tc := range cases {
		if got := Classify(tc.line); got != tc.want {
			t.Errorf("Classify(%q) = %v, want %v", tc.line, got, tc.want)
		}
	}
}
