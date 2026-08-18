package git

import (
	"context"
	"errors"
	"strconv"
	"strings"
)

// Repo performs high-level git operations against a working directory,
// using a Runner to actually invoke git.
type Repo struct {
	runner Runner
	dir    string
}

// NewRepo returns a Repo that runs git commands with dir as the working
// directory (git itself resolves upward to find the enclosing .git).
func NewRepo(runner Runner, dir string) *Repo {
	return &Repo{runner: runner, dir: dir}
}

// IsGitRepo reports whether dir is inside a git work tree.
func (r *Repo) IsGitRepo(ctx context.Context) bool {
	stdout, _, err := r.runner.Run(ctx, r.dir, "rev-parse", "--is-inside-work-tree")
	return err == nil && strings.TrimSpace(stdout) == "true"
}

// Clone clones url into destDir.
func (r *Repo) Clone(ctx context.Context, url, destDir string) error {
	_, _, err := r.runner.Run(ctx, r.dir, "clone", url, destDir)
	return err
}

// Init initializes a new repository with the given initial branch name.
func (r *Repo) Init(ctx context.Context, branch string) error {
	_, _, err := r.runner.Run(ctx, r.dir, "init", "-b", branch)
	return err
}

// AddRemote adds a remote named name pointing at url.
func (r *Repo) AddRemote(ctx context.Context, name, url string) error {
	_, _, err := r.runner.Run(ctx, r.dir, "remote", "add", name, url)
	return err
}

// RemoteExists reports whether a remote named name is configured.
func (r *Repo) RemoteExists(ctx context.Context, name string) (bool, error) {
	_, _, err := r.runner.Run(ctx, r.dir, "remote", "get-url", name)
	if err != nil {
		var exitErr *ExitError
		if errors.As(err, &exitErr) && strings.Contains(strings.ToLower(exitErr.Stderr), "no such remote") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// CurrentBranch returns the name of the currently checked-out branch.
func (r *Repo) CurrentBranch(ctx context.Context) (string, error) {
	stdout, _, err := r.runner.Run(ctx, r.dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout), nil
}

// Fetch fetches branch from remote.
func (r *Repo) Fetch(ctx context.Context, remote, branch string) error {
	_, _, err := r.runner.Run(ctx, r.dir, "fetch", remote, branch)
	return err
}

// MergeFFOnly fast-forwards the current branch to ref, failing if that's
// not possible.
func (r *Repo) MergeFFOnly(ctx context.Context, ref string) error {
	_, _, err := r.runner.Run(ctx, r.dir, "merge", "--ff-only", ref)
	return err
}

// Rebase replays the current branch's commits on top of ref.
func (r *Repo) Rebase(ctx context.Context, ref string) error {
	_, _, err := r.runner.Run(ctx, r.dir, "rebase", ref)
	return err
}

// RebaseAbort cancels an in-progress rebase, restoring the pre-rebase state.
func (r *Repo) RebaseAbort(ctx context.Context) error {
	_, _, err := r.runner.Run(ctx, r.dir, "rebase", "--abort")
	return err
}

// Merge performs a plain (non-ff-only) merge of ref into the current branch.
func (r *Repo) Merge(ctx context.Context, ref string) error {
	_, _, err := r.runner.Run(ctx, r.dir, "merge", ref)
	return err
}

// HasConflicts reports whether any files currently have unresolved merge
// conflicts.
func (r *Repo) HasConflicts(ctx context.Context) (bool, error) {
	stdout, _, err := r.runner.Run(ctx, r.dir, "diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(stdout) != "", nil
}

// IsUpToDate reports whether HEAD already contains everything reachable
// from ref (i.e. there is nothing new to merge or rebase in).
func (r *Repo) IsUpToDate(ctx context.Context, ref string) (bool, error) {
	stdout, _, err := r.runner.Run(ctx, r.dir, "rev-list", "HEAD.."+ref, "--count")
	if err != nil {
		return false, err
	}
	count, err := strconv.Atoi(strings.TrimSpace(stdout))
	if err != nil {
		return false, err
	}
	return count == 0, nil
}

// HasChanges reports whether pathspec has any uncommitted changes
// (staged, unstaged, or untracked).
func (r *Repo) HasChanges(ctx context.Context, pathspec string) (bool, error) {
	stdout, _, err := r.runner.Run(ctx, r.dir, "status", "--porcelain", "--", pathspec)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(stdout) != "", nil
}

// AddAll stages every change under pathspec.
func (r *Repo) AddAll(ctx context.Context, pathspec string) error {
	_, _, err := r.runner.Run(ctx, r.dir, "add", "-A", "--", pathspec)
	return err
}

// Commit commits staged changes with message.
func (r *Repo) Commit(ctx context.Context, message string) error {
	_, _, err := r.runner.Run(ctx, r.dir, "commit", "-m", message)
	if err != nil {
		var exitErr *ExitError
		if errors.As(err, &exitErr) && strings.Contains(strings.ToLower(exitErr.Stderr), "nothing to commit") {
			return ErrNothingToCommit
		}
		return err
	}
	return nil
}

// Push pushes ref to remote, setting up upstream tracking.
func (r *Repo) Push(ctx context.Context, remote, ref string) error {
	_, _, err := r.runner.Run(ctx, r.dir, "push", "-u", remote, ref)
	return err
}
