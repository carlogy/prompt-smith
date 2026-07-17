package naming

import (
	"testing"
	"time"
)

func TestSuggestFilename(t *testing.T) {
	ts := time.Date(2026, 7, 16, 14, 3, 12, 0, time.UTC)

	cases := []struct {
		name string
		goal string
		want string
	}{
		{
			name: "typical goal",
			goal: "Fix the flaky checkout test",
			want: "promptsmith-20260716T140312Z-fix-the-flaky-checkout-test.txt",
		},
		{
			name: "empty goal falls back to timestamp only",
			goal: "",
			want: "promptsmith-20260716T140312Z.txt",
		},
		{
			name: "whitespace-only goal falls back to timestamp only",
			goal: "   ",
			want: "promptsmith-20260716T140312Z.txt",
		},
		{
			name: "punctuation is sanitized and collapsed",
			goal: "Debug the API's rate-limiter (v2)!!",
			want: "promptsmith-20260716T140312Z-debug-the-api-s-rate-limiter.txt",
		},
		{
			name: "long goal is capped at 6 words",
			goal: "one two three four five six seven eight nine",
			want: "promptsmith-20260716T140312Z-one-two-three-four-five-six.txt",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := SuggestFilename(tc.goal, ts)
			if got != tc.want {
				t.Errorf("SuggestFilename(%q, ts) = %q, want %q", tc.goal, got, tc.want)
			}
		})
	}
}
