package cli

import (
	"os"
	"os/user"
	"path/filepath"
	"testing"
)

func TestExpandPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("os.UserHomeDir() error = %v", err)
	}

	cases := []struct {
		name string
		path string
		want string
	}{
		{
			name: "bare tilde expands to home",
			path: "~",
			want: home,
		},
		{
			name: "tilde slash expands to home-relative path",
			path: "~/prompts/out.txt",
			want: filepath.Join(home, "prompts/out.txt"),
		},
		{
			name: "absolute path is unchanged",
			path: "/tmp/out.txt",
			want: "/tmp/out.txt",
		},
		{
			name: "relative path is unchanged",
			path: "out.txt",
			want: "out.txt",
		},
		{
			name: "empty path is unchanged",
			path: "",
			want: "",
		},
		{
			name: "tilde mid-word is unchanged (not a home shorthand)",
			path: "foo~bar",
			want: "foo~bar",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := expandPath(tc.path)
			if err != nil {
				t.Fatalf("expandPath(%q) error = %v", tc.path, err)
			}
			if got != tc.want {
				t.Errorf("expandPath(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

func TestExpandPath_NamedUser(t *testing.T) {
	me, err := user.Current()
	if err != nil {
		t.Skipf("user.Current() unavailable in this environment: %v", err)
	}
	if me.Username == "" {
		t.Skip("current user has no username to test against")
	}

	cases := []struct {
		name string
		path string
		want string
	}{
		{
			name: "named user expands to that user's home",
			path: "~" + me.Username + "/out.txt",
			want: filepath.Join(me.HomeDir, "out.txt"),
		},
		{
			name: "bare named user expands to that user's home directory",
			path: "~" + me.Username,
			want: me.HomeDir,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := expandPath(tc.path)
			if err != nil {
				t.Fatalf("expandPath(%q) error = %v", tc.path, err)
			}
			if got != tc.want {
				t.Errorf("expandPath(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

func TestExpandPath_UnknownUserErrors(t *testing.T) {
	_, err := expandPath("~this-user-should-not-exist-anywhere/out.txt")
	if err == nil {
		t.Fatal("expandPath() error = nil, want an error for an unresolvable user")
	}
}
