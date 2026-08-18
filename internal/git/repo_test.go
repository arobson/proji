package git_test

import (
	"context"
	"errors"
	"testing"

	"github.com/arobson/proji/internal/git"
	"github.com/arobson/proji/internal/git/gittest"
)

func TestRepo_IsGitRepo(t *testing.T) {
	tests := []struct {
		name   string
		stdout string
		err    error
		want   bool
	}{
		{"inside work tree", "true\n", nil, true},
		{"outside work tree", "false\n", nil, false},
		{"command failed", "", &git.ExitError{ExitCode: 128}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fr := &gittest.FakeRunner{Responses: []*gittest.Response{
				{Match: gittest.MatchPrefix("rev-parse", "--is-inside-work-tree"), Stdout: tt.stdout, Err: tt.err},
			}}
			repo := git.NewRepo(fr, "/work")
			if got := repo.IsGitRepo(context.Background()); got != tt.want {
				t.Errorf("IsGitRepo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRepo_Clone_PassesURLAndDest(t *testing.T) {
	fr := &gittest.FakeRunner{Responses: []*gittest.Response{
		{Match: gittest.MatchPrefix("clone"), Stdout: ""},
	}}
	repo := git.NewRepo(fr, "/work")

	if err := repo.Clone(context.Background(), "https://example.com/a/b.git", "/work/b"); err != nil {
		t.Fatalf("Clone() error = %v", err)
	}
	wantArgs := []string{"clone", "https://example.com/a/b.git", "/work/b"}
	assertLastCallArgs(t, fr, wantArgs)
}

func TestRepo_Init(t *testing.T) {
	fr := &gittest.FakeRunner{Responses: []*gittest.Response{{Match: gittest.MatchPrefix("init", "-b")}}}
	repo := git.NewRepo(fr, "/work")
	if err := repo.Init(context.Background(), "main"); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	assertLastCallArgs(t, fr, []string{"init", "-b", "main"})
}

func TestRepo_AddRemote(t *testing.T) {
	fr := &gittest.FakeRunner{Responses: []*gittest.Response{
		{Match: gittest.MatchPrefix("remote", "add")},
	}}
	repo := git.NewRepo(fr, "/work")

	if err := repo.AddRemote(context.Background(), "upstream", "https://example.com/a/b.git"); err != nil {
		t.Fatalf("AddRemote() error = %v", err)
	}
	assertLastCallArgs(t, fr, []string{"remote", "add", "upstream", "https://example.com/a/b.git"})
}

func TestRepo_RemoteExists(t *testing.T) {
	t.Run("exists", func(t *testing.T) {
		fr := &gittest.FakeRunner{Responses: []*gittest.Response{
			{Match: gittest.MatchPrefix("remote", "get-url"), Stdout: "https://example.com/a/b.git\n"},
		}}
		repo := git.NewRepo(fr, "/work")
		ok, err := repo.RemoteExists(context.Background(), "origin")
		if err != nil || !ok {
			t.Fatalf("RemoteExists() = %v, %v, want true, nil", ok, err)
		}
	})
	t.Run("missing", func(t *testing.T) {
		fr := &gittest.FakeRunner{Responses: []*gittest.Response{
			{
				Match:  gittest.MatchPrefix("remote", "get-url"),
				Stderr: "error: No such remote 'origin'",
				Err:    &git.ExitError{ExitCode: 2, Stderr: "error: No such remote 'origin'"},
			},
		}}
		repo := git.NewRepo(fr, "/work")
		ok, err := repo.RemoteExists(context.Background(), "origin")
		if err != nil || ok {
			t.Fatalf("RemoteExists() = %v, %v, want false, nil", ok, err)
		}
	})
	t.Run("other error propagates", func(t *testing.T) {
		wantErr := errors.New("boom")
		fr := &gittest.FakeRunner{Responses: []*gittest.Response{
			{Match: gittest.MatchPrefix("remote", "get-url"), Err: wantErr},
		}}
		repo := git.NewRepo(fr, "/work")
		_, err := repo.RemoteExists(context.Background(), "origin")
		if !errors.Is(err, wantErr) {
			t.Fatalf("RemoteExists() error = %v, want %v", err, wantErr)
		}
	})
}

func TestRepo_CurrentBranch(t *testing.T) {
	fr := &gittest.FakeRunner{Responses: []*gittest.Response{
		{Match: gittest.MatchPrefix("rev-parse", "--abbrev-ref", "HEAD"), Stdout: "main\n"},
	}}
	repo := git.NewRepo(fr, "/work")
	got, err := repo.CurrentBranch(context.Background())
	if err != nil || got != "main" {
		t.Fatalf("CurrentBranch() = %q, %v, want %q, nil", got, err, "main")
	}
}

func TestRepo_Fetch(t *testing.T) {
	fr := &gittest.FakeRunner{Responses: []*gittest.Response{{Match: gittest.MatchPrefix("fetch")}}}
	repo := git.NewRepo(fr, "/work")
	if err := repo.Fetch(context.Background(), "upstream", "main"); err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	assertLastCallArgs(t, fr, []string{"fetch", "upstream", "main"})
}

func TestRepo_MergeFFOnly(t *testing.T) {
	fr := &gittest.FakeRunner{Responses: []*gittest.Response{{Match: gittest.MatchPrefix("merge", "--ff-only")}}}
	repo := git.NewRepo(fr, "/work")
	if err := repo.MergeFFOnly(context.Background(), "FETCH_HEAD"); err != nil {
		t.Fatalf("MergeFFOnly() error = %v", err)
	}
	assertLastCallArgs(t, fr, []string{"merge", "--ff-only", "FETCH_HEAD"})
}

func TestRepo_Rebase(t *testing.T) {
	fr := &gittest.FakeRunner{Responses: []*gittest.Response{{Match: gittest.MatchPrefix("rebase")}}}
	repo := git.NewRepo(fr, "/work")
	if err := repo.Rebase(context.Background(), "FETCH_HEAD"); err != nil {
		t.Fatalf("Rebase() error = %v", err)
	}
	assertLastCallArgs(t, fr, []string{"rebase", "FETCH_HEAD"})
}

func TestRepo_RebaseAbort(t *testing.T) {
	fr := &gittest.FakeRunner{Responses: []*gittest.Response{{Match: gittest.MatchPrefix("rebase", "--abort")}}}
	repo := git.NewRepo(fr, "/work")
	if err := repo.RebaseAbort(context.Background()); err != nil {
		t.Fatalf("RebaseAbort() error = %v", err)
	}
	assertLastCallArgs(t, fr, []string{"rebase", "--abort"})
}

func TestRepo_Merge(t *testing.T) {
	fr := &gittest.FakeRunner{Responses: []*gittest.Response{{Match: gittest.MatchPrefix("merge")}}}
	repo := git.NewRepo(fr, "/work")
	if err := repo.Merge(context.Background(), "FETCH_HEAD"); err != nil {
		t.Fatalf("Merge() error = %v", err)
	}
	assertLastCallArgs(t, fr, []string{"merge", "FETCH_HEAD"})
}

func TestRepo_HasConflicts(t *testing.T) {
	t.Run("has conflicts", func(t *testing.T) {
		fr := &gittest.FakeRunner{Responses: []*gittest.Response{
			{Match: gittest.MatchPrefix("diff", "--name-only", "--diff-filter=U"), Stdout: "a.txt\nb.txt\n"},
		}}
		repo := git.NewRepo(fr, "/work")
		got, err := repo.HasConflicts(context.Background())
		if err != nil || !got {
			t.Fatalf("HasConflicts() = %v, %v, want true, nil", got, err)
		}
	})
	t.Run("clean", func(t *testing.T) {
		fr := &gittest.FakeRunner{Responses: []*gittest.Response{
			{Match: gittest.MatchPrefix("diff", "--name-only", "--diff-filter=U"), Stdout: ""},
		}}
		repo := git.NewRepo(fr, "/work")
		got, err := repo.HasConflicts(context.Background())
		if err != nil || got {
			t.Fatalf("HasConflicts() = %v, %v, want false, nil", got, err)
		}
	})
}

func TestRepo_IsUpToDate(t *testing.T) {
	t.Run("up to date", func(t *testing.T) {
		fr := &gittest.FakeRunner{Responses: []*gittest.Response{
			{Match: gittest.MatchPrefix("rev-list", "HEAD..FETCH_HEAD", "--count"), Stdout: "0\n"},
		}}
		repo := git.NewRepo(fr, "/work")
		got, err := repo.IsUpToDate(context.Background(), "FETCH_HEAD")
		if err != nil || !got {
			t.Fatalf("IsUpToDate() = %v, %v, want true, nil", got, err)
		}
	})
	t.Run("behind", func(t *testing.T) {
		fr := &gittest.FakeRunner{Responses: []*gittest.Response{
			{Match: gittest.MatchPrefix("rev-list", "HEAD..FETCH_HEAD", "--count"), Stdout: "3\n"},
		}}
		repo := git.NewRepo(fr, "/work")
		got, err := repo.IsUpToDate(context.Background(), "FETCH_HEAD")
		if err != nil || got {
			t.Fatalf("IsUpToDate() = %v, %v, want false, nil", got, err)
		}
	})
}

func TestRepo_HasChanges(t *testing.T) {
	t.Run("dirty", func(t *testing.T) {
		fr := &gittest.FakeRunner{Responses: []*gittest.Response{
			{Match: gittest.MatchPrefix("status", "--porcelain"), Stdout: " M file.txt\n"},
		}}
		repo := git.NewRepo(fr, "/work")
		got, err := repo.HasChanges(context.Background(), ".")
		if err != nil || !got {
			t.Fatalf("HasChanges() = %v, %v, want true, nil", got, err)
		}
	})
	t.Run("clean", func(t *testing.T) {
		fr := &gittest.FakeRunner{Responses: []*gittest.Response{
			{Match: gittest.MatchPrefix("status", "--porcelain"), Stdout: ""},
		}}
		repo := git.NewRepo(fr, "/work")
		got, err := repo.HasChanges(context.Background(), ".")
		if err != nil || got {
			t.Fatalf("HasChanges() = %v, %v, want false, nil", got, err)
		}
	})
}

func TestRepo_AddAll(t *testing.T) {
	fr := &gittest.FakeRunner{Responses: []*gittest.Response{{Match: gittest.MatchPrefix("add", "-A")}}}
	repo := git.NewRepo(fr, "/work")
	if err := repo.AddAll(context.Background(), "."); err != nil {
		t.Fatalf("AddAll() error = %v", err)
	}
	assertLastCallArgs(t, fr, []string{"add", "-A", "--", "."})
}

func TestRepo_Commit(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		fr := &gittest.FakeRunner{Responses: []*gittest.Response{{Match: gittest.MatchPrefix("commit", "-m")}}}
		repo := git.NewRepo(fr, "/work")
		if err := repo.Commit(context.Background(), "msg"); err != nil {
			t.Fatalf("Commit() error = %v", err)
		}
		assertLastCallArgs(t, fr, []string{"commit", "-m", "msg"})
	})
	t.Run("nothing to commit", func(t *testing.T) {
		fr := &gittest.FakeRunner{Responses: []*gittest.Response{
			{
				Match:  gittest.MatchPrefix("commit", "-m"),
				Stderr: "nothing to commit, working tree clean",
				Err:    &git.ExitError{ExitCode: 1, Stderr: "nothing to commit, working tree clean"},
			},
		}}
		repo := git.NewRepo(fr, "/work")
		err := repo.Commit(context.Background(), "msg")
		if !errors.Is(err, git.ErrNothingToCommit) {
			t.Fatalf("Commit() error = %v, want ErrNothingToCommit", err)
		}
	})
}

func TestRepo_Push(t *testing.T) {
	fr := &gittest.FakeRunner{Responses: []*gittest.Response{{Match: gittest.MatchPrefix("push", "-u")}}}
	repo := git.NewRepo(fr, "/work")
	if err := repo.Push(context.Background(), "origin", "HEAD"); err != nil {
		t.Fatalf("Push() error = %v", err)
	}
	assertLastCallArgs(t, fr, []string{"push", "-u", "origin", "HEAD"})
}

func assertLastCallArgs(t *testing.T, fr *gittest.FakeRunner, want []string) {
	t.Helper()
	if len(fr.Calls) == 0 {
		t.Fatal("no calls recorded")
	}
	got := fr.Calls[len(fr.Calls)-1].Args
	if len(got) != len(want) {
		t.Fatalf("args = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("args = %v, want %v", got, want)
		}
	}
}
