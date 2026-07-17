package cli

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

// expandPath resolves a leading "~" (the calling user's home directory)
// or "~name" (that user's home directory) in path, the same shorthand a
// shell would expand. It's a no-op for any path that doesn't start with
// "~" (absolute, relative, or empty). Shared by the flag-only (--out)
// and TUI save-path delivery paths so both accept the same shorthand.
func expandPath(path string) (string, error) {
	if path == "" || path[0] != '~' {
		return path, nil
	}

	rest := path[1:]
	if rest == "" || rest[0] == '/' {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("promptsmith: resolve home directory: %w", err)
		}
		return filepath.Join(home, rest), nil
	}

	name, tail, _ := strings.Cut(rest, "/")
	u, err := user.Lookup(name)
	if err != nil {
		return "", fmt.Errorf("promptsmith: resolve home directory for %q: %w", name, err)
	}
	return filepath.Join(u.HomeDir, tail), nil
}
