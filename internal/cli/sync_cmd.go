package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/arobson/proji/internal/git"
	"github.com/arobson/proji/internal/sync"
)

var errNotAGitRepo = errors.New(`this isn't a git repository proji can manage. Run this from inside a project you copied with "proji copy"`)

// runSync implements the shared body of "proji check upstream" and
// "proji checkout": fetch remote, then fast-forward, rebase, or (with
// confirmation) merge. sourceLabel names the remote in success messages
// ("upstream", or "" for a generic "origin" message).
func runSync(cmd *cobra.Command, deps *Deps, remote, sourceLabel, missingRemoteMsg string) error {
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

	exists, err := repo.RemoteExists(ctx, remote)
	if err != nil {
		return fmt.Errorf("could not check the %q remote: %w", remote, err)
	}
	if !exists {
		return errors.New(missingRemoteMsg)
	}

	branch, err := repo.CurrentBranch(ctx)
	if err != nil {
		return fmt.Errorf("could not determine the current branch: %w", err)
	}

	syncer := &sync.Syncer{Repo: repo, Prompt: deps.Prompt}
	result, err := syncer.Sync(ctx, remote, branch)
	if err != nil {
		return fmt.Errorf("could not check %s for changes: %w", remote, err)
	}

	fmt.Fprintln(deps.Out, describeSyncResult(result.Action, sourceLabel))
	return nil
}

func describeSyncResult(action sync.Action, sourceLabel string) string {
	suffix := ""
	if sourceLabel != "" {
		suffix = " from " + sourceLabel
	}
	switch action {
	case sync.ActionUpToDate:
		if sourceLabel != "" {
			return fmt.Sprintf("Already up to date with %s.", sourceLabel)
		}
		return "Already up to date."
	case sync.ActionFastForward:
		return fmt.Sprintf("Fast-forwarded to the latest changes%s.", suffix)
	case sync.ActionRebase:
		return fmt.Sprintf("Your work has been rebased on top of the latest changes%s.", suffix)
	case sync.ActionMerge:
		return fmt.Sprintf("Merged the latest changes%s.", suffix)
	case sync.ActionMergeUnclean:
		return fmt.Sprintf("Merged the latest changes%s, but some files have conflicts that need to be resolved manually. Run \"git status\" to see them.", suffix)
	case sync.ActionAborted:
		return "No changes were applied. Your rebase was cancelled and your work is unchanged."
	default:
		return string(action)
	}
}
