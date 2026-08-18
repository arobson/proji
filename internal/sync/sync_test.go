package sync_test

import (
	"context"
	"testing"

	"github.com/arobson/proji/internal/git"
	"github.com/arobson/proji/internal/git/gittest"
	"github.com/arobson/proji/internal/prompt/prompttest"
	"github.com/arobson/proji/internal/sync"
)

func TestSyncer_Sync_AlreadyUpToDate(t *testing.T) {
	fr := &gittest.FakeRunner{Responses: []*gittest.Response{
		{Match: gittest.MatchPrefix("fetch")},
		{Match: gittest.MatchPrefix("rev-list"), Stdout: "0\n"},
	}}
	syncer := &sync.Syncer{Repo: git.NewRepo(fr, "/work"), Prompt: &prompttest.Fake{}}

	result, err := syncer.Sync(context.Background(), "upstream", "main")
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if result.Action != sync.ActionUpToDate {
		t.Errorf("Action = %q, want %q", result.Action, sync.ActionUpToDate)
	}
	assertNoRebaseOrMerge(t, fr)
}

func TestSyncer_Sync_FastForwards(t *testing.T) {
	fr := &gittest.FakeRunner{Responses: []*gittest.Response{
		{Match: gittest.MatchPrefix("fetch")},
		{Match: gittest.MatchPrefix("rev-list"), Stdout: "2\n"},
		{Match: gittest.MatchPrefix("merge", "--ff-only")},
	}}
	syncer := &sync.Syncer{Repo: git.NewRepo(fr, "/work"), Prompt: &prompttest.Fake{}}

	result, err := syncer.Sync(context.Background(), "upstream", "main")
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if result.Action != sync.ActionFastForward {
		t.Errorf("Action = %q, want %q", result.Action, sync.ActionFastForward)
	}
	assertNoRebaseOrMerge(t, fr)
}

func TestSyncer_Sync_FallsBackToRebase(t *testing.T) {
	fr := &gittest.FakeRunner{Responses: []*gittest.Response{
		{Match: gittest.MatchPrefix("fetch")},
		{Match: gittest.MatchPrefix("rev-list"), Stdout: "2\n"},
		{Match: gittest.MatchPrefix("merge", "--ff-only"), Err: &git.ExitError{ExitCode: 128}},
		{Match: gittest.MatchPrefix("rebase"), Stderr: ""},
	}}
	syncer := &sync.Syncer{Repo: git.NewRepo(fr, "/work"), Prompt: &prompttest.Fake{}}

	result, err := syncer.Sync(context.Background(), "upstream", "main")
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if result.Action != sync.ActionRebase {
		t.Errorf("Action = %q, want %q", result.Action, sync.ActionRebase)
	}
	if calledPlainMerge(fr) {
		t.Error("expected no plain merge call when rebase succeeds")
	}
}

func TestSyncer_Sync_ConflictThenDeclineMerge(t *testing.T) {
	fr := &gittest.FakeRunner{Responses: []*gittest.Response{
		{Match: gittest.MatchPrefix("fetch")},
		{Match: gittest.MatchPrefix("rev-list"), Stdout: "2\n"},
		{Match: gittest.MatchPrefix("merge", "--ff-only"), Err: &git.ExitError{ExitCode: 128}},
		{Match: matchRebaseNotAbort, Err: &git.ExitError{ExitCode: 1, Stderr: "CONFLICT"}},
		{Match: gittest.MatchPrefix("rebase", "--abort")},
	}}
	fake := &prompttest.Fake{Confirms: []bool{false}}
	syncer := &sync.Syncer{Repo: git.NewRepo(fr, "/work"), Prompt: fake}

	result, err := syncer.Sync(context.Background(), "upstream", "main")
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if result.Action != sync.ActionAborted {
		t.Errorf("Action = %q, want %q", result.Action, sync.ActionAborted)
	}
	if len(fake.Prompts) != 1 {
		t.Fatalf("expected exactly one prompt, got %d", len(fake.Prompts))
	}
	assertAbortBeforePrompt(t, fr)
}

func TestSyncer_Sync_ConflictThenAcceptCleanMerge(t *testing.T) {
	fr := &gittest.FakeRunner{Responses: []*gittest.Response{
		{Match: gittest.MatchPrefix("fetch")},
		{Match: gittest.MatchPrefix("rev-list"), Stdout: "2\n"},
		{Match: gittest.MatchPrefix("merge", "--ff-only"), Err: &git.ExitError{ExitCode: 128}},
		{Match: matchRebaseNotAbort, Err: &git.ExitError{ExitCode: 1, Stderr: "CONFLICT"}},
		{Match: gittest.MatchPrefix("rebase", "--abort")},
		{Match: matchPlainMerge},
		{Match: gittest.MatchPrefix("diff", "--name-only", "--diff-filter=U"), Stdout: ""},
	}}
	fake := &prompttest.Fake{Confirms: []bool{true}}
	syncer := &sync.Syncer{Repo: git.NewRepo(fr, "/work"), Prompt: fake}

	result, err := syncer.Sync(context.Background(), "upstream", "main")
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if result.Action != sync.ActionMerge {
		t.Errorf("Action = %q, want %q", result.Action, sync.ActionMerge)
	}
}

func TestSyncer_Sync_ConflictThenAcceptUncleanMerge(t *testing.T) {
	fr := &gittest.FakeRunner{Responses: []*gittest.Response{
		{Match: gittest.MatchPrefix("fetch")},
		{Match: gittest.MatchPrefix("rev-list"), Stdout: "2\n"},
		{Match: gittest.MatchPrefix("merge", "--ff-only"), Err: &git.ExitError{ExitCode: 128}},
		{Match: matchRebaseNotAbort, Err: &git.ExitError{ExitCode: 1, Stderr: "CONFLICT"}},
		{Match: gittest.MatchPrefix("rebase", "--abort")},
		{Match: matchPlainMerge},
		{Match: gittest.MatchPrefix("diff", "--name-only", "--diff-filter=U"), Stdout: "conflicted.txt\n"},
	}}
	fake := &prompttest.Fake{Confirms: []bool{true}}
	syncer := &sync.Syncer{Repo: git.NewRepo(fr, "/work"), Prompt: fake}

	result, err := syncer.Sync(context.Background(), "upstream", "main")
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if result.Action != sync.ActionMergeUnclean {
		t.Errorf("Action = %q, want %q", result.Action, sync.ActionMergeUnclean)
	}
}

func matchRebaseNotAbort(args []string) bool {
	return len(args) > 0 && args[0] == "rebase" && !(len(args) > 1 && args[1] == "--abort")
}

func matchPlainMerge(args []string) bool {
	return len(args) == 2 && args[0] == "merge" && args[1] == "FETCH_HEAD"
}

func calledPlainMerge(fr *gittest.FakeRunner) bool {
	for _, c := range fr.Calls {
		if matchPlainMerge(c.Args) {
			return true
		}
	}
	return false
}

func assertNoRebaseOrMerge(t *testing.T, fr *gittest.FakeRunner) {
	t.Helper()
	for _, c := range fr.Calls {
		if len(c.Args) > 0 && (c.Args[0] == "rebase" || matchPlainMerge(c.Args)) {
			t.Errorf("unexpected call: git %v", c.Args)
		}
	}
}

func assertAbortBeforePrompt(t *testing.T, fr *gittest.FakeRunner) {
	t.Helper()
	for _, c := range fr.Calls {
		if len(c.Args) >= 2 && c.Args[0] == "rebase" && c.Args[1] == "--abort" {
			return
		}
	}
	t.Error("expected rebase --abort to have been called")
}
