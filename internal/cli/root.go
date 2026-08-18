// Package cli wires proji's cobra commands together.
package cli

import (
	"context"
	"io"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/oauth2"

	"github.com/arobson/proji/internal/config"
	"github.com/arobson/proji/internal/ghclient"
	"github.com/arobson/proji/internal/git"
	"github.com/arobson/proji/internal/prompt"
	"github.com/arobson/proji/internal/setup"
)

// githubOAuthClientID is proji's registered GitHub OAuth App, used for
// Device Flow sign-in. OAuth client IDs are public identifiers, not
// secrets: GitHub Device Flow (RFC 8628) is designed for public clients
// and never involves a client secret, so it's safe to bake this in.
const githubOAuthClientID = "Ov23liBp5u8GXm72iuL6"

// GitHubAPI is the subset of ghclient.Client that commands need. Command
// tests inject a hand-written fake instead of standing up an httptest
// server, since ghclient's own tests already cover the HTTP layer.
type GitHubAPI interface {
	ForkRepo(ctx context.Context, owner, repo string) (ghclient.RepoResult, error)
	CreateRepo(ctx context.Context, name string, private bool) (ghclient.RepoResult, error)
	CurrentUser(ctx context.Context) (string, error)
	AddSSHKey(ctx context.Context, title, publicKey string) error
}

// GitBootstrapper installs and initializes git on a machine that doesn't
// have it yet. It's satisfied by *setup.Bootstrapper.
type GitBootstrapper interface {
	Run(ctx context.Context) error
}

// Deps holds every external dependency a command needs, so tests can
// substitute fakes and main.go can wire real implementations.
type Deps struct {
	Out    io.Writer
	ErrOut io.Writer
	In     io.Reader

	CredsStore *config.Store
	Prompt     prompt.Prompter

	// NewGitHubClient builds a GitHubAPI authenticated with token. It's a
	// factory (not a fixed instance) because the token isn't known until
	// after auth.Flow.EnsureToken runs, and auth.Flow itself needs to
	// build a client per candidate token while validating sign-in.
	NewGitHubClient func(token string) GitHubAPI

	// NewRunner builds a git.Runner. It's a factory (rather than a shared
	// instance) so each command creates a Runner bound to its own context
	// as needed; tests override it to return a shared *gittest.FakeRunner.
	NewRunner func() git.Runner

	// LookPath checks whether a program is available, used to detect
	// whether git itself is installed before any command that needs it.
	LookPath func(file string) (string, error)

	// NewBootstrapper builds a GitBootstrapper to install and initialize
	// git when LookPath reports it's missing.
	NewBootstrapper func() GitBootstrapper

	Now   func() time.Time
	Getwd func() (string, error)

	ClientID      string
	OAuthEndpoint oauth2.Endpoint
}

// DefaultDeps wires real, production dependencies.
func DefaultDeps() *Deps {
	credsPath, err := config.CredsPath()
	if err != nil {
		// os.UserHomeDir() failing is exceedingly rare (no HOME/USERPROFILE);
		// fall back to a relative path rather than panicking at startup.
		credsPath = ".proji/creds.yml" // #nosec G101 -- a file path, not a credential
	}

	deps := &Deps{
		Out:        os.Stdout,
		ErrOut:     os.Stderr,
		In:         os.Stdin,
		CredsStore: config.NewStore(credsPath),
		Prompt:     prompt.NewIOPrompter(os.Stdin, os.Stdout),
		NewGitHubClient: func(token string) GitHubAPI {
			return ghclient.NewClient(http.DefaultClient, token)
		},
		NewRunner: func() git.Runner {
			return git.NewExecRunner()
		},
		LookPath: exec.LookPath,
		Now:      time.Now,
		Getwd:    os.Getwd,
		ClientID: githubOAuthClientID,
	}

	// NewBootstrapper is wired after deps is constructed so its
	// Authenticate callback can capture deps itself (only invoked much
	// later, when a command actually needs to bootstrap git).
	deps.NewBootstrapper = func() GitBootstrapper {
		return &setup.Bootstrapper{
			Prompt:        deps.Prompt,
			Out:           deps.Out,
			Runner:        setup.ExecCommandRunner{},
			LookPath:      deps.LookPath,
			ReadOSRelease: setup.ReadOSRelease,
			HomeDir:       os.UserHomeDir,
			Hostname:      os.Hostname,
			Authenticate: func(ctx context.Context) (setup.SSHKeyRegistrar, error) {
				_, client, err := ensureGitHubToken(ctx, deps)
				return client, err
			},
		}
	}

	return deps
}

// NewRootCmd builds the proji root command with all subcommands attached.
func NewRootCmd(deps *Deps) *cobra.Command {
	root := &cobra.Command{
		Use:           "proji",
		Short:         "proji simplifies git for working on instructor-provided assignments",
		SilenceUsage:  true,
		SilenceErrors: false,
	}
	root.SetOut(deps.Out)
	root.SetErr(deps.ErrOut)
	root.SetIn(deps.In)

	root.AddCommand(
		newCopyCmd(deps),
		newCreateCmd(deps),
		newCheckCmd(deps),
		newCheckoutCmd(deps),
		newCheckinCmd(deps),
	)
	return root
}
