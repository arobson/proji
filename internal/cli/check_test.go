package cli_test

import (
	"strings"
	"testing"

	"github.com/arobson/proji/internal/git"
	"github.com/arobson/proji/internal/git/gittest"
)

func TestCheckUpstream_NotAGitRepo(t *testing.T) {
	td := newTestDeps(t)
	td.Runner.Responses = []*gittest.Response{
		{Match: gittest.MatchPrefix("rev-parse", "--is-inside-work-tree"), Stdout: "false\n"},
	}

	err := runCLI(t, td.Deps, "check", "upstream")
	if err == nil {
		t.Fatal("check upstream: expected an error outside a git repo")
	}
	if !strings.Contains(err.Error(), "git repository") {
		t.Errorf("error = %q, want a not-a-git-repo message", err.Error())
	}
}

func TestCheckUpstream_NoUpstreamRemote(t *testing.T) {
	td := newTestDeps(t)
	td.Runner.Responses = []*gittest.Response{
		{Match: gittest.MatchPrefix("rev-parse", "--is-inside-work-tree"), Stdout: "true\n"},
		{
			Match:  gittest.MatchPrefix("remote", "get-url"),
			Stderr: "error: No such remote 'upstream'",
			Err:    &git.ExitError{ExitCode: 2, Stderr: "error: No such remote 'upstream'"},
		},
	}

	err := runCLI(t, td.Deps, "check", "upstream")
	if err == nil {
		t.Fatal("check upstream: expected an error with no upstream remote")
	}
	if !strings.Contains(err.Error(), `"upstream"`) {
		t.Errorf("error = %q, want it to mention the upstream remote", err.Error())
	}
}

func TestCheckUpstream_FastForward(t *testing.T) {
	td := newTestDeps(t)
	td.Runner.Responses = []*gittest.Response{
		{Match: gittest.MatchPrefix("rev-parse", "--is-inside-work-tree"), Stdout: "true\n"},
		{Match: gittest.MatchPrefix("remote", "get-url"), Stdout: "https://github.com/instructor/hw.git\n"},
		{Match: gittest.MatchPrefix("rev-parse", "--abbrev-ref", "HEAD"), Stdout: "main\n"},
		{Match: gittest.MatchPrefix("fetch", "upstream", "main")},
		{Match: gittest.MatchPrefix("rev-list"), Stdout: "3\n"},
		{Match: gittest.MatchPrefix("merge", "--ff-only")},
	}

	if err := runCLI(t, td.Deps, "check", "upstream"); err != nil {
		t.Fatalf("check upstream: unexpected error: %v", err)
	}
	if !strings.Contains(td.Out.String(), "Fast-forwarded to the latest changes from upstream.") {
		t.Errorf("output = %q, want a fast-forward confirmation", td.Out.String())
	}
}

func TestCheckUpstream_AlreadyUpToDate(t *testing.T) {
	td := newTestDeps(t)
	td.Runner.Responses = []*gittest.Response{
		{Match: gittest.MatchPrefix("rev-parse", "--is-inside-work-tree"), Stdout: "true\n"},
		{Match: gittest.MatchPrefix("remote", "get-url"), Stdout: "https://github.com/instructor/hw.git\n"},
		{Match: gittest.MatchPrefix("rev-parse", "--abbrev-ref", "HEAD"), Stdout: "main\n"},
		{Match: gittest.MatchPrefix("fetch", "upstream", "main")},
		{Match: gittest.MatchPrefix("rev-list"), Stdout: "0\n"},
	}

	if err := runCLI(t, td.Deps, "check", "upstream"); err != nil {
		t.Fatalf("check upstream: unexpected error: %v", err)
	}
	if !strings.Contains(td.Out.String(), "Already up to date with upstream.") {
		t.Errorf("output = %q, want an up-to-date confirmation", td.Out.String())
	}
}
