// Package setup bootstraps a fresh machine that doesn't have git installed
// yet: installing git, generating an SSH key, configuring the user's git
// identity, and registering the SSH key with GitHub.
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
// Run stops immediately rather than treating it like a normal failure.
var errXcodeInstallerTriggered = errors.New("the Xcode Command Line Tools installer was opened; finish it, then run this command again")

// Bootstrapper installs git and sets up a new machine to use it, when git
// isn't already on PATH. Every field has a production default that's easy
// to fake in tests.
type Bootstrapper struct {
	GOOS   string
	Prompt prompt.Prompter
	Out    io.Writer
	Runner CommandRunner

	LookPath      func(file string) (string, error)
	ReadOSRelease func() (map[string]string, error)
	HomeDir       func() (string, error)
	Hostname      func() (string, error)

	// Authenticate signs the user in to GitHub and returns a client that
	// can register their SSH key. A nil result (or an error) means proji
	// couldn't sign in, so the SSH key is printed for manual setup instead.
	Authenticate func(ctx context.Context) (SSHKeyRegistrar, error)
}

// Run installs git (with the user's confirmation), generates an SSH key if
// one doesn't already exist, configures the user's git identity, and
// registers the SSH key with GitHub (or prints it for manual setup).
func (b *Bootstrapper) Run(ctx context.Context) error {
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

	pubKeyPath, err := b.ensureSSHKey(ctx)
	if err != nil {
		return fmt.Errorf("set up an SSH key: %w", err)
	}

	if err := b.ensureGitIdentity(ctx); err != nil {
		return fmt.Errorf("set up your git identity: %w", err)
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

// ensureSSHKey generates a passphrase-less ECDSA-256 SSH key at the
// standard location if one doesn't already exist there, and returns the
// public key's path either way.
func (b *Bootstrapper) ensureSSHKey(ctx context.Context) (string, error) {
	home, err := b.HomeDir()
	if err != nil {
		return "", err
	}
	sshDir := filepath.Join(home, ".ssh")
	privPath := filepath.Join(sshDir, "id_ecdsa")
	pubPath := privPath + ".pub"

	if _, err := os.Stat(pubPath); err == nil {
		fmt.Fprintf(b.Out, "Using your existing SSH key at %s.\n", pubPath)
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

func (b *Bootstrapper) ensureGitIdentity(ctx context.Context) error {
	name, err := b.askNonEmpty("What name should git use for your commits? ")
	if err != nil {
		return err
	}
	email, err := b.askNonEmpty("What email should git use for your commits? ")
	if err != nil {
		return err
	}
	if err := b.Runner.Run(ctx, "git", "config", "--global", "user.name", name); err != nil {
		return fmt.Errorf("set git user.name: %w", err)
	}
	if err := b.Runner.Run(ctx, "git", "config", "--global", "user.email", email); err != nil {
		return fmt.Errorf("set git user.email: %w", err)
	}
	return nil
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
// When it can't sign in or the API call fails, it prints the key and
// manual setup instructions instead.
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
			fmt.Fprintln(b.Out, "This SSH key is already registered with a GitHub account.")
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
