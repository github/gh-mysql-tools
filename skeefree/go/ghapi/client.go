package ghapi

import (
	"context"

	"github.com/github/skeefree/go/config"
	"github.com/google/go-github/github"

	"golang.org/x/oauth2"
)

// NewGitHubClient creates a new GitHub API client
func newGitHubClient(c *config.Config) (*github.Client, error) {
	tokenSource := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: c.GitHubAPIToken},
	)
	return github.NewClient(oauth2.NewClient(context.Background(), tokenSource)), nil
}
