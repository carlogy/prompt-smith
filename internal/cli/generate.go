package cli

import (
	"errors"

	"github.com/spf13/cobra"
)

// ErrNotImplemented is returned by commands whose behavior lands in a
// later build phase. It gives a clear signal during scaffolding instead of
// a silent no-op.
var ErrNotImplemented = errors.New("promptsmith: not yet implemented")

// runGenerate is the root command's action. Flag parsing and assembly wire
// up in later phases (internal/registry, internal/assemble); this stub
// keeps the command surface real and testable during scaffolding.
func runGenerate(cmd *cobra.Command, args []string) error {
	return ErrNotImplemented
}
