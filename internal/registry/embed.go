package registry

import (
	"fmt"
	"os"
)

// Load parses the registry embedded in this binary (see embeddedData in
// embed_default.go / embed_empty.go), then merges in any user-provided
// skills found in userSkillsDir (see userskills.go). A problem loading
// user skills never fails the whole load - it's reported back as a
// warning instead, so a bad drop-in can't take down an otherwise-working
// CLI; the caller decides how to surface warnings (Execute prints them
// to stderr).
//
// LoadFS does the actual embedded-registry parsing and is what tests
// exercise against synthetic filesystems.
func Load() (*Registry, []string, error) {
	sub, err := embeddedData()
	if err != nil {
		return nil, nil, fmt.Errorf("registry: %w", err)
	}

	base, err := LoadFS(sub)
	if err != nil {
		return nil, nil, err
	}

	dir, err := userSkillsDir()
	if err != nil {
		return base, []string{fmt.Sprintf("resolve user skills directory: %v", err)}, nil
	}

	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return base, nil, nil // most common case: no user skills directory at all
	}
	if err != nil {
		return base, []string{fmt.Sprintf("user skills directory %s: %v", dir, err)}, nil
	}
	if !info.IsDir() {
		return base, []string{fmt.Sprintf("user skills path %s is not a directory", dir)}, nil
	}

	userSkills, newCategories, warnings := loadUserSkills(os.DirFS(dir))
	return mergeUserSkills(base, userSkills, newCategories), warnings, nil
}
