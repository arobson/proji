package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newInitCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Set up git on this computer: identity, default branch, and an SSH key registered with GitHub",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runInit(cmd, deps)
		},
	}
}

// runInit is idempotent: every step it performs is safe to run again,
// skipping (with a message) whatever is already done.
func runInit(cmd *cobra.Command, deps *Deps) error {
	ctx := cmd.Context()
	b := deps.NewBootstrapper()

	if _, err := deps.LookPath("git"); err != nil {
		if err := b.InstallGit(ctx); err != nil {
			return err
		}
	} else {
		fmt.Fprintln(deps.Out, "git is already installed, skipping.")
	}

	return b.ConfigureGit(ctx)
}
