package cli_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/arobson/proji/internal/cli"
	"github.com/arobson/proji/internal/git/gittest"
)

func TestEnsureGit_SkipsBootstrapWhenGitFound(t *testing.T) {
	td := newTestDeps(t)
	bootstrapRan := false
	td.Deps.NewBootstrapper = func() cli.GitBootstrapper {
		bootstrapRan = true
		return stubBootstrapper{}
	}
	td.Runner.Responses = []*gittest.Response{
		{Match: gittest.MatchPrefix("rev-parse", "--is-inside-work-tree"), Stdout: "true\n"},
		{Match: gittest.MatchPrefix("status", "--porcelain"), Stdout: ""},
		{Match: gittest.MatchPrefix("rev-list"), Stdout: "0\n"},
	}

	if err := runCLI(t, td.Deps, "checkin"); err != nil {
		t.Fatalf("checkin: unexpected error: %v", err)
	}
	if bootstrapRan {
		t.Error("bootstrap should not run when git is already on PATH")
	}
}

func TestEnsureGit_RunsBootstrapWhenGitMissing(t *testing.T) {
	td := newTestDeps(t)
	td.Deps.LookPath = func(string) (string, error) { return "", errors.New("not found") }
	bootstrapRan := false
	td.Deps.NewBootstrapper = func() cli.GitBootstrapper {
		bootstrapRan = true
		return stubBootstrapper{err: errors.New("git is required to use proji. Install it, then run this command again")}
	}

	err := runCLI(t, td.Deps, "checkin")
	if err == nil {
		t.Fatal("checkin: expected an error when the bootstrap fails")
	}
	if !bootstrapRan {
		t.Error("bootstrap should have run when git is missing")
	}
	if !strings.Contains(err.Error(), "Install it") {
		t.Errorf("error = %q, want the bootstrap's error propagated", err.Error())
	}
	for _, c := range td.Runner.Calls {
		t.Errorf("should not have attempted any git operations, got call: %v", c)
	}
}

func TestEnsureGit_ProceedsAfterSuccessfulBootstrap(t *testing.T) {
	td := newTestDeps(t)
	td.Deps.LookPath = func(string) (string, error) { return "", errors.New("not found") }
	td.Deps.NewBootstrapper = func() cli.GitBootstrapper {
		return stubBootstrapper{} // succeeds
	}
	// No responses are scripted on td.Runner, so IsGitRepo's underlying
	// call fails and it reports "not a git repository" — proving
	// execution reached the real git.Repo calls rather than stopping at
	// ensureGit (which would instead surface the bootstrap's own error).

	err := runCLI(t, td.Deps, "checkin")
	if err == nil {
		t.Fatal("checkin: expected an error past a successful-but-unscripted bootstrap")
	}
	if !strings.Contains(err.Error(), "git repository") {
		t.Errorf("error = %q, want the not-a-git-repo message (proving bootstrap didn't block execution)", err.Error())
	}
}
