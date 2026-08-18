// Package setup bootstraps a fresh machine that doesn't have git installed
// yet: installing git, generating an SSH key, configuring the user's git
// identity, and registering the SSH key with GitHub. Every step is
// idempotent: a step that's already done is skipped (with a message)
// rather than repeated or treated as an error, so a command that failed
// partway through — for a missing prerequisite, a network error, or
// anything else — can simply be re-run to completion.
package setup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/arobson/proji/internal/git"
	"github.com/arobson/proji/internal/prompt"
)

// SSHKeyRegistrar registers a public key with a GitHub account. It's
// satisfied by ghclient.Client (and by cli.GitHubAPI, structurally,
// without either package importing this one).
type SSHKeyRegistrar interface {
	AddSSHKey(ctx context.Context, title, publicKey string) error
}

// errXcodeInstallerTriggered is returned when macOS install falls back to
// launching the (asynchronous, GUI) Xcode Command Line Tools installer;
// callers stop immediately rather than treating it like a normal failure.
var errXcodeInstallerTriggered = errors.New("the Xcode Command Line Tools installer was opened; finish it, then run this command again")

// Bootstrapper installs git and configures a machine to use it. Every field
// has a production default that's easy to fake in tests.
type Bootstrapper struct {
	GOOS   string
	Prompt prompt.Prompter
	Out    io.Writer

	// Runner executes interactive/system commands: package installs,
	// ssh-keygen, sudo. Its output goes straight to the real terminal.
	Runner CommandRunner

	// GitRunner executes git subcommands whose output needs to be read
	// (config gets, in particular), unlike Runner's fire-and-forget style.
	GitRunner git.Runner

	LookPath      func(file string) (string, error)
	ReadOSRelease func() (map[string]string, error)
	HomeDir       func() (string, error)
	Hostname      func() (string, error)

	// Authenticate signs the user in to GitHub and returns a client that
	// can register their SSH key. A nil result (or an error) means proji
	// couldn't sign in, so the SSH key is printed for manual setup instead.
	Authenticate func(ctx context.Context) (SSHKeyRegistrar, error)
}

// Run installs git (with the user's confirmation) and then configures it.
// Callers that have already confirmed git is missing (the auto-triggered
// bootstrap) use this; callers that want to configure an already-installed
// git (the explicit "proji init" command) call InstallGit and ConfigureGit
// separately so they can skip the install step entirely.
func (b *Bootstrapper) Run(ctx context.Context) error {
	if err := b.InstallGit(ctx); err != nil {
		return err
	}
	return b.ConfigureGit(ctx)
}

// InstallGit asks the user to confirm installing git, then does so. It
// assumes the caller has already established that git is missing.
func (b *Bootstrapper) InstallGit(ctx context.Context) error {
	confirmed, err := b.Prompt.Confirm("git isn't installed on this computer. Would you like proji to install it now?", true)
	if err != nil {
		return fmt.Errorf("ask about installing git: %w", err)
	}
	if !confirmed {
		return errors.New("git is required to use proji. Install it, then run this command again")
	}

	osRelease, _ := b.readOSRelease()
	switch platform := DetectPlatform(b.goos(), osRelease); platform {
	case PlatformMacOS:
		if err := b.installMacOS(ctx); err != nil {
			if errors.Is(err, errXcodeInstallerTriggered) {
				return err
			}
			return fmt.Errorf("install git: %w", err)
		}
	case PlatformDebianFamily:
		if err := b.installDebianFamily(ctx); err != nil {
			return fmt.Errorf("install git: %w", err)
		}
	default:
		return errors.New("proji doesn't know how to install git automatically on this system. Install it yourself from https://git-scm.com/downloads, then run this command again")
	}

	if _, err := b.LookPath("git"); err != nil {
		return errors.New("git still isn't available after attempting to install it. Try installing it yourself, then run this command again")
	}
	fmt.Fprintln(b.Out, "git is installed.")
	return nil
}

// ConfigureGit sets the global default branch name and git identity (if
// not already set), generates an SSH key (if one doesn't already exist),
// and registers it with GitHub. Every step is skipped, with a message,
// when it's already done — safe to call as often as needed.
func (b *Bootstrapper) ConfigureGit(ctx context.Context) error {
	home, err := b.HomeDir()
	if err != nil {
		return err
	}
	repo := git.NewRepo(b.GitRunner, home)

	if err := b.ensureDefaultBranch(ctx, repo); err != nil {
		return fmt.Errorf("set the default branch name: %w", err)
	}
	if err := b.ensureGitIdentity(ctx, repo); err != nil {
		return fmt.Errorf("set up your git identity: %w", err)
	}

	pubKeyPath, err := b.ensureSSHKey(ctx, home)
	if err != nil {
		return fmt.Errorf("set up an SSH key: %w", err)
	}

	b.registerSSHKey(ctx, pubKeyPath)
	return nil
}

func (b *Bootstrapper) installMacOS(ctx context.Context) error {
	if _, err := b.LookPath("brew"); err == nil {
		return b.Runner.Run(ctx, "brew", "install", "git")
	}

	fmt.Fprintln(b.Out, "Homebrew isn't installed, so proji can't install git silently. Opening the Xcode Command Line Tools installer instead...")
	if err := b.Runner.Run(ctx, "xcode-select", "--install"); err != nil {
		return fmt.Errorf("open the Xcode Command Line Tools installer: %w", err)
	}
	return errXcodeInstallerTriggered
}

func (b *Bootstrapper) installDebianFamily(ctx context.Context) error {
	if err := b.Runner.Run(ctx, "sudo", "apt-get", "update"); err != nil {
		return fmt.Errorf("update apt package lists: %w", err)
	}
	if err := b.Runner.Run(ctx, "sudo", "apt-get", "install", "-y", "git"); err != nil {
		return fmt.Errorf("install git via apt-get: %w", err)
	}
	return nil
}

