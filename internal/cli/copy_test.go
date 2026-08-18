package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arobson/proji/internal/cli"
	"github.com/arobson/proji/internal/ghclient"
	"github.com/arobson/proji/internal/git/gittest"
)

func runCLI(t *testing.T, deps *cli.Deps, args ...string) error {
	t.Helper()
	cmd := cli.NewRootCmd(deps)
	cmd.SetArgs(args)
	return cmd.Execute()
}

func TestCopy_Success(t *testing.T) {
	td := newTestDeps(t)
	td.GitHub.forkResult = ghclient.RepoResult{
		Owner:    "learner",
		Repo:     "homework-1",
		HTMLURL:  "https://github.com/learner/homework-1",
		CloneURL: "https://github.com/learner/homework-1.git",
	}
	td.Runner.Responses = []*gittest.Response{
		{Match: gittest.MatchPrefix("clone")},
		{Match: gittest.MatchPrefix("remote", "add")},
	}

	if err := runCLI(t, td.Deps, "copy", "instructor/homework-1"); err != nil {
		t.Fatalf("copy: unexpected error: %v", err)
	}

	if len(td.GitHub.forkCalls) != 1 || td.GitHub.forkCalls[0] != "instructor/homework-1" {
		t.Errorf("ForkRepo calls = %v, want [instructor/homework-1]", td.GitHub.forkCalls)
	}

	destDir := filepath.Join(td.Cwd, "homework-1")
	out := td.Out.String()
	if !strings.Contains(out, "Forked instructor/homework-1 to learner/homework-1") {
		t.Errorf("output missing fork confirmation: %q", out)
	}
	if !strings.Contains(out, "Copied your repository to "+destDir) {
		t.Errorf("output missing copy confirmation: %q", out)
	}
	if !strings.Contains(out, "cd "+destDir) {
		t.Errorf("output missing cd instruction: %q", out)
	}

	var cloneArgs, remoteArgs []string
	for _, c := range td.Runner.Calls {
		if len(c.Args) > 0 && c.Args[0] == "clone" {
			cloneArgs = c.Args
		}
		if len(c.Args) > 1 && c.Args[0] == "remote" && c.Args[1] == "add" {
			remoteArgs = c.Args
		}
	}
	if len(cloneArgs) != 3 || cloneArgs[2] != destDir {
		t.Errorf("clone args = %v, want destDir %q as last arg", cloneArgs, destDir)
	}
	if !strings.Contains(cloneArgs[1], "@github.com/learner/homework-1.git") {
		t.Errorf("clone URL = %q, want the fork's authenticated URL", cloneArgs[1])
	}
	wantRemote := []string{"remote", "add", "upstream", "https://github.com/instructor/homework-1.git"}
	if len(remoteArgs) != 4 || remoteArgs[3] != wantRemote[3] {
		t.Errorf("remote add args = %v, want %v", remoteArgs, wantRemote)
	}
}

func TestCopy_InvalidArgFormat(t *testing.T) {
	td := newTestDeps(t)

	err := runCLI(t, td.Deps, "copy", "not-a-valid-arg")
	if err == nil {
		t.Fatal("copy: expected an error for a malformed owner/repo argument")
	}
	if !strings.Contains(err.Error(), "owner/repository") {
		t.Errorf("error = %q, want a hint about the owner/repository format", err.Error())
	}
	if len(td.GitHub.forkCalls) != 0 {
		t.Errorf("ForkRepo should not have been called, got calls = %v", td.GitHub.forkCalls)
	}
}

func TestCopy_ForkFailure(t *testing.T) {
	td := newTestDeps(t)
	td.GitHub.forkErr = errNotFoundForTest{}

	err := runCLI(t, td.Deps, "copy", "instructor/does-not-exist")
	if err == nil {
		t.Fatal("copy: expected an error when ForkRepo fails")
	}
	if !strings.Contains(err.Error(), "could not fork") {
		t.Errorf("error = %q, want a fork-failure message", err.Error())
	}
}

func TestCopy_DestinationAlreadyExists(t *testing.T) {
	td := newTestDeps(t)
	td.GitHub.forkResult = ghclient.RepoResult{Owner: "learner", Repo: "homework-1", CloneURL: "https://github.com/learner/homework-1.git"}
	if err := os.MkdirAll(filepath.Join(td.Cwd, "homework-1"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	err := runCLI(t, td.Deps, "copy", "instructor/homework-1")
	if err == nil {
		t.Fatal("copy: expected an error when the destination directory already exists")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %q, want an already-exists message", err.Error())
	}
	for _, c := range td.Runner.Calls {
		if len(c.Args) > 0 && c.Args[0] == "clone" {
			t.Errorf("clone should not have been attempted, got call: %v", c.Args)
		}
	}
}

type errNotFoundForTest struct{}

func (errNotFoundForTest) Error() string { return "404 Not Found" }
