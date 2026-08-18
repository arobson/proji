// Package auth obtains and persists a GitHub token for proji: it prefers a
// previously stored token, then GitHub OAuth Device Flow (if a client ID is
// configured), and falls back to a manually pasted personal access token.
package auth

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"golang.org/x/oauth2"
	oauth2github "golang.org/x/oauth2/github"

	"github.com/arobson/proji/internal/config"
	"github.com/arobson/proji/internal/prompt"
)

const maxSignInAttempts = 3

// Flow obtains and persists a GitHub token.
type Flow struct {
	// ClientID is the GitHub OAuth App client ID used for Device Flow.
	// When empty, EnsureToken falls back to prompting for a manually
	// pasted personal access token.
	ClientID string
	Store    *config.Store
	Prompt   prompt.Prompter
	// Validate confirms a token works and returns the authenticated
	// username; it's typically ghclient.Client.CurrentUser.
	Validate func(ctx context.Context, token string) (username string, err error)
	// Endpoint overrides the OAuth endpoint, e.g. to point Device Flow at
	// a fake server in tests. Defaults to GitHub's real endpoint.
	Endpoint oauth2.Endpoint
	Out      io.Writer
}

// EnsureToken returns a valid GitHub token, obtaining and persisting a new
// one via Device Flow or a pasted PAT if no valid token is already stored.
func (f *Flow) EnsureToken(ctx context.Context) (string, error) {
	creds, err := f.Store.Load()
	switch {
	case err == nil:
		if _, verr := f.Validate(ctx, creds.Token); verr == nil {
			return creds.Token, nil
		}
		fmt.Fprintln(f.out(), "Your stored GitHub credentials are no longer valid. Signing in again.")
	case errors.Is(err, config.ErrNotFound):
		// No credentials yet; proceed to sign in.
	default:
		return "", fmt.Errorf("load stored credentials: %w", err)
	}

	var lastErr error
	for attempt := 1; attempt <= maxSignInAttempts; attempt++ {
		token, tokenType, err := f.obtainToken(ctx)
		if err != nil {
			return "", err
		}

		username, err := f.Validate(ctx, token)
		if err != nil {
			lastErr = err
			fmt.Fprintf(f.out(), "That didn't work: %v. Let's try again.\n", err)
			continue
		}

		if err := f.Store.Save(config.Creds{
			Token:     token,
			TokenType: tokenType,
			Username:  username,
			SavedAt:   time.Now(),
		}); err != nil {
			return "", fmt.Errorf("save credentials: %w", err)
		}
		return token, nil
	}
	return "", fmt.Errorf("could not sign in to GitHub after %d attempts: %w", maxSignInAttempts, lastErr)
}

func (f *Flow) obtainToken(ctx context.Context) (token, tokenType string, err error) {
	if f.ClientID != "" {
		return f.deviceFlow(ctx)
	}
	return f.patPrompt()
}

func (f *Flow) deviceFlow(ctx context.Context) (string, string, error) {
	conf := &oauth2.Config{
		ClientID: f.ClientID,
		Endpoint: f.endpoint(),
		Scopes:   []string{"repo", "write:public_key"},
	}

	devResp, err := conf.DeviceAuth(ctx)
	if err != nil {
		return "", "", fmt.Errorf("start GitHub device authorization: %w", err)
	}

	fmt.Fprintf(f.out(), "To authorize proji, open %s and enter code: %s\n", devResp.VerificationURI, devResp.UserCode)
	if devResp.VerificationURIComplete != "" {
		fmt.Fprintf(f.out(), "Or open: %s\n", devResp.VerificationURIComplete)
	}

	tok, err := conf.DeviceAccessToken(ctx, devResp)
	if err != nil {
		return "", "", fmt.Errorf("complete GitHub device authorization: %w", err)
	}
	return tok.AccessToken, "oauth", nil
}

func (f *Flow) patPrompt() (string, string, error) {
	fmt.Fprintln(f.out(), "Create a token at https://github.com/settings/tokens/new?scopes=repo,write:public_key and paste it below.")
	token, err := f.Prompt.AskSecret("Paste your GitHub token: ")
	if err != nil {
		return "", "", fmt.Errorf("read token: %w", err)
	}
	if token == "" {
		return "", "", errors.New("no token entered")
	}
	return token, "pat", nil
}

func (f *Flow) out() io.Writer {
	if f.Out != nil {
		return f.Out
	}
	return io.Discard
}

func (f *Flow) endpoint() oauth2.Endpoint {
	if f.Endpoint != (oauth2.Endpoint{}) {
		return f.Endpoint
	}
	return oauth2github.Endpoint
}
