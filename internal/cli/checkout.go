package cli

import "github.com/spf13/cobra"

const missingOriginRemoteMsg = `no "origin" remote is configured for this repository`

func newCheckoutCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "checkout",
		Short: "Fetch your latest changes from your GitHub fork and bring them into your work",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runSync(cmd, deps, "origin", "", missingOriginRemoteMsg)
		},
	}
}
