package cli_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/arobson/proji/internal/ghclient"
	"github.com/arobson/proji/internal/git"
	"github.com/arobson/proji/internal/git/gittest"
)

func TestAdd_Success(t *testing.T) {
	td := newTestDeps(t)
	td.GitHub.createResult = ghclient.RepoResult{
		Owner:    "learner",
		Repo:     filepath.Base(td.Cwd),
		CloneURL: "https://github.com/learner/" + filepath.Base(td.Cwd) + ".git",
	}
	td.Runner.Responses = []*gittest.Response{
		{Match: gittest.MatchPrefix("rev-parse", "--is-inside-work-tree"), Stdout: "false\n"},
		{Match: gittest.MatchPrefix("init", "-b", "main")},
		{Match: gittest.MatchPrefix("status", "--porcelain"), Stdout: " A file.txt\n"},
		{Match: gittest.MatchPrefix("add", "-A")},
		{Match: gittest.MatchPrefix("commit", "-m")},
		{
			Match:  gittest.MatchPrefix("remote", "get-url"),
			Stderr: "error: No such remote 'origin'",
			Err:    &git.ExitError{ExitCode: 2, Stderr: "error: No such remote 'origin'"},
		},
		{Match: gittest.MatchPrefix("remote", "add")},
		{Match: gittest.MatchPrefix("rev-parse", "--abbrev-ref", "HEAD"), Stdout: "main\n"},
		{Match: gittest.MatchPrefix("push", "-u")},
	}

	if err := runCLI(t, td.Deps, "add"); err != nil {
		t.Fatalf("add: unexpected error: %v", err)
	}

	wantName := filepath.Base(td.Cwd)
	if len(td.GitHub.createCalls) != 1 || td.GitHub.createCalls[0] != wantName {
		t.Errorf("CreateRepo calls = %v, want [%s]", td.GitHub.createCalls, wantName)
	}

	var commitArgs, pushArgs []string
	for _, c := range td.Runner.Calls {
		if len(c.Args) > 0 && c.Args[0] == "commit" {
			commitArgs = c.Args
		}
		if len(c.Args) > 0 && c.Args[0] == "push" {
			pushArgs = c.Args
		}
	}
	if len(commitArgs) != 3 || commitArgs[2] != "init: initializing" {
		t.Errorf("commit args = %v, want message %q", commitArgs, "init: initializing")
	}
	if len(pushArgs) != 4 || pushArgs[2] != "origin" || pushArgs[3] != "main" {
		t.Errorf("push args = %v, want origin main", pushArgs)
	}

	out := td.Out.String()
	if !strings.Contains(out, "Pushed "+td.Cwd) {
		t.Errorf("output = %q, want a pushed confirmation", out)
	}
}

func TestAdd_ReusesExistingGitHubAndLocalRepo(t *testing.T) {
	td := newTestDeps(t)
	name := filepath.Base(td.Cwd)
	td.GitHub.getRepoResult = ghclient.RepoResult{
		Owner:    "learner",
		Repo:     name,
		CloneURL: "https://github.com/learner/" + name + ".git",
	}
	td.Runner.Responses = []*gittest.Response{
		{Match: gittest.MatchPrefix("rev-parse", "--is-inside-work-tree"), Stdout: "true\n"},
		{Match: gittest.MatchPrefix("status", "--porcelain"), Stdout: ""},
		{Match: gittest.MatchPrefix("remote", "get-url"), Stdout: "https://github.com/learner/" + name + ".git\n"},
		{Match: gittest.MatchPrefix("rev-parse", "--abbrev-ref", "HEAD"), Stdout: "main\n"},
		{Match: gittest.MatchPrefix("push", "-u")},
	}

	if err := runCLI(t, td.Deps, "add"); err != nil {
		t.Fatalf("add: unexpected error: %v", err)
	}

	if len(td.GitHub.createCalls) != 0 {
		t.Errorf("CreateRepo should not have been called, got calls = %v", td.GitHub.createCalls)
	}
	out := td.Out.String()
	if !strings.Contains(out, "already exists on GitHub, reusing it") {
		t.Errorf("output = %q, want a GitHub reuse message", out)
	}
	if !strings.Contains(out, "This folder is already a git repository, skipping") {
		t.Errorf("output = %q, want a local reuse message", out)
	}
	if !strings.Contains(out, "Nothing new to commit, skipping") {
		t.Errorf("output = %q, want a commit skip message", out)
	}
	if !strings.Contains(out, `Remote "origin" is already configured, skipping`) {
		t.Errorf("output = %q, want a remote skip message", out)
	}
	for _, c := range td.Runner.Calls {
		if len(c.Args) > 0 && (c.Args[0] == "init" || c.Args[0] == "commit") {
			t.Errorf("should not have re-initialized or re-committed, got call: %v", c.Args)
		}
	}
}
