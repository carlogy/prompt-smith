package cli

import "github.com/spf13/cobra"

// newListCmd builds the "list" subcommand, which will browse registry
// skills by category and target. Behavior lands in a later build phase.
func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available skills by category and target",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return ErrNotImplemented
		},
	}
}
