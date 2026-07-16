package prompt_test

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

// update regenerates .golden fixtures from current output. Run via
// `make update-golden` after an intentional behavior change.
var update = flag.Bool("update", false, "update .golden files")

// assertGolden compares got against testdata/<name>.golden, rewriting the
// fixture first when -update is passed.
func assertGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")

	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("update golden %s: %v", path, err)
		}
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run `make update-golden` if this is a new case)", path, err)
	}

	if got != string(want) {
		t.Errorf("%s mismatch:\n got:  %q\nwant:  %q", name, got, string(want))
	}
}
