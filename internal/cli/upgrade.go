package cli

import "github.com/spf13/cobra"

func newUpgradeCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "upgrade",
		Short: "Check for a newer version of proji and install it",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return deps.NewUpgrader().Run(cmd.Context())
		},
	}
}
