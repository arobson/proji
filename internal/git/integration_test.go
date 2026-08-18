//go:build integration

package git_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/arobson/proji/internal/git"
)

// runGit is a tiny helper for setting up fixture repos directly; it is
// intentionally separate from the ExecRunner under test.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=proji-test", "GIT_AUTHOR_EMAIL=proji-test@example.com",
		"GIT_COMMITTER_NAME=proji-test", "GIT_COMMITTER_EMAIL=proji-test@example.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

// newRemoteAndClone creates a bare "remote" repo with one commit and a
// working clone of it, returning both directories. It also sets a git
// identity for the whole test via env vars, since commits made through
// the real ExecRunner (not just the runGit helper) need one too, and a
// clean CI runner has no global git config at all.
func newRemoteAndClone(t *testing.T) (remoteDir, cloneDir string) {
	t.Helper()
	t.Setenv("GIT_AUTHOR_NAME", "proji-test")
	t.Setenv("GIT_AUTHOR_EMAIL", "proji-test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "proji-test")
	t.Setenv("GIT_COMMITTER_EMAIL", "proji-test@example.com")

	base := t.TempDir()
	remoteDir = filepath.Join(base, "remote.git")
	seedDir := filepath.Join(base, "seed")
	cloneDir = filepath.Join(base, "clone")

	runGit(t, base, "init", "--bare", remoteDir)

	if err := os.MkdirAll(seedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, seedDir, "init", "-b", "main")
	writeFile(t, filepath.Join(seedDir, "README.md"), "hello\n")
	runGit(t, seedDir, "add", "README.md")
	runGit(t, seedDir, "commit", "-m", "initial commit")
	runGit(t, seedDir, "remote", "add", "origin", remoteDir)
	runGit(t, seedDir, "push", "origin", "main")

	runGit(t, base, "clone", remoteDir, cloneDir)
	return remoteDir, cloneDir
}

func TestIntegration_FetchAndFastForward(t *testing.T) {
	remoteDir, cloneDir := newRemoteAndClone(t)

	// Advance the remote via a second working copy.
	base := t.TempDir()
	otherClone := filepath.Join(base, "other")
	runGit(t, base, "clone", remoteDir, otherClone)
	writeFile(t, filepath.Join(otherClone, "NEW.md"), "new\n")
	runGit(t, otherClone, "add", "NEW.md")
	runGit(t, otherClone, "commit", "-m", "add NEW.md")
	runGit(t, otherClone, "push", "origin", "main")

	runner := git.NewExecRunner()
	repo := git.NewRepo(runner, cloneDir)
	ctx := context.Background()

	if err := repo.Fetch(ctx, "origin", "main"); err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	upToDate, err := repo.IsUpToDate(ctx, "FETCH_HEAD")
	if err != nil {
		t.Fatalf("IsUpToDate() error = %v", err)
	}
	if upToDate {
		t.Fatal("IsUpToDate() = true, want false before merging new remote commit")
	}
	if err := repo.MergeFFOnly(ctx, "FETCH_HEAD"); err != nil {
		t.Fatalf("MergeFFOnly() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(cloneDir, "NEW.md")); err != nil {
		t.Fatalf("expected NEW.md after fast-forward: %v", err)
	}
}

func TestIntegration_RebaseSucceedsWithoutConflict(t *testing.T) {
	remoteDir, cloneDir := newRemoteAndClone(t)

	base := t.TempDir()
	otherClone := filepath.Join(base, "other")
	runGit(t, base, "clone", remoteDir, otherClone)
	writeFile(t, filepath.Join(otherClone, "REMOTE.md"), "remote change\n")
	runGit(t, otherClone, "add", "REMOTE.md")
	runGit(t, otherClone, "commit", "-m", "remote change")
	runGit(t, otherClone, "push", "origin", "main")

	writeFile(t, filepath.Join(cloneDir, "LOCAL.md"), "local change\n")
	runGit(t, cloneDir, "add", "LOCAL.md")
	runGit(t, cloneDir, "commit", "-m", "local change")

	runner := git.NewExecRunner()
	repo := git.NewRepo(runner, cloneDir)
	ctx := context.Background()

	if err := repo.Fetch(ctx, "origin", "main"); err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if err := repo.MergeFFOnly(ctx, "FETCH_HEAD"); err == nil {
		t.Fatal("MergeFFOnly() error = nil, want failure (histories diverged)")
	}
	if err := repo.Rebase(ctx, "FETCH_HEAD"); err != nil {
		t.Fatalf("Rebase() error = %v", err)
	}
	for _, f := range []string{"REMOTE.md", "LOCAL.md"} {
		if _, err := os.Stat(filepath.Join(cloneDir, f)); err != nil {
			t.Fatalf("expected %s after rebase: %v", f, err)
		}
	}
}

func TestIntegration_RebaseConflictAborts(t *testing.T) {
	remoteDir, cloneDir := newRemoteAndClone(t)

	base := t.TempDir()
	otherClone := filepath.Join(base, "other")
	runGit(t, base, "clone", remoteDir, otherClone)
	writeFile(t, filepath.Join(otherClone, "README.md"), "remote wins\n")
	runGit(t, otherClone, "add", "README.md")
	runGit(t, otherClone, "commit", "-m", "remote edits README")
	runGit(t, otherClone, "push", "origin", "main")

	writeFile(t, filepath.Join(cloneDir, "README.md"), "local wins\n")
	runGit(t, cloneDir, "add", "README.md")
	runGit(t, cloneDir, "commit", "-m", "local edits README")

	runner := git.NewExecRunner()
	repo := git.NewRepo(runner, cloneDir)
	ctx := context.Background()

	if err := repo.Fetch(ctx, "origin", "main"); err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if err := repo.MergeFFOnly(ctx, "FETCH_HEAD"); err == nil {
		t.Fatal("MergeFFOnly() error = nil, want failure")
	}
	if err := repo.Rebase(ctx, "FETCH_HEAD"); err == nil {
		t.Fatal("Rebase() error = nil, want a conflict")
	}
	if err := repo.RebaseAbort(ctx); err != nil {
		t.Fatalf("RebaseAbort() error = %v", err)
	}

	dirty, err := repo.HasChanges(ctx, ".")
	if err != nil {
		t.Fatalf("HasChanges() error = %v", err)
	}
	if dirty {
		t.Fatal("HasChanges() = true after RebaseAbort, want a clean working tree restored to pre-rebase state")
	}
	data, err := os.ReadFile(filepath.Join(cloneDir, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "local wins\n" {
		t.Fatalf("README.md = %q after abort, want local content preserved", data)
	}
}

func TestIntegration_CommitAndPush(t *testing.T) {
	_, cloneDir := newRemoteAndClone(t)

	runner := git.NewExecRunner()
	repo := git.NewRepo(runner, cloneDir)
	ctx := context.Background()

	writeFile(t, filepath.Join(cloneDir, "WORK.md"), "work\n")
	if err := repo.AddAll(ctx, "."); err != nil {
		t.Fatalf("AddAll() error = %v", err)
	}
	if err := repo.Commit(ctx, "add WORK.md"); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
	if err := repo.Push(ctx, "origin", "HEAD"); err != nil {
		t.Fatalf("Push() error = %v", err)
	}

	if err := repo.Commit(ctx, "nothing changed"); err == nil {
		t.Fatal("Commit() error = nil with no staged changes, want ErrNothingToCommit")
	}
}
