package cli

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/arobson/proji/internal/ghclient"
	"github.com/arobson/proji/internal/git"
)

// ensureGitHubRepo returns an existing name repository under the
// authenticated user's account, creating it (as public/private per
// private) only if it doesn't already exist. Idempotent: re-running this
// after name was already created just reuses it instead of erroring.
func ensureGitHubRepo(ctx context.Context, client GitHubAPI, name string, private bool, out io.Writer) (ghclient.RepoResult, error) {
	login, err := client.CurrentUser(ctx)
	if err != nil {
		return ghclient.RepoResult{}, fmt.Errorf("look up your GitHub username: %w", err)
	}

	existing, err := client.GetRepo(ctx, login, name)
	switch {
	case err == nil:
		fmt.Fprintf(out, "%s/%s already exists on GitHub, reusing it.\n", existing.Owner, existing.Repo)
		return existing, nil
	case errors.Is(err, ghclient.ErrRepoNotFound):
		// fall through to create it below
	default:
		return ghclient.RepoResult{}, fmt.Errorf("check for an existing %q repository: %w", name, err)
	}

	result, err := client.CreateRepo(ctx, name, private)
	if err != nil {
		return ghclient.RepoResult{}, fmt.Errorf("could not create %q on GitHub: %w", name, err)
	}
	fmt.Fprintf(out, "Created %s/%s on GitHub\n", result.Owner, result.Repo)
	return result, nil
}

// ensureLocalGitRepo initializes repo's directory as a git repository,
// skipping the step if it already is one.
func ensureLocalGitRepo(ctx context.Context, repo *git.Repo, out io.Writer) error {
	if repo.IsGitRepo(ctx) {
		fmt.Fprintln(out, "This folder is already a git repository, skipping.")
		return nil
	}
	if err := repo.Init(ctx, "main"); err != nil {
		return fmt.Errorf("could not initialize the local repository: %w", err)
	}
	fmt.Fprintln(out, "Initialized a new git repository.")
	return nil
}

// ensureCommit stages and commits every change in repo with message,
// skipping the step if there's nothing new to commit.
func ensureCommit(ctx context.Context, repo *git.Repo, message string, out io.Writer) error {
	dirty, err := repo.HasChanges(ctx, ".")
	if err != nil {
		return fmt.Errorf("check for changes to commit: %w", err)
	}
	if !dirty {
		fmt.Fprintln(out, "Nothing new to commit, skipping.")
		return nil
	}

	if err := repo.AddAll(ctx, "."); err != nil {
		return fmt.Errorf("could not stage your files: %w", err)
	}
	if err := repo.Commit(ctx, message); err != nil {
		if errors.Is(err, git.ErrNothingToCommit) {
			fmt.Fprintln(out, "Nothing new to commit, skipping.")
			return nil
		}
		return fmt.Errorf("could not create the commit: %w", err)
	}
	fmt.Fprintf(out, "Committed: %q\n", message)
	return nil
}

// ensureRemote configures repo's name remote to point at url, skipping the
// step if that remote is already configured.
func ensureRemote(ctx context.Context, repo *git.Repo, name, url string, out io.Writer) error {
	exists, err := repo.RemoteExists(ctx, name)
	if err != nil {
		return fmt.Errorf("check the %q remote: %w", name, err)
	}
	if exists {
		fmt.Fprintf(out, "Remote %q is already configured, skipping.\n", name)
		return nil
	}
	if err := repo.AddRemote(ctx, name, url); err != nil {
		return fmt.Errorf("could not configure the %q remote: %w", name, err)
	}
	fmt.Fprintf(out, "Configured remote %q.\n", name)
	return nil
}
