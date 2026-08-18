package cli

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/arobson/proji/internal/git"
)

func newCopyCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "copy owner/repository",
		Short: "Fork an instructor's repository and clone your copy of it",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCopy(cmd, deps, args[0])
		},
	}
}

func runCopy(cmd *cobra.Command, deps *Deps, ownerRepo string) error {
	owner, repoName, ok := strings.Cut(ownerRepo, "/")
	if !ok || owner == "" || repoName == "" {
		return fmt.Errorf(`expected an argument in the form owner/repository, e.g. "proji copy instructor/homework-1"`)
	}

	ctx := cmd.Context()
	if err := ensureGit(ctx, deps); err != nil {
		return err
	}

	token, client, err := ensureGitHubToken(ctx, deps)
	if err != nil {
		return err
	}

	fork, err := client.ForkRepo(ctx, owner, repoName)
	if err != nil {
		return fmt.Errorf("could not fork %s/%s: %w. Check the repository name and that you have access to it", owner, repoName, err)
	}
	fmt.Fprintf(deps.Out, "Forked %s/%s to %s/%s\n", owner, repoName, fork.Owner, fork.Repo)

	cwd, err := deps.Getwd()
	if err != nil {
		return fmt.Errorf("could not determine the current directory: %w", err)
	}
	destDir := filepath.Join(cwd, repoName)
	if _, err := os.Stat(destDir); err == nil {
		return fmt.Errorf("a directory named %q already exists here. Remove it or run this command somewhere else", repoName)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("could not check destination directory: %w", err)
	}

	cloneURL, err := withToken(fork.CloneURL, token)
	if err != nil {
		return fmt.Errorf("could not build a clone URL for %s/%s: %w", fork.Owner, fork.Repo, err)
	}
	parent := git.NewRepo(deps.NewRunner(), cwd)
	if err := parent.Clone(ctx, cloneURL, destDir); err != nil {
		return fmt.Errorf("failed to clone %s/%s: %w", fork.Owner, fork.Repo, err)
	}

	upstreamURL := fmt.Sprintf("https://github.com/%s/%s.git", owner, repoName)
	newRepo := git.NewRepo(deps.NewRunner(), destDir)
	if err := newRepo.AddRemote(ctx, "upstream", upstreamURL); err != nil {
		return fmt.Errorf("cloned successfully, but could not configure the upstream remote: %w", err)
	}

	fmt.Fprintf(deps.Out, "Copied your repository to %s\n", destDir)
	fmt.Fprintf(deps.Out, "Run \"cd %s\" to start working.\n", destDir)
	return nil
}

// withToken returns rawURL with token embedded as the HTTPS userinfo, so
// git push works without a system credential helper.
func withToken(rawURL, token string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	u.User = url.UserPassword("x-access-token", token)
	return u.String(), nil
}
