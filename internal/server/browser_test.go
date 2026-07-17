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
