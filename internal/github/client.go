package github

import (
	"context"
	"fmt"
	"time"

	"github.com/google/go-github/v80/github"
	"github.com/user/ghrepo-indexer/internal/models"
)

type Client struct {
	client *github.Client
}

func NewClient(token string) *Client {
	client := github.NewClient(nil).WithAuthToken(token)

	return &Client{
		client: client,
	}
}

func withRateLimitHandling(ctx context.Context) context.Context {
	return context.WithValue(ctx, github.SleepUntilPrimaryRateLimitResetWhenRateLimited, true)
}

// ListRepositories fetches all repositories for an organization with pagination.
func (c *Client) ListRepositories(ctx context.Context, org string) ([]*models.Repository, error) {
	var allRepos []*models.Repository
	ctx = withRateLimitHandling(ctx)

	opts := &github.RepositoryListByOrgOptions{
		Type: "all",
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	for {
		repos, resp, err := c.client.Repositories.ListByOrg(ctx, org, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list repositories for org %s: %w", org, err)
		}

		for _, repo := range repos {
			allRepos = append(allRepos, convertRepository(repo, org))
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allRepos, nil
}

// ListTags fetches all tags for a repository with pagination.
func (c *Client) ListTags(ctx context.Context, owner, repo string) ([]models.Tag, error) {
	var allTags []models.Tag
	ctx = withRateLimitHandling(ctx)

	opts := &github.ListOptions{
		PerPage: 100,
	}

	for {
		tags, resp, err := c.client.Repositories.ListTags(ctx, owner, repo, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list tags for %s/%s: %w", owner, repo, err)
		}

		for _, tag := range tags {
			allTags = append(allTags, convertTag(tag))
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allTags, nil
}

func convertRepository(repo *github.Repository, org string) *models.Repository {
	r := &models.Repository{
		ID:           repo.GetID(),
		Name:         repo.GetName(),
		FullName:     repo.GetFullName(),
		Organization: org,
		Description:  repo.GetDescription(),
		URL:          repo.GetHTMLURL(),
		Tags:         []models.Tag{},
		TagCount:     0,
		IndexedAt:    time.Now().UTC(),
	}
	return r
}

func convertTag(tag *github.RepositoryTag) models.Tag {
	t := models.Tag{
		Name: tag.GetName(),
	}
	if tag.Commit != nil {
		t.CommitSHA = tag.Commit.GetSHA()
		t.CommitURL = tag.Commit.GetURL()
	}
	return t
}
