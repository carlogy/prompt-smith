package cli

import (
	"testing"

	"github.com/carlogy/prompt-smith/internal/registry"
)

// testRegistry loads the real embedded registry for CLI-level tests. CLI
// tests exercise flag plumbing and output routing, not registry content,
// so using the real shipped data (already guarded by its own package's
// tests) avoids a third duplicate fixture.
func testRegistry(t *testing.T) *registry.Registry {
	t.Helper()
	reg, err := registry.Load()
	if err != nil {
		t.Fatalf("registry.Load() error = %v", err)
	}
	return reg
}
