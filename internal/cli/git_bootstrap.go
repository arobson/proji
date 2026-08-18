package cli

import "context"

// ensureGit makes sure the git CLI is available before a command tries to
// use it, bootstrapping it (with the user's confirmation) if it's missing.
func ensureGit(ctx context.Context, deps *Deps) error {
	if _, err := deps.LookPath("git"); err == nil {
		return nil
	}
	return deps.NewBootstrapper().Run(ctx)
}
