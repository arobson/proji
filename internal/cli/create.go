package cli

import (
	"fmt"
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

	repoResult, err := client.CreateRepo(ctx, name, false)
	if err != nil {
		return fmt.Errorf("could not create %q on GitHub: %w", name, err)
	}
	fmt.Fprintf(deps.Out, "Created %s/%s on GitHub\n", repoResult.Owner, repoResult.Repo)

	cwd, err := deps.Getwd()
	if err != nil {
		return fmt.Errorf("could not determine the current directory: %w", err)
	}
	destDir := filepath.Join(cwd, name)
	if _, err := os.Stat(destDir); err == nil {
		return fmt.Errorf("your new repository was created on GitHub, but a directory named %q already exists here. Remove it, then clone %s/%s yourself", name, repoResult.Owner, repoResult.Repo)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("could not check destination directory: %w", err)
	}
	if err := os.MkdirAll(destDir, 0o750); err != nil {
		return fmt.Errorf("your new repository was created on GitHub, but proji could not create %s locally: %w", destDir, err)
	}

	repo := git.NewRepo(deps.NewRunner(), destDir)
	if err := repo.Init(ctx, "main"); err != nil {
		return fmt.Errorf("could not initialize the local repository: %w", err)
	}

	readmePath := filepath.Join(destDir, "README.md")
	if err := os.WriteFile(readmePath, []byte("# "+name+"\n"), 0o644); err != nil { // #nosec G306 -- README.md is meant to be world-readable, like the rest of the working tree
		return fmt.Errorf("could not create README.md: %w", err)
	}
	if err := repo.AddAll(ctx, "."); err != nil {
		return fmt.Errorf("could not stage the placeholder README: %w", err)
	}
	if err := repo.Commit(ctx, "Initial commit"); err != nil {
		return fmt.Errorf("could not create the initial commit: %w", err)
	}

	cloneURL, err := withToken(repoResult.CloneURL, token)
	if err != nil {
		return fmt.Errorf("could not build a clone URL for %s/%s: %w", repoResult.Owner, repoResult.Repo, err)
	}
	if err := repo.AddRemote(ctx, "origin", cloneURL); err != nil {
		return fmt.Errorf("could not configure the origin remote: %w", err)
	}
	if err := repo.Push(ctx, "origin", "main"); err != nil {
		return fmt.Errorf("created the repository locally, but could not push it to GitHub: %w", err)
	}

	fmt.Fprintf(deps.Out, "Created your repository at %s\n", destDir)
	fmt.Fprintf(deps.Out, "Run \"cd %s\" to start working.\n", destDir)
	return nil
}
