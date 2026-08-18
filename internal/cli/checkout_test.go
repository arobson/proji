package cli_test

import (
	"strings"
	"testing"

	"github.com/arobson/proji/internal/git"
	"github.com/arobson/proji/internal/git/gittest"
	"github.com/arobson/proji/internal/prompt/prompttest"
)

func TestCheckout_NoOriginRemote(t *testing.T) {
	td := newTestDeps(t)
	td.Runner.Responses = []*gittest.Response{
		{Match: gittest.MatchPrefix("rev-parse", "--is-inside-work-tree"), Stdout: "true\n"},
		{
			Match:  gittest.MatchPrefix("remote", "get-url"),
			Stderr: "error: No such remote 'origin'",
			Err:    &git.ExitError{ExitCode: 2, Stderr: "error: No such remote 'origin'"},
		},
	}

	err := runCLI(t, td.Deps, "checkout")
	if err == nil {
		t.Fatal("checkout: expected an error with no origin remote")
	}
	if !strings.Contains(err.Error(), `"origin"`) {
		t.Errorf("error = %q, want it to mention the origin remote", err.Error())
	}
}

func TestCheckout_ConflictThenDeclineMergeIsNotAnError(t *testing.T) {
	td := newTestDeps(t)
	td.Prompt = &prompttest.Fake{Confirms: []bool{false}}
	td.Deps.Prompt = td.Prompt
	td.Runner.Responses = []*gittest.Response{
		{Match: gittest.MatchPrefix("rev-parse", "--is-inside-work-tree"), Stdout: "true\n"},
		{Match: gittest.MatchPrefix("remote", "get-url"), Stdout: "https://github.com/learner/hw.git\n"},
		{Match: gittest.MatchPrefix("rev-parse", "--abbrev-ref", "HEAD"), Stdout: "main\n"},
		{Match: gittest.MatchPrefix("fetch", "origin", "main")},
		{Match: gittest.MatchPrefix("rev-list"), Stdout: "2\n"},
		{Match: gittest.MatchPrefix("merge", "--ff-only"), Err: &git.ExitError{ExitCode: 128}},
		{Match: func(a []string) bool { return len(a) == 2 && a[0] == "rebase" && a[1] == "FETCH_HEAD" }, Err: &git.ExitError{ExitCode: 1, Stderr: "CONFLICT"}},
		{Match: gittest.MatchPrefix("rebase", "--abort")},
	}

	if err := runCLI(t, td.Deps, "checkout"); err != nil {
		t.Fatalf("checkout: unexpected error: %v", err)
	}
	if !strings.Contains(td.Out.String(), "No changes were applied") {
		t.Errorf("output = %q, want an aborted-rebase message", td.Out.String())
	}
}
