package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arobson/proji/internal/ghclient"
	"github.com/arobson/proji/internal/git/gittest"
)

func TestCreate_Success(t *testing.T) {
	td := newTestDeps(t)
	td.GitHub.createResult = ghclient.RepoResult{
		Owner:    "learner",
		Repo:     "my-project",
		CloneURL: "https://github.com/learner/my-project.git",
	}
	td.Runner.Responses = []*gittest.Response{
		{Match: gittest.MatchPrefix("init", "-b", "main")},
		{Match: gittest.MatchPrefix("add", "-A")},
		{Match: gittest.MatchPrefix("commit", "-m")},
		{Match: gittest.MatchPrefix("remote", "add")},
		{Match: gittest.MatchPrefix("push", "-u")},
	}

	if err := runCLI(t, td.Deps, "create", "my-project"); err != nil {
		t.Fatalf("create: unexpected error: %v", err)
	}

	if len(td.GitHub.createCalls) != 1 || td.GitHub.createCalls[0] != "my-project" {
		t.Errorf("CreateRepo calls = %v, want [my-project]", td.GitHub.createCalls)
	}

	destDir := filepath.Join(td.Cwd, "my-project")
	out := td.Out.String()
	if !strings.Contains(out, "Created learner/my-project on GitHub") {
		t.Errorf("output missing creation confirmation: %q", out)
	}
	if !strings.Contains(out, "Created your repository at "+destDir) {
		t.Errorf("output missing local confirmation: %q", out)
	}
	if !strings.Contains(out, "cd "+destDir) {
		t.Errorf("output missing cd instruction: %q", out)
	}

	readme, err := os.ReadFile(filepath.Join(destDir, "README.md"))
	if err != nil {
		t.Fatalf("expected README.md to be written: %v", err)
	}
	if string(readme) != "# my-project\n" {
		t.Errorf("README.md = %q, want %q", readme, "# my-project\n")
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
	if len(commitArgs) != 3 || commitArgs[2] != "Initial commit" {
		t.Errorf("commit args = %v, want message %q", commitArgs, "Initial commit")
	}
	if len(pushArgs) != 4 || pushArgs[2] != "origin" || pushArgs[3] != "main" {
		t.Errorf("push args = %v, want origin main", pushArgs)
	}
}

func TestCreate_RejectsNameWithSlash(t *testing.T) {
	td := newTestDeps(t)

	err := runCLI(t, td.Deps, "create", "not/valid")
	if err == nil {
		t.Fatal("create: expected an error for a name containing a slash")
	}
	if len(td.GitHub.createCalls) != 0 {
		t.Errorf("CreateRepo should not have been called, got calls = %v", td.GitHub.createCalls)
	}
}

func TestCreate_APIFailure(t *testing.T) {
	td := newTestDeps(t)
	td.GitHub.createErr = errNotFoundForTest{}

	err := runCLI(t, td.Deps, "create", "my-project")
	if err == nil {
		t.Fatal("create: expected an error when CreateRepo fails")
	}
	if !strings.Contains(err.Error(), "could not create") {
		t.Errorf("error = %q, want a create-failure message", err.Error())
	}
}

func TestCreate_DestinationAlreadyExists(t *testing.T) {
	td := newTestDeps(t)
	td.GitHub.createResult = ghclient.RepoResult{Owner: "learner", Repo: "my-project", CloneURL: "https://github.com/learner/my-project.git"}
	if err := os.MkdirAll(filepath.Join(td.Cwd, "my-project"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	err := runCLI(t, td.Deps, "create", "my-project")
	if err == nil {
		t.Fatal("create: expected an error when the destination directory already exists")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %q, want an already-exists message", err.Error())
	}
	for _, c := range td.Runner.Calls {
		if len(c.Args) > 0 && c.Args[0] == "init" {
			t.Errorf("init should not have been attempted, got call: %v", c.Args)
		}
	}
}