// ensureDefaultBranch sets git's global default branch name to "main",
// skipping the write if it's already set that way.
func (b *Bootstrapper) ensureDefaultBranch(ctx context.Context, repo *git.Repo) error {
	const key = "init.defaultBranch"
	current, ok, err := repo.ConfigGetGlobal(ctx, key)
	if err != nil {
		return err
	}
	if ok && current == "main" {
		fmt.Fprintln(b.Out, `Default branch name is already set to "main", skipping.`)
		return nil
	}
	if err := repo.ConfigSetGlobal(ctx, key, "main"); err != nil {
		return err
	}
	fmt.Fprintln(b.Out, `Set the default branch name to "main".`)
	return nil
}

// ensureGitIdentity sets user.name and user.email globally, skipping (and
// not prompting for) whichever is already set.
func (b *Bootstrapper) ensureGitIdentity(ctx context.Context, repo *git.Repo) error {
	if err := b.ensureGlobalConfig(ctx, repo, "user.name", "name", "What name should git use for your commits? "); err != nil {
		return err
	}
	return b.ensureGlobalConfig(ctx, repo, "user.email", "email", "What email should git use for your commits? ")
}

func (b *Bootstrapper) ensureGlobalConfig(ctx context.Context, repo *git.Repo, key, label, question string) error {
	current, ok, err := repo.ConfigGetGlobal(ctx, key)
	if err != nil {
		return err
	}
	if ok && current != "" {
		fmt.Fprintf(b.Out, "git %s is already set (%s), skipping.\n", label, current)
		return nil
	}
	value, err := b.askNonEmpty(question)
	if err != nil {
		return err
	}
	if err := repo.ConfigSetGlobal(ctx, key, value); err != nil {
		return err
	}
	fmt.Fprintf(b.Out, "Set git %s to %q.\n", label, value)
	return nil
}

// ensureSSHKey generates a passphrase-less ECDSA-256 SSH key at the
// standard location if one doesn't already exist there, and returns the
// public key's path either way.
func (b *Bootstrapper) ensureSSHKey(ctx context.Context, home string) (string, error) {
	sshDir := filepath.Join(home, ".ssh")
	privPath := filepath.Join(sshDir, "id_ecdsa")
	pubPath := privPath + ".pub"

	if _, err := os.Stat(pubPath); err == nil {
		fmt.Fprintf(b.Out, "Using your existing SSH key at %s, skipping.\n", pubPath)
		return pubPath, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}

	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		return "", err
	}
	if err := b.Runner.Run(ctx, "ssh-keygen", "-t", "ecdsa", "-b", "256", "-N", "", "-f", privPath, "-q"); err != nil {
		return "", fmt.Errorf("generate an SSH key: %w", err)
	}
	fmt.Fprintf(b.Out, "Generated a new SSH key at %s.\n", pubPath)
	return pubPath, nil
}

func (b *Bootstrapper) askNonEmpty(question string) (string, error) {
	for {
		answer, err := b.Prompt.Ask(question)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(answer) != "" {
			return answer, nil
		}
		fmt.Fprintln(b.Out, "That can't be empty, try again.")
	}
}

// registerSSHKey is best-effort: it never fails the overall bootstrap.
// When it can't sign in, the key is already registered, or the API call
// fails, it prints the key and manual setup instructions instead.
func (b *Bootstrapper) registerSSHKey(ctx context.Context, pubKeyPath string) {
	pubKeyBytes, err := os.ReadFile(pubKeyPath) // #nosec G304 -- path is built internally from HomeDir(), never user input
	if err != nil {
		fmt.Fprintf(b.Out, "Could not read your SSH key to register it with GitHub: %v\n", err)
		b.printManualKeyInstructions("")
		return
	}
	pubKey := strings.TrimSpace(string(pubKeyBytes))

	if b.Authenticate == nil {
		b.printManualKeyInstructions(pubKey)
		return
	}
	registrar, err := b.Authenticate(ctx)
	if err != nil || registrar == nil {
		fmt.Fprintln(b.Out, "Couldn't sign in to GitHub to register your SSH key automatically.")
		b.printManualKeyInstructions(pubKey)
		return
	}

	title := "proji"
	if host, err := b.Hostname(); err == nil && host != "" {
		title = "proji on " + host
	}
	if err := registrar.AddSSHKey(ctx, title, pubKey); err != nil {
		if strings.Contains(err.Error(), "key is already in use") {
			fmt.Fprintln(b.Out, "This SSH key is already registered with a GitHub account, skipping.")
			return
		}
		fmt.Fprintf(b.Out, "Could not register your SSH key with GitHub automatically: %v\n", err)
		b.printManualKeyInstructions(pubKey)
		return
	}
	fmt.Fprintf(b.Out, "Registered your SSH key with GitHub as %q.\n", title)
}

func (b *Bootstrapper) printManualKeyInstructions(pubKey string) {
	if pubKey != "" {
		fmt.Fprintln(b.Out, "Add this SSH key to your GitHub account:")
		fmt.Fprintln(b.Out, pubKey)
	}
	fmt.Fprintln(b.Out, `Open https://github.com/settings/ssh/new, paste the key into the "Key" field, give it a title, and click "Add SSH key".`)
}

func (b *Bootstrapper) goos() string {
	if b.GOOS != "" {
		return b.GOOS
	}
	return runtime.GOOS
}

func (b *Bootstrapper) readOSRelease() (map[string]string, error) {
	if b.ReadOSRelease != nil {
		return b.ReadOSRelease()
	}
	return map[string]string{}, nil
}
