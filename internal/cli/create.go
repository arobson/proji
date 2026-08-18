package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/arobson/proji/internal/git"
)

func newCreateCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "create name",
		Short: "Create a new public GitHub repository and a matching local folder",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCreate(cmd, deps, args[0])
		},
	}
}

// runCreate is idempotent: every step is safe to retry after a partial
// failure (a missing prerequisite, a network error, and so on) — a step
// that's already done is skipped, with a message, rather than erroring.
func runCreate(cmd *cobra.Command, deps *Deps, name string) error {
	if strings.TrimSpace(name) == "" || strings.Contains(name, "/") {
		return fmt.Errorf(`expected a simple repository name without slashes, e.g. "proji create my-project"`)
	}

	ctx := cmd.Context()
	if err := ensureGit(ctx, deps); err != nil {
		return err
	}

	token, client, err := ensureGitHubToken(ctx, deps)
	if err != nil {
		return err
	}

	repoResult, err := ensureGitHubRepo(ctx, client, name, false, deps.Out)
	if err != nil {
		return err
	}

	cwd, err := deps.Getwd()
	if err != nil {
		return fmt.Errorf("could not determine the current directory: %w", err)
	}
	destDir := filepath.Join(cwd, name)
	repo := git.NewRepo(deps.NewRunner(), destDir)
	alreadyGitRepo, err := ensureLocalDir(ctx, deps, repo, destDir, name)
	if err != nil {
		return err
	}
	if !alreadyGitRepo {
		if err := repo.Init(ctx, "main"); err != nil {
			return fmt.Errorf("could not initialize the local repository: %w", err)
		}
		fmt.Fprintln(deps.Out, "Initialized a new git repository.")
	}

	if err := ensurePlaceholderReadme(destDir, name, deps.Out); err != nil {
		return err
	}
	if err := ensureCommit(ctx, repo, "Initial commit", deps.Out); err != nil {
		return err
	}

	cloneURL, err := withToken(repoResult.CloneURL, token)
	if err != nil {
		return fmt.Errorf("could not build a clone URL for %s/%s: %w", repoResult.Owner, repoResult.Repo, err)
	}
	if err := ensureRemote(ctx, repo, "origin", cloneURL, deps.Out); err != nil {
		return err
	}
	if err := repo.Push(ctx, "origin", "main"); err != nil {
		return fmt.Errorf("created the repository locally, but could not push it to GitHub: %w", err)
	}

	fmt.Fprintf(deps.Out, "Created your repository at %s\n", destDir)
	fmt.Fprintf(deps.Out, "Run \"cd %s\" to start working.\n", destDir)
	return nil
}

// ensureLocalDir makes sure destDir exists and is safe to reuse, returning
// whether it's already a git repository (so the caller can skip Init). If
// it's already there, it must be a git repository proji can continue with
// (most likely from a prior, partially-failed run of this same command) —
// anything else refuses rather than silently taking over an unrelated
// folder that happens to share the repository's name.
func ensureLocalDir(ctx context.Context, deps *Deps, repo *git.Repo, destDir, name string) (alreadyGitRepo bool, err error) {
	info, statErr := os.Stat(destDir)
	switch {
	case statErr == nil:
		if !info.IsDir() {
			return false, fmt.Errorf("a file named %q already exists here and isn't a directory", name)
		}
		if !repo.IsGitRepo(ctx) {
			return false, fmt.Errorf("a directory named %q already exists here and isn't a git repository proji created. Remove it or run this command somewhere else", name)
		}
		fmt.Fprintf(deps.Out, "%s already exists locally, continuing with it.\n", destDir)
		return true, nil
	case os.IsNotExist(statErr):
		if err := os.MkdirAll(destDir, 0o750); err != nil {
			return false, fmt.Errorf("your new repository was created on GitHub, but proji could not create %s locally: %w", destDir, err)
		}
		return false, nil
	default:
		return false, fmt.Errorf("could not check destination directory: %w", statErr)
	}
}

// ensurePlaceholderReadme writes a placeholder README.md, skipping the
// step (and preserving any existing content) if one is already there.
func ensurePlaceholderReadme(destDir, name string, out io.Writer) error {
	readmePath := filepath.Join(destDir, "README.md")
	if _, err := os.Stat(readmePath); err == nil {
		fmt.Fprintln(out, "README.md already exists, skipping.")
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("could not check for an existing README.md: %w", err)
	}
	if err := os.WriteFile(readmePath, []byte("# "+name+"\n"), 0o644); err != nil { // #nosec G306 -- README.md is meant to be world-readable, like the rest of the working tree
		return fmt.Errorf("could not create README.md: %w", err)
	}
	fmt.Fprintln(out, "Created a placeholder README.md.")
	return nil
}
