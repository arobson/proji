package cli_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/arobson/proji/internal/git"
	"github.com/arobson/proji/internal/git/gittest"
	"github.com/arobson/proji/internal/prompt/prompttest"
)

func TestCheckin_NothingToCheckIn(t *testing.T) {
	td := newTestDeps(t)
	td.Runner.Responses = []*gittest.Response{
		{Match: gittest.MatchPrefix("rev-parse", "--is-inside-work-tree"), Stdout: "true\n"},
		{Match: gittest.MatchPrefix("status", "--porcelain"), Stdout: ""},
		{Match: gittest.MatchPrefix("rev-list"), Stdout: "0\n"},
	}

	if err := runCLI(t, td.Deps, "checkin"); err != nil {
		t.Fatalf("checkin: unexpected error: %v", err)
	}
	if !strings.Contains(td.Out.String(), "Nothing to check in") {
		t.Errorf("output = %q, want a nothing-to-check-in message", td.Out.String())
	}
	for _, c := range td.Runner.Calls {
		if len(c.Args) > 0 && (c.Args[0] == "commit" || c.Args[0] == "push") {
			t.Errorf("should not have committed/pushed, got call: %v", c.Args)
		}
	}
}

func TestCheckin_DefaultMessageUsesDirNameAndDate(t *testing.T) {
	td := newTestDeps(t)
	td.Prompt = &prompttest.Fake{Answers: []string{""}}
	td.Deps.Prompt = td.Prompt
	td.Runner.Responses = []*gittest.Response{
		{Match: gittest.MatchPrefix("rev-parse", "--is-inside-work-tree"), Stdout: "true\n"},
		{Match: gittest.MatchPrefix("status", "--porcelain"), Stdout: " M file.txt\n"},
		{Match: gittest.MatchPrefix("rev-list"), Err: &git.ExitError{ExitCode: 128, Stderr: "unknown revision"}},
		{Match: gittest.MatchPrefix("add", "-A")},
		{Match: gittest.MatchPrefix("commit", "-m")},
		{Match: gittest.MatchPrefix("push", "-u")},
	}

	if err := runCLI(t, td.Deps, "checkin"); err != nil {
		t.Fatalf("checkin: unexpected error: %v", err)
	}

	dirname := filepath.Base(td.Cwd)
	wantMessage := dirname + ": 2026-03-04 15:05"

	var commitArgs []string
	for _, c := range td.Runner.Calls {
		if len(c.Args) > 0 && c.Args[0] == "commit" {
			commitArgs = c.Args
		}
	}
	if len(commitArgs) != 3 || commitArgs[2] != wantMessage {
		t.Errorf("commit args = %v, want message %q", commitArgs, wantMessage)
	}
	if !strings.Contains(td.Out.String(), wantMessage) {
		t.Errorf("output = %q, want it to include %q", td.Out.String(), wantMessage)
	}
}

func TestCheckin_CustomMessage(t *testing.T) {
	td := newTestDeps(t)
	td.Prompt = &prompttest.Fake{Answers: []string{"finished the loop exercise"}}
	td.Deps.Prompt = td.Prompt
	td.Runner.Responses = []*gittest.Response{
		{Match: gittest.MatchPrefix("rev-parse", "--is-inside-work-tree"), Stdout: "true\n"},
		{Match: gittest.MatchPrefix("status", "--porcelain"), Stdout: " M file.txt\n"},
		{Match: gittest.MatchPrefix("rev-list"), Err: &git.ExitError{ExitCode: 128}},
		{Match: gittest.MatchPrefix("add", "-A")},
		{Match: gittest.MatchPrefix("commit", "-m")},
		{Match: gittest.MatchPrefix("push", "-u")},
	}

	if err := runCLI(t, td.Deps, "checkin"); err != nil {
		t.Fatalf("checkin: unexpected error: %v", err)
	}

	dirname := filepath.Base(td.Cwd)
	wantMessage := dirname + ": finished the loop exercise"

	var commitArgs []string
	for _, c := range td.Runner.Calls {
		if len(c.Args) > 0 && c.Args[0] == "commit" {
			commitArgs = c.Args
		}
	}
	if len(commitArgs) != 3 || commitArgs[2] != wantMessage {
		t.Errorf("commit args = %v, want message %q", commitArgs, wantMessage)
	}
}

func TestCheckin_PushRejectedNonFastForward(t *testing.T) {
	td := newTestDeps(t)
	td.Prompt = &prompttest.Fake{Answers: []string{"msg"}}
	td.Deps.Prompt = td.Prompt
	td.Runner.Responses = []*gittest.Response{
		{Match: gittest.MatchPrefix("rev-parse", "--is-inside-work-tree"), Stdout: "true\n"},
		{Match: gittest.MatchPrefix("status", "--porcelain"), Stdout: " M file.txt\n"},
		{Match: gittest.MatchPrefix("rev-list"), Err: &git.ExitError{ExitCode: 128}},
		{Match: gittest.MatchPrefix("add", "-A")},
		{Match: gittest.MatchPrefix("commit", "-m")},
		{
			Match: gittest.MatchPrefix("push", "-u"),
			Err:   &git.ExitError{ExitCode: 1, Stderr: "! [rejected]  main -> main (non-fast-forward)"},
		},
	}

	err := runCLI(t, td.Deps, "checkin")
	if err == nil {
		t.Fatal("checkin: expected an error when push is rejected")
	}
	if !strings.Contains(err.Error(), "proji checkout") {
		t.Errorf("error = %q, want it to suggest running proji checkout", err.Error())
	}
}

func TestCheckin_NothingToCommitAfterStaging(t *testing.T) {
	td := newTestDeps(t)
	td.Prompt = &prompttest.Fake{Answers: []string{""}}
	td.Deps.Prompt = td.Prompt
	td.Runner.Responses = []*gittest.Response{
		{Match: gittest.MatchPrefix("rev-parse", "--is-inside-work-tree"), Stdout: "true\n"},
		{Match: gittest.MatchPrefix("status", "--porcelain"), Stdout: " M file.txt\n"},
		{Match: gittest.MatchPrefix("rev-list"), Err: &git.ExitError{ExitCode: 128}},
		{Match: gittest.MatchPrefix("add", "-A")},
		{
			Match:  gittest.MatchPrefix("commit", "-m"),
			Stderr: "nothing to commit, working tree clean",
			Err:    &git.ExitError{ExitCode: 1, Stderr: "nothing to commit, working tree clean"},
		},
	}

	if err := runCLI(t, td.Deps, "checkin"); err != nil {
		t.Fatalf("checkin: unexpected error: %v", err)
	}
	if !strings.Contains(td.Out.String(), "nothing to commit") {
		t.Errorf("output = %q, want a nothing-to-commit message", td.Out.String())
	}
}
