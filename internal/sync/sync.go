// Package sync implements the three-tier fetch/fast-forward/rebase logic
// shared by proji's "check upstream" and "checkout" commands.
package sync

import (
	"context"
	"fmt"

	"github.com/arobson/proji/internal/prompt"
)

// gitRepo is the subset of *git.Repo that Syncer needs. Defined here (not
// imported directly as *git.Repo) so tests can substitute a narrower fake
// if ever needed; in practice callers pass a real *git.Repo.
type gitRepo interface {
	Fetch(ctx context.Context, remote, branch string) error
	IsUpToDate(ctx context.Context, ref string) (bool, error)
	MergeFFOnly(ctx context.Context, ref string) error
	Rebase(ctx context.Context, ref string) error
	RebaseAbort(ctx context.Context) error
	Merge(ctx context.Context, ref string) error
	HasConflicts(ctx context.Context) (bool, error)
}

// Action identifies which strategy Sync actually applied.
type Action string

const (
	ActionUpToDate     Action = "up-to-date"
	ActionFastForward  Action = "fast-forward"
	ActionRebase       Action = "rebase"
	ActionMerge        Action = "merge"
	ActionMergeUnclean Action = "merge-unclean"
	ActionAborted      Action = "aborted"
)

// Result reports what Sync did.
type Result struct {
	Action Action
}

// Syncer implements the fetch -> ff-only merge -> rebase -> (ask to merge)
// state machine.
type Syncer struct {
	Repo   gitRepo
	Prompt prompt.Prompter
}

const conflictPrompt = "Rebase hit conflicts and was cancelled to protect your work. " +
	"Merge instead? This may require you to resolve conflicts and could affect files you've changed."

// Sync fetches branch from remote and reconciles it with the current
// branch, preferring a fast-forward, falling back to a rebase, and finally
// asking the user before attempting a plain merge if a rebase conflicts.
func (s *Syncer) Sync(ctx context.Context, remote, branch string) (Result, error) {
	if err := s.Repo.Fetch(ctx, remote, branch); err != nil {
		return Result{}, fmt.Errorf("fetch %s/%s: %w", remote, branch, err)
	}

	upToDate, err := s.Repo.IsUpToDate(ctx, "FETCH_HEAD")
	if err != nil {
		return Result{}, fmt.Errorf("check for new changes: %w", err)
	}
	if upToDate {
		return Result{Action: ActionUpToDate}, nil
	}

	if err := s.Repo.MergeFFOnly(ctx, "FETCH_HEAD"); err == nil {
		return Result{Action: ActionFastForward}, nil
	}

	if err := s.Repo.Rebase(ctx, "FETCH_HEAD"); err == nil {
		return Result{Action: ActionRebase}, nil
	}

	if err := s.Repo.RebaseAbort(ctx); err != nil {
		return Result{}, fmt.Errorf("cancel conflicted rebase: %w", err)
	}

	confirmed, err := s.Prompt.Confirm(conflictPrompt, false)
	if err != nil {
		return Result{}, fmt.Errorf("ask about merging instead: %w", err)
	}
	if !confirmed {
		return Result{Action: ActionAborted}, nil
	}

	if err := s.Repo.Merge(ctx, "FETCH_HEAD"); err != nil {
		return Result{}, fmt.Errorf("merge: %w", err)
	}
	hasConflicts, err := s.Repo.HasConflicts(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("check merge result: %w", err)
	}
	if hasConflicts {
		return Result{Action: ActionMergeUnclean}, nil
	}
	return Result{Action: ActionMerge}, nil
}
