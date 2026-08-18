package cli_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/arobson/proji/internal/cli"
	"github.com/arobson/proji/internal/config"
	"github.com/arobson/proji/internal/ghclient"
	"github.com/arobson/proji/internal/git"
	"github.com/arobson/proji/internal/git/gittest"
	"github.com/arobson/proji/internal/prompt/prompttest"
)

// fakeGitHubAPI is a hand-written cli.GitHubAPI test double. The HTTP layer
// itself is already covered by internal/ghclient's own tests, so command
// tests only need to verify orchestration.
type fakeGitHubAPI struct {
	forkResult ghclient.RepoResult
	forkErr    error
	forkCalls  []string // "owner/repo" for each ForkRepo call

	createResult ghclient.RepoResult
	createErr    error
	createCalls  []string // repo name for each CreateRepo call

	getRepoResult ghclient.RepoResult
	getRepoErr    error    // defaults to ghclient.ErrRepoNotFound if zero-value; set explicitly to override
	getRepoCalls  []string // "owner/name" for each GetRepo call

	currentUser    string
	currentUserErr error

	addSSHKeyErr   error
	addSSHKeyCalls []string // title for each AddSSHKey call
}

func (f *fakeGitHubAPI) ForkRepo(_ context.Context, owner, repo string) (ghclient.RepoResult, error) {
	f.forkCalls = append(f.forkCalls, owner+"/"+repo)
	return f.forkResult, f.forkErr
}

func (f *fakeGitHubAPI) CreateRepo(_ context.Context, name string, _ bool) (ghclient.RepoResult, error) {
	f.createCalls = append(f.createCalls, name)
	return f.createResult, f.createErr
}

func (f *fakeGitHubAPI) GetRepo(_ context.Context, owner, name string) (ghclient.RepoResult, error) {
	f.getRepoCalls = append(f.getRepoCalls, owner+"/"+name)
	if f.getRepoErr != nil {
		return ghclient.RepoResult{}, f.getRepoErr
	}
	if f.getRepoResult == (ghclient.RepoResult{}) {
		return ghclient.RepoResult{}, ghclient.ErrRepoNotFound
	}
	return f.getRepoResult, nil
}

func (f *fakeGitHubAPI) CurrentUser(_ context.Context) (string, error) {
	return f.currentUser, f.currentUserErr
}

func (f *fakeGitHubAPI) AddSSHKey(_ context.Context, title, _ string) error {
	f.addSSHKeyCalls = append(f.addSSHKeyCalls, title)
	return f.addSSHKeyErr
}

// testDeps bundles everything a command test needs: a *cli.Deps wired to
// fakes, plus the fakes themselves for assertions.
type testDeps struct {
	Deps    *cli.Deps
	Out     *stringWriter
	Runner  *gittest.FakeRunner
	GitHub  *fakeGitHubAPI
	Prompt  *prompttest.Fake
	Cwd     string
	NowTime time.Time
}

func newTestDeps(t *testing.T) *testDeps {
	t.Helper()

	cwd := t.TempDir()
	runner := &gittest.FakeRunner{}
	gh := &fakeGitHubAPI{currentUser: "learner"}
	p := &prompttest.Fake{}
	out := &stringWriter{}
	now := time.Date(2026, 3, 4, 15, 5, 0, 0, time.UTC)

	store := config.NewStore(filepath.Join(t.TempDir(), "creds.yml"))
	if err := store.Save(config.Creds{Token: "stored-token", TokenType: "pat"}); err != nil {
		t.Fatalf("seed creds store: %v", err)
	}

	deps := &cli.Deps{
		Out:        out,
		ErrOut:     out,
		In:         strings.NewReader(""),
		CredsStore: store,
		Prompt:     p,
		NewGitHubClient: func(string) cli.GitHubAPI {
			return gh
		},
		NewRunner: func() git.Runner {
			return runner
		},
		// git is always "found" by default so existing command tests never
		// trigger the bootstrap flow; tests that want to exercise it
		// override LookPath and NewBootstrapper themselves.
		LookPath: func(string) (string, error) { return "/usr/bin/git", nil },
		NewBootstrapper: func() cli.GitBootstrapper {
			return &stubBootstrapper{runErr: errors.New("bootstrap should not run when git is already found")}
		},
		NewUpgrader: func() cli.Upgrader {
			return &stubUpgrader{}
		},
		Now:     func() time.Time { return now },
		Getwd:   func() (string, error) { return cwd, nil },
		Version: "proji-vtest",
	}

	return &testDeps{Deps: deps, Out: out, Runner: runner, GitHub: gh, Prompt: p, Cwd: cwd, NowTime: now}
}

// stubBootstrapper is a minimal cli.GitBootstrapper test double, tracking
// whether each of its three methods was invoked.
type stubBootstrapper struct {
	runErr, installErr, configureErr          error
	runCalled, installCalled, configureCalled bool
}

func (s *stubBootstrapper) Run(context.Context) error {
	s.runCalled = true
	return s.runErr
}

func (s *stubBootstrapper) InstallGit(context.Context) error {
	s.installCalled = true
	return s.installErr
}

func (s *stubBootstrapper) ConfigureGit(context.Context) error {
	s.configureCalled = true
	return s.configureErr
}

// stubUpgrader is a minimal cli.Upgrader test double.
type stubUpgrader struct {
	err    error
	called bool
}

func (s *stubUpgrader) Run(context.Context) error {
	s.called = true
	return s.err
}

// stringWriter is a minimal, allocation-friendly io.Writer for capturing
// command output in tests.
type stringWriter struct {
	buf []byte
}

func (w *stringWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	return len(p), nil
}

func (w *stringWriter) String() string {
	return string(w.buf)
}
