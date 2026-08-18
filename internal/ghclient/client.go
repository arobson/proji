// Package ghclient wraps the GitHub REST API calls proji needs (forking or
// creating a repository, identifying the authenticated user, registering an
// SSH key) behind a small interface.
package ghclient

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/google/go-github/v66/github"
)

const (
	defaultPollInterval = 2 * time.Second
	defaultPollTimeout  = 90 * time.Second
)

// ErrRepoNotFound is returned by GetRepo when no such repository exists.
var ErrRepoNotFound = errors.New("repository not found")

// RepoResult describes a repository proji forked or created (or confirmed
// already exists).
type RepoResult struct {
	Owner    string
	Repo     string
	HTMLURL  string
	CloneURL string
}

// Client wraps a go-github client with proji-specific operations.
type Client struct {
	gh *github.Client

	// PollInterval/PollTimeout/Sleep control how ForkRepo and CreateRepo
	// wait for GitHub to finish asynchronously creating a repository.
	// Tests override Sleep (and shrink PollInterval/PollTimeout) so
	// polling loops run instantly.
	PollInterval time.Duration
	PollTimeout  time.Duration
	Sleep        func(time.Duration)
}

// NewClient returns a Client authenticated with token.
func NewClient(httpClient *http.Client, token string) *Client {
	return &Client{
		gh:           github.NewClient(httpClient).WithAuthToken(token),
		PollInterval: defaultPollInterval,
		PollTimeout:  defaultPollTimeout,
		Sleep:        time.Sleep,
	}
}

// SetBaseURL points the client at a different API base, e.g. an httptest
// server in tests or a GitHub Enterprise instance.
func (c *Client) SetBaseURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	c.gh.BaseURL = u
	c.gh.UploadURL = u
	return nil
}

// CurrentUser returns the login of the authenticated user, and is used to
// validate a token is still good.
func (c *Client) CurrentUser(ctx context.Context) (string, error) {
	user, _, err := c.gh.Users.Get(ctx, "")
	if err != nil {
		return "", err
	}
	return user.GetLogin(), nil
}

// ForkRepo forks owner/repo into the authenticated user's account, waiting
// for GitHub to finish the (asynchronous) fork before returning. Forking is
// idempotent server-side: if a fork already exists, GitHub just returns it.
func (c *Client) ForkRepo(ctx context.Context, owner, repo string) (RepoResult, error) {
	fork, _, err := c.gh.Repositories.CreateFork(ctx, owner, repo, nil)
	var acceptedErr *github.AcceptedError
	if err != nil && !errors.As(err, &acceptedErr) {
		return RepoResult{}, fmt.Errorf("fork %s/%s: %w", owner, repo, err)
	}
	if fork == nil {
		return RepoResult{}, fmt.Errorf("fork %s/%s: no repository returned", owner, repo)
	}

	forkOwner, forkName := fork.GetOwner().GetLogin(), fork.GetName()
	if err := c.waitUntilReady(ctx, forkOwner, forkName); err != nil {
		return RepoResult{}, fmt.Errorf("fork %s/%s: %w", owner, repo, err)
	}

	return RepoResult{
		Owner:    forkOwner,
		Repo:     forkName,
		HTMLURL:  fork.GetHTMLURL(),
		CloneURL: fork.GetCloneURL(),
	}, nil
}

// CreateRepo creates a new repository named name under the authenticated
// user's account, waiting for GitHub to finish creating it before
// returning.
func (c *Client) CreateRepo(ctx context.Context, name string, private bool) (RepoResult, error) {
	created, _, err := c.gh.Repositories.Create(ctx, "", &github.Repository{
		Name:    github.String(name),
		Private: github.Bool(private),
	})
	if err != nil {
		return RepoResult{}, fmt.Errorf("create repository %q: %w", name, err)
	}

	owner, repoName := created.GetOwner().GetLogin(), created.GetName()
	if err := c.waitUntilReady(ctx, owner, repoName); err != nil {
		return RepoResult{}, fmt.Errorf("create repository %q: %w", name, err)
	}

	return RepoResult{
		Owner:    owner,
		Repo:     repoName,
		HTMLURL:  created.GetHTMLURL(),
		CloneURL: created.GetCloneURL(),
	}, nil
}

// GetRepo fetches owner/repo, returning ErrRepoNotFound if it doesn't exist.
func (c *Client) GetRepo(ctx context.Context, owner, repo string) (RepoResult, error) {
	found, resp, err := c.gh.Repositories.Get(ctx, owner, repo)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return RepoResult{}, ErrRepoNotFound
		}
		return RepoResult{}, fmt.Errorf("get repository %s/%s: %w", owner, repo, err)
	}
	return RepoResult{
		Owner:    found.GetOwner().GetLogin(),
		Repo:     found.GetName(),
		HTMLURL:  found.GetHTMLURL(),
		CloneURL: found.GetCloneURL(),
	}, nil
}

// AddSSHKey registers publicKey with the authenticated user's GitHub
// account under the given title.
func (c *Client) AddSSHKey(ctx context.Context, title, publicKey string) error {
	_, _, err := c.gh.Users.CreateKey(ctx, &github.Key{
		Title: github.String(title),
		Key:   github.String(publicKey),
	})
	if err != nil {
		return fmt.Errorf("register SSH key: %w", err)
	}
	return nil
}

func (c *Client) waitUntilReady(ctx context.Context, owner, repo string) error {
	deadline := time.Now().Add(c.PollTimeout)
	for {
		if _, _, err := c.gh.Repositories.Get(ctx, owner, repo); err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for GitHub to finish creating %s/%s", owner, repo)
		}
		c.Sleep(c.PollInterval)
	}
}
