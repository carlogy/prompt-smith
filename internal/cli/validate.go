package cli

import (
	"github.com/spf13/cobra"

	"github.com/carlogy/prompt-smith/internal/registry"
)

// newValidateCmd builds the "validate" subcommand: checks the registry's
// semantic integrity (duplicate ids, dangling categories/refs) before a
// rebuild ships.
func newValidateCmd(reg *registry.Registry) *cobra.Command {
	return &cobra.Command{
		Use:           "validate",
		Short:         "Validate the embedded skill registry",
		Example:       `  promptsmith validate`,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := reg.Validate(); err != nil {
				return err
			}
			cmd.Println("registry ok")
			return nil
		},
	}
}
