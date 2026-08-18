package cli

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/arobson/proji/internal/git"
)

func newCheckinCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "checkin",
		Short: "Commit and push your work",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runCheckin(cmd, deps)
		},
	}
}

func runCheckin(cmd *cobra.Command, deps *Deps) error {
	ctx := cmd.Context()
	if err := ensureGit(ctx, deps); err != nil {
		return err
	}

	cwd, err := deps.Getwd()
	if err != nil {
		return fmt.Errorf("could not determine the current directory: %w", err)
	}
	repo := git.NewRepo(deps.NewRunner(), cwd)

	if !repo.IsGitRepo(ctx) {
		return errNotAGitRepo
	}

	dirty, err := repo.HasChanges(ctx, ".")
	if err != nil {
		return fmt.Errorf("could not check for changes: %w", err)
	}
	// No upstream tracking ref (e.g. never pushed yet) is treated as "not
	// up to date" so we still attempt to push any prior local commits.
	upToDate, _ := repo.IsUpToDate(ctx, "@{u}")

	if !dirty && upToDate {
		fmt.Fprintln(deps.Out, "Nothing to check in. Your work is already up to date.")
		return nil
	}

	answer, err := deps.Prompt.Ask("Enter a commit message (press Enter to use the default): ")
	if err != nil {
		return fmt.Errorf("could not read your commit message: %w", err)
	}

	dirname := filepath.Base(cwd)
	detail := answer
	if detail == "" {
		detail = deps.Now().Format("2006-01-02 15:04")
	}
	message := dirname + ": " + detail

	if err := repo.AddAll(ctx, "."); err != nil {
		return fmt.Errorf("could not stage your changes: %w", err)
	}

	if err := repo.Commit(ctx, message); err != nil {
		if errors.Is(err, git.ErrNothingToCommit) {
			fmt.Fprintln(deps.Out, "There's nothing to commit in this directory.")
			return nil
		}
		return fmt.Errorf("could not commit your changes: %w", err)
	}

	if err := repo.Push(ctx, "origin", "HEAD"); err != nil {
		return describePushError(err)
	}

	fmt.Fprintf(deps.Out, "Checked in and pushed: %q\n", message)
	return nil
}

func describePushError(err error) error {
	var exitErr *git.ExitError
	stderr := ""
	if errors.As(err, &exitErr) {
		stderr = strings.ToLower(exitErr.Stderr)
	}

	switch {
	case strings.Contains(stderr, "non-fast-forward") || strings.Contains(stderr, "fetch first"):
		return errors.New(`GitHub rejected the push because there are changes you don't have locally. Run "proji checkout" first, then try again`)
	case strings.Contains(stderr, "authentication") || strings.Contains(stderr, "permission denied") || strings.Contains(stderr, "could not read username"):
		return errors.New("GitHub rejected your credentials. Delete ~/.proji/creds.yml and run any proji command to sign in again")
	default:
		return fmt.Errorf("failed to push your changes: %w", err)
	}
}
