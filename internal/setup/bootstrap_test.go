package setup_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arobson/proji/internal/git"
	"github.com/arobson/proji/internal/git/gittest"
	"github.com/arobson/proji/internal/prompt/prompttest"
	"github.com/arobson/proji/internal/setup"
	"github.com/arobson/proji/internal/setup/setuptest"
)

type fakeRegistrar struct {
	err   error
	calls []struct{ title, key string }
}

func (f *fakeRegistrar) AddSSHKey(_ context.Context, title, key string) error {
	f.calls = append(f.calls, struct{ title, key string }{title, key})
	return f.err
}

func matchConfigGet(key string) func([]string) bool {
	return func(args []string) bool {
		return len(args) == 4 && args[0] == "config" && args[1] == "--global" && args[2] == "--get" && args[3] == key
	}
}

func matchConfigSet(key string) func([]string) bool {
	return func(args []string) bool {
		return len(args) >= 3 && args[0] == "config" && args[1] == "--global" && args[2] == key
	}
}

// unsetConfigResponses scripts "not configured yet" for the three global
// config values ConfigureGit checks, plus a generic success for setting
// each of them. This is the default, first-time-setup script; tests that
// want to exercise the idempotent skip paths override individual entries.
func unsetConfigResponses() []*gittest.Response {
	return []*gittest.Response{
		{Match: matchConfigGet("init.defaultBranch"), Err: &git.ExitError{ExitCode: 1}},
		{Match: matchConfigSet("init.defaultBranch")},
		{Match: matchConfigGet("user.name"), Err: &git.ExitError{ExitCode: 1}},
		{Match: matchConfigSet("user.name")},
		{Match: matchConfigGet("user.email"), Err: &git.ExitError{ExitCode: 1}},
		{Match: matchConfigSet("user.email")},
	}
}

// testBootstrapper bundles a *setup.Bootstrapper wired to fakes, plus the
// fakes themselves for assertions. gitFound reports whether "git" should
// be found on LookPath (simulating a successful install); "brew" is never
// found unless the test overrides LookPath directly.
func testBootstrapper(t *testing.T, goos string, gitFound bool) (b *setup.Bootstrapper, cmdRunner *setuptest.FakeCommandRunner, gitRunner *gittest.FakeRunner, out *strings.Builder, registrar *fakeRegistrar) {
	t.Helper()
	home := t.TempDir()
	cmdRunner = &setuptest.FakeCommandRunner{
		OnCall: func(name string, args []string) {
			if name != "ssh-keygen" {
				return
			}
			for i, a := range args {
				if a == "-f" && i+1 < len(args) {
					if err := os.WriteFile(args[i+1]+".pub", []byte("ecdsa-sha2-nistp256 fake-key\n"), 0o600); err != nil {
						t.Fatalf("fake ssh-keygen: write pub key: %v", err)
					}
				}
			}
		},
	}
	gitRunner = &gittest.FakeRunner{Responses: unsetConfigResponses()}
	p := &prompttest.Fake{Confirms: []bool{true}, Answers: []string{"Ada Lovelace", "ada@example.com"}}
	out = &strings.Builder{}
	registrar = &fakeRegistrar{}

	b = &setup.Bootstrapper{
		GOOS:      goos,
		Prompt:    p,
		Out:       out,
		Runner:    cmdRunner,
		GitRunner: gitRunner,
		LookPath: func(file string) (string, error) {
			if file == "git" && gitFound {
				return "/usr/bin/git", nil
			}
			return "", errors.New("not found")
		},
		ReadOSRelease: func() (map[string]string, error) { return map[string]string{"ID": "ubuntu"}, nil },
		HomeDir:       func() (string, error) { return home, nil },
		Hostname:      func() (string, error) { return "my-laptop", nil },
		Authenticate:  func(context.Context) (setup.SSHKeyRegistrar, error) { return registrar, nil },
	}
	return b, cmdRunner, gitRunner, out, registrar
}

func TestBootstrapper_Run_DeclineInstall(t *testing.T) {
	b, cmdRunner, _, _, _ := testBootstrapper(t, "linux", true)
	b.Prompt = &prompttest.Fake{Confirms: []bool{false}}

	err := b.Run(context.Background())
	if err == nil {
		t.Fatal("Run() error = nil, want an error when the user declines")
	}
	if len(cmdRunner.Calls) != 0 {
		t.Errorf("expected no commands to run, got %v", cmdRunner.Calls)
	}
}

