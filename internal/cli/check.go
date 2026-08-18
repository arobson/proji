package cli

import "github.com/spf13/cobra"

const missingUpstreamRemoteMsg = `no "upstream" remote is configured for this repository. "check upstream" only works in projects created with "proji copy"`

func newCheckCmd(deps *Deps) *cobra.Command {
	checkCmd := &cobra.Command{
		Use:   "check",
		Short: "Check a remote for changes without necessarily applying them yet",
	}
	checkCmd.AddCommand(newCheckUpstreamCmd(deps))
	return checkCmd
}

func newCheckUpstreamCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "upstream",
		Short: "Fetch changes from the instructor's original repository and bring them into your work",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runSync(cmd, deps, "upstream", "upstream", missingUpstreamRemoteMsg)
		},
	}
}
