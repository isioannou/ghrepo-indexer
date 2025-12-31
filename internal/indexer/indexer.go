package indexer

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/user/ghrepo-indexer/internal/config"
	"github.com/user/ghrepo-indexer/internal/elasticsearch"
	"github.com/user/ghrepo-indexer/internal/github"
	"github.com/user/ghrepo-indexer/internal/models"
)

// VCSClient defines the interface for version control system operations.
// This can be implemented by GitHub, GitLab, or other VCS providers.
type VCSClient interface {
	ListRepositories(ctx context.Context, org string) ([]*models.Repository, error)
	ListTags(ctx context.Context, owner, repo string) ([]models.Tag, error)
}

type ESClient interface {
	EnsureIndex(ctx context.Context) error
	BulkIndex(ctx context.Context, repos []*models.Repository) error
}

type Indexer struct {
	vcsClient VCSClient
	esClient  ESClient
	config    *config.Config
}

// NewIndexer creates a new Indexer with the provided clients and configuration.
func NewIndexer(vcsClient VCSClient, esClient ESClient, cfg *config.Config) *Indexer {
	return &Indexer{
		vcsClient: vcsClient,
		esClient:  esClient,
		config:    cfg,
	}
}

// NewIndexerFromConfig creates a new Indexer with clients initialized from configuration.
func NewIndexerFromConfig(cfg *config.Config) (*Indexer, error) {
	vcsClient := github.NewClient(cfg.GitHubToken)

	esClient, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create elasticsearch client: %w", err)
	}

	return &Indexer{
		vcsClient: vcsClient,
		esClient:  esClient,
		config:    cfg,
	}, nil
}

// Run executes a single synchronization for all configured organizations.
func (i *Indexer) Run(ctx context.Context) (*models.OverallSyncResult, error) {
	startTime := time.Now()
	result := &models.OverallSyncResult{
		StartedAt:     startTime,
		Organizations: make([]models.SyncResult, 0, len(i.config.GitHubOrgs)),
	}

	if err := i.esClient.EnsureIndex(ctx); err != nil {
		return nil, fmt.Errorf("failed to ensure index exists: %w", err)
	}

	for _, org := range i.config.GitHubOrgs {
		select {
		case <-ctx.Done():
			result.CompletedAt = time.Now()
			result.Duration = result.CompletedAt.Sub(result.StartedAt)
			return result, ctx.Err()
		default:
		}

		log.Printf("Starting sync for organization: %s", org)
		orgResult, err := i.SyncOrganization(ctx, org)
		if err != nil {
			log.Printf("Error syncing organization %s: %v", org, err)
			orgResult = &models.SyncResult{
				Organization: org,
				Errors:       []string{err.Error()},
				StartedAt:    time.Now(),
				CompletedAt:  time.Now(),
			}
		}
		result.Organizations = append(result.Organizations, *orgResult)
	}

	for _, org := range result.Organizations {
		result.TotalRepos += org.ReposIndexed
		result.TotalTags += org.TotalTags
		result.TotalErrors += len(org.Errors)
	}
	result.TotalOrgs = len(result.Organizations)
	result.CompletedAt = time.Now()
	result.Duration = result.CompletedAt.Sub(result.StartedAt)

	log.Printf("Sync completed: %d orgs, %d repos, %d tags, %d errors in %v",
		result.TotalOrgs, result.TotalRepos, result.TotalTags, result.TotalErrors, result.Duration)

	return result, nil
}

// SyncOrganization synchronizes all repositories for a single organization.
func (i *Indexer) SyncOrganization(ctx context.Context, org string) (*models.SyncResult, error) {
	startTime := time.Now()
	result := &models.SyncResult{
		Organization: org,
		StartedAt:    startTime,
		Errors:       []string{},
	}

	repos, err := i.vcsClient.ListRepositories(ctx, org)
	if err != nil {
		return nil, fmt.Errorf("failed to list repositories for %s: %w", org, err)
	}

	log.Printf("Found %d repositories for organization %s", len(repos), org)

	for idx, repo := range repos {
		select {
		case <-ctx.Done():
			result.CompletedAt = time.Now()
			result.Duration = result.CompletedAt.Sub(result.StartedAt)
			return result, ctx.Err()
		default:
		}

		log.Printf("[%d/%d] Fetching tags for %s", idx+1, len(repos), repo.FullName)

		tags, err := i.vcsClient.ListTags(ctx, org, repo.Name)
		if err != nil {
			errMsg := fmt.Sprintf("failed to fetch tags for %s: %v", repo.FullName, err)
			log.Printf("Warning: %s", errMsg)
			result.Errors = append(result.Errors, errMsg)
			tags = []models.Tag{}
		}

		repo.Tags = tags
		repo.TagCount = len(tags)
		result.TotalTags += len(tags)
	}

	// Bulk index all repositories
	if len(repos) > 0 {
		log.Printf("Bulk indexing %d repositories for %s", len(repos), org)
		if err := i.esClient.BulkIndex(ctx, repos); err != nil {
			errMsg := fmt.Sprintf("failed to bulk index repositories: %v", err)
			log.Printf("Error: %s", errMsg)
			result.Errors = append(result.Errors, errMsg)
		} else {
			result.ReposIndexed = len(repos)
		}
	}

	result.CompletedAt = time.Now()
	result.Duration = result.CompletedAt.Sub(result.StartedAt)

	log.Printf("Completed sync for %s: %d repos, %d tags, %d errors in %v",
		org, result.ReposIndexed, result.TotalTags, len(result.Errors), result.Duration)

	return result, nil
}