func TestBootstrapper_Run_MacOSWithBrew(t *testing.T) {
	b, cmdRunner, gitRunner, out, registrar := testBootstrapper(t, "darwin", true)
	b.LookPath = func(file string) (string, error) {
		return "/usr/bin/" + file, nil // both "brew" and "git" are found
	}

	if err := b.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	assertCalled(t, cmdRunner, "brew", "install", "git")
	assertCalled(t, cmdRunner, "ssh-keygen", "-t", "ecdsa", "-b", "256", "-N", "", "-f", filepath.Join(mustHome(b), ".ssh", "id_ecdsa"), "-q")
	assertGitCalled(t, gitRunner, "config", "--global", "init.defaultBranch", "main")
	assertGitCalled(t, gitRunner, "config", "--global", "user.name", "Ada Lovelace")
	assertGitCalled(t, gitRunner, "config", "--global", "user.email", "ada@example.com")
	if len(registrar.calls) != 1 || registrar.calls[0].title != "proji on my-laptop" {
		t.Errorf("AddSSHKey calls = %v, want one call titled %q", registrar.calls, "proji on my-laptop")
	}
	if !strings.Contains(out.String(), `Registered your SSH key with GitHub as "proji on my-laptop"`) {
		t.Errorf("output = %q, want a registration confirmation", out.String())
	}
}

func TestBootstrapper_Run_MacOSNoBrewOpensXcodeInstaller(t *testing.T) {
	b, cmdRunner, gitRunner, _, _ := testBootstrapper(t, "darwin", true)
	// LookPath from testBootstrapper only recognizes "git"; "brew" is
	// implicitly "not found", which is what this test wants.

	err := b.Run(context.Background())
	if err == nil {
		t.Fatal("Run() error = nil, want the Xcode installer message")
	}
	if !strings.Contains(err.Error(), "Xcode Command Line Tools") {
		t.Errorf("error = %q, want it to mention the Xcode Command Line Tools installer", err.Error())
	}
	assertCalled(t, cmdRunner, "xcode-select", "--install")
	for _, c := range cmdRunner.Calls {
		if c.Name == "ssh-keygen" {
			t.Errorf("should not have proceeded past the Xcode installer, got call: %v", c)
		}
	}
	if len(gitRunner.Calls) != 0 {
		t.Errorf("should not have proceeded to git configuration, got calls: %v", gitRunner.Calls)
	}
}

func TestBootstrapper_Run_DebianFamily(t *testing.T) {
	b, cmdRunner, _, _, _ := testBootstrapper(t, "linux", true)

	if err := b.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	assertCalled(t, cmdRunner, "sudo", "apt-get", "update")
	assertCalled(t, cmdRunner, "sudo", "apt-get", "install", "-y", "git")

	updateIdx, installIdx := -1, -1
	for i, c := range cmdRunner.Calls {
		if c.Name == "sudo" && len(c.Args) > 0 && c.Args[0] == "apt-get" && c.Args[1] == "update" {
			updateIdx = i
		}
		if c.Name == "sudo" && len(c.Args) > 1 && c.Args[0] == "apt-get" && c.Args[1] == "install" {
			installIdx = i
		}
	}
	if updateIdx == -1 || installIdx == -1 || updateIdx > installIdx {
		t.Errorf("expected apt-get update before install, got calls: %v", cmdRunner.Calls)
	}
}

func TestBootstrapper_Run_UnsupportedPlatform(t *testing.T) {
	b, cmdRunner, _, _, _ := testBootstrapper(t, "linux", true)
	b.ReadOSRelease = func() (map[string]string, error) { return map[string]string{"ID": "fedora"}, nil }

	err := b.Run(context.Background())
	if err == nil {
		t.Fatal("Run() error = nil, want an error for an unsupported platform")
	}
	if len(cmdRunner.Calls) != 0 {
		t.Errorf("expected no commands to run, got %v", cmdRunner.Calls)
	}
}

func TestBootstrapper_Run_StillMissingAfterInstall(t *testing.T) {
	b, _, _, _, _ := testBootstrapper(t, "linux", false)

	err := b.Run(context.Background())
	if err == nil {
		t.Fatal("Run() error = nil, want an error when git is still missing after install")
	}
	if !strings.Contains(err.Error(), "still isn't available") {
		t.Errorf("error = %q, want it to say git is still unavailable", err.Error())
	}
}

func TestBootstrapper_InstallGit_SkippableByCaller(t *testing.T) {
	// InstallGit always runs the install flow when called directly (it
	// assumes the caller already checked git is missing) — this documents
	// that "proji init" is the one responsible for skipping the call
	// entirely when git is already present, not InstallGit itself.
	b, cmdRunner, _, _, _ := testBootstrapper(t, "linux", true)

	if err := b.InstallGit(context.Background()); err != nil {
		t.Fatalf("InstallGit() error = %v", err)
	}
	assertCalled(t, cmdRunner, "sudo", "apt-get", "install", "-y", "git")
}

