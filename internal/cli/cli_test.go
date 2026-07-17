package cli

import (
	"testing"

	"github.com/carlogy/prompt-smith/internal/registry"
)

// testRegistry loads the real embedded registry for CLI-level tests. CLI
// tests exercise flag plumbing and output routing, not registry content,
// so using the real shipped data (already guarded by its own package's
// tests) avoids a third duplicate fixture.
//
// PROMPTSMITH_SKILLS_DIR is pinned to an empty temp directory so these
// tests stay hermetic regardless of the developer machine's real user
// skills directory (see registry.Load).
func testRegistry(t *testing.T) *registry.Registry {
	t.Helper()
	t.Setenv("PROMPTSMITH_SKILLS_DIR", t.TempDir())

	reg, warnings, err := registry.Load()
	if err != nil {
		t.Fatalf("registry.Load() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("registry.Load() warnings = %v, want none", warnings)
	}
	return reg
}
