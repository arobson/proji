package cli

import (
	"context"
	"fmt"

	"github.com/arobson/proji/internal/auth"
)

// ensureGitHubToken signs the user in (using a stored token if still valid,
// otherwise Device Flow or a pasted PAT) and returns a token plus a
// GitHubAPI client authenticated with it.
func ensureGitHubToken(ctx context.Context, deps *Deps) (string, GitHubAPI, error) {
	flow := &auth.Flow{
		ClientID: deps.ClientID,
		Store:    deps.CredsStore,
		Prompt:   deps.Prompt,
		Validate: func(ctx context.Context, token string) (string, error) {
			return deps.NewGitHubClient(token).CurrentUser(ctx)
		},
		Endpoint: deps.OAuthEndpoint,
		Out:      deps.Out,
	}

	token, err := flow.EnsureToken(ctx)
	if err != nil {
		return "", nil, fmt.Errorf("unable to authenticate with GitHub: %w", err)
	}
	return token, deps.NewGitHubClient(token), nil
}