func TestBootstrapper_ConfigureGit_SkipsAlreadySetDefaultBranch(t *testing.T) {
	b, _, gitRunner, out, _ := testBootstrapper(t, "linux", true)
	gitRunner.Responses = []*gittest.Response{
		{Match: matchConfigGet("init.defaultBranch"), Stdout: "main\n"},
		{Match: matchConfigGet("user.name"), Err: &git.ExitError{ExitCode: 1}},
		{Match: matchConfigSet("user.name")},
		{Match: matchConfigGet("user.email"), Err: &git.ExitError{ExitCode: 1}},
		{Match: matchConfigSet("user.email")},
	}

	if err := b.ConfigureGit(context.Background()); err != nil {
		t.Fatalf("ConfigureGit() error = %v", err)
	}
	if !strings.Contains(out.String(), `Default branch name is already set to "main", skipping.`) {
		t.Errorf("output = %q, want a skip message for the default branch", out.String())
	}
	for _, c := range gitRunner.Calls {
		if matchConfigSet("init.defaultBranch")(c.Args) {
			t.Errorf("should not have set init.defaultBranch again, got call: %v", c.Args)
		}
	}
}

func TestBootstrapper_ConfigureGit_SkipsAlreadySetIdentity(t *testing.T) {
	b, _, gitRunner, out, _ := testBootstrapper(t, "linux", true)
	gitRunner.Responses = []*gittest.Response{
		{Match: matchConfigGet("init.defaultBranch"), Stdout: "main\n"},
		{Match: matchConfigGet("user.name"), Stdout: "Existing Name\n"},
		{Match: matchConfigGet("user.email"), Stdout: "existing@example.com\n"},
	}
	b.Prompt = &prompttest.Fake{} // no answers scripted: a prompt call would error the test

	if err := b.ConfigureGit(context.Background()); err != nil {
		t.Fatalf("ConfigureGit() error = %v", err)
	}
	if !strings.Contains(out.String(), "git name is already set (Existing Name), skipping.") {
		t.Errorf("output = %q, want a skip message for user.name", out.String())
	}
	if !strings.Contains(out.String(), "git email is already set (existing@example.com), skipping.") {
		t.Errorf("output = %q, want a skip message for user.email", out.String())
	}
	for _, c := range gitRunner.Calls {
		if matchConfigSet("user.name")(c.Args) || matchConfigSet("user.email")(c.Args) {
			t.Errorf("should not have written already-set identity, got call: %v", c.Args)
		}
	}
}

func TestBootstrapper_Run_ExistingSSHKeySkipsKeygen(t *testing.T) {
	b, cmdRunner, _, out, _ := testBootstrapper(t, "linux", true)
	home := mustHome(b)
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sshDir, "id_ecdsa.pub"), []byte("ssh-ecdsa existing-key\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := b.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	for _, c := range cmdRunner.Calls {
		if c.Name == "ssh-keygen" {
			t.Errorf("should not have generated a new key, got call: %v", c)
		}
	}
	if !strings.Contains(out.String(), "Using your existing SSH key") {
		t.Errorf("output = %q, want it to mention the existing key", out.String())
	}
}

func TestBootstrapper_Run_RegistrationAlreadyInUse(t *testing.T) {
	b, _, _, out, registrar := testBootstrapper(t, "linux", true)
	registrar.err = errors.New("PUT https://api.github.com/user/keys: 422 key is already in use")

	if err := b.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(out.String(), "already registered") {
		t.Errorf("output = %q, want an already-registered message", out.String())
	}
}

func TestBootstrapper_Run_RegistrationFailureFallsBackToInstructions(t *testing.T) {
	b, _, _, out, registrar := testBootstrapper(t, "linux", true)
	registrar.err = errors.New("403 Forbidden")

	if err := b.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(out.String(), "github.com/settings/ssh/new") {
		t.Errorf("output = %q, want manual setup instructions", out.String())
	}
}

func TestBootstrapper_Run_NoAuthenticateFallsBackToInstructions(t *testing.T) {
	b, _, _, out, _ := testBootstrapper(t, "linux", true)
	b.Authenticate = func(context.Context) (setup.SSHKeyRegistrar, error) {
		return nil, errors.New("could not sign in")
	}

	if err := b.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(out.String(), "github.com/settings/ssh/new") {
		t.Errorf("output = %q, want manual setup instructions", out.String())
	}
}

func mustHome(b *setup.Bootstrapper) string {
	home, _ := b.HomeDir()
	return home
}

func assertCalled(t *testing.T, runner *setuptest.FakeCommandRunner, name string, args ...string) {
	t.Helper()
	for _, c := range runner.Calls {
		if c.Name != name || len(c.Args) != len(args) {
			continue
		}
		match := true
		for i := range args {
			if c.Args[i] != args[i] {
				match = false
				break
			}
		}
		if match {
			return
		}
	}
	t.Errorf("expected a call to %s %v, got calls: %v", name, args, runner.Calls)
}

func assertGitCalled(t *testing.T, runner *gittest.FakeRunner, args ...string) {
	t.Helper()
	for _, c := range runner.Calls {
		if len(c.Args) != len(args) {
			continue
		}
		match := true
		for i := range args {
			if c.Args[i] != args[i] {
				match = false
				break
			}
		}
		if match {
			return
		}
	}
	t.Errorf("expected a git call %v, got calls: %v", args, runner.Calls)
}
