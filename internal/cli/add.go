package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/arobson/proji/internal/git"
)

func newAddCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "add",
		Short: "Turn the current folder into a new public GitHub repository",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runAdd(cmd, deps)
		},
	}
}

// runAdd is idempotent: every step is safe to retry after a partial
// failure (a missing prerequisite, a network error, and so on) — a step
// that's already done is skipped, with a message, rather than erroring.
func runAdd(cmd *cobra.Command, deps *Deps) error {
	ctx := cmd.Context()
	if err := ensureGit(ctx, deps); err != nil {
		return err
	}

	cwd, err := deps.Getwd()
	if err != nil {
		return fmt.Errorf("could not determine the current directory: %w", err)
	}
	name := filepath.Base(cwd)
	if name == "." || name == string(filepath.Separator) || name == "" {
		return fmt.Errorf("could not determine a repository name from the current directory")
	}

	token, client, err := ensureGitHubToken(ctx, deps)
	if err != nil {
		return err
	}

	repoResult, err := ensureGitHubRepo(ctx, client, name, false, deps.Out)
	if err != nil {
		return err
	}

	repo := git.NewRepo(deps.NewRunner(), cwd)
	if err := ensureLocalGitRepo(ctx, repo, deps.Out); err != nil {
		return err
	}
	if err := ensureCommit(ctx, repo, "init: initializing", deps.Out); err != nil {
		return err
	}

	cloneURL, err := withToken(repoResult.CloneURL, token)
	if err != nil {
		return fmt.Errorf("could not build a clone URL for %s/%s: %w", repoResult.Owner, repoResult.Repo, err)
	}
	if err := ensureRemote(ctx, repo, "origin", cloneURL, deps.Out); err != nil {
		return err
	}

	branch, err := repo.CurrentBranch(ctx)
	if err != nil {
		return fmt.Errorf("could not determine the current branch: %w", err)
	}
	if err := repo.Push(ctx, "origin", branch); err != nil {
		return fmt.Errorf("could not push to GitHub: %w", err)
	}

	fmt.Fprintf(deps.Out, "Pushed %s to %s/%s\n", cwd, repoResult.Owner, repoResult.Repo)
	return nil
}
