package cli

import "github.com/spf13/cobra"

// newValidateCmd builds the "validate" subcommand, which will check the
// embedded registry for integrity (bad categories, missing target bodies,
// dangling references) before a rebuild ships. Behavior lands in a later
// build phase.
func newValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate the embedded skill registry",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return ErrNotImplemented
		},
	}
}
