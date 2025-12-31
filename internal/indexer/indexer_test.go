package indexer

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/user/ghrepo-indexer/internal/config"
	"github.com/user/ghrepo-indexer/internal/models"
)

type MockVCSClient struct {
	ListRepositoriesFunc func(ctx context.Context, org string) ([]*models.Repository, error)
	ListTagsFunc         func(ctx context.Context, owner, repo string) ([]models.Tag, error)
}

func (m *MockVCSClient) ListRepositories(ctx context.Context, org string) ([]*models.Repository, error) {
	if m.ListRepositoriesFunc != nil {
		return m.ListRepositoriesFunc(ctx, org)
	}
	return nil, nil
}

func (m *MockVCSClient) ListTags(ctx context.Context, owner, repo string) ([]models.Tag, error) {
	if m.ListTagsFunc != nil {
		return m.ListTagsFunc(ctx, owner, repo)
	}
	return nil, nil
}

// MockESClient is a mock implementation of ESClient for testing.
type MockESClient struct {
	EnsureIndexFunc func(ctx context.Context) error
	BulkIndexFunc   func(ctx context.Context, repos []*models.Repository) error

	IndexedRepos []*models.Repository
}

func (m *MockESClient) EnsureIndex(ctx context.Context) error {
	if m.EnsureIndexFunc != nil {
		return m.EnsureIndexFunc(ctx)
	}
	return nil
}

func (m *MockESClient) BulkIndex(ctx context.Context, repos []*models.Repository) error {
	if m.BulkIndexFunc != nil {
		return m.BulkIndexFunc(ctx, repos)
	}
	m.IndexedRepos = append(m.IndexedRepos, repos...)
	return nil
}

func TestNewIndexer(t *testing.T) {
	vcsClient := &MockVCSClient{}
	esClient := &MockESClient{}
	cfg := &config.Config{
		GitHubOrgs: []string{"testorg"},
	}

	indexer := NewIndexer(vcsClient, esClient, cfg)

	assert.NotNil(t, indexer)
	assert.Equal(t, vcsClient, indexer.vcsClient)
	assert.Equal(t, esClient, indexer.esClient)
	assert.Equal(t, cfg, indexer.config)
}

func TestRun_Success(t *testing.T) {
	vcsClient := &MockVCSClient{
		ListRepositoriesFunc: func(ctx context.Context, org string) ([]*models.Repository, error) {
			return []*models.Repository{
				{ID: 1, Name: "repo1", FullName: "testorg/repo1", Organization: org},
				{ID: 2, Name: "repo2", FullName: "testorg/repo2", Organization: org},
			}, nil
		},
		ListTagsFunc: func(ctx context.Context, owner, repo string) ([]models.Tag, error) {
			return []models.Tag{
				{Name: "v1.0.0", CommitSHA: "abc123"},
			}, nil
		},
	}

	esClient := &MockESClient{}

	cfg := &config.Config{
		GitHubOrgs: []string{"testorg"},
	}

	indexer := NewIndexer(vcsClient, esClient, cfg)
	result, err := indexer.Run(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 1, result.TotalOrgs)
	assert.Equal(t, 2, result.TotalRepos)
	assert.Equal(t, 2, result.TotalTags) // 1 tag per repo
	assert.Equal(t, 0, result.TotalErrors)
	assert.Len(t, esClient.IndexedRepos, 2)
}

func TestRun_MultipleOrganizations(t *testing.T) {
	vcsClient := &MockVCSClient{
		ListRepositoriesFunc: func(ctx context.Context, org string) ([]*models.Repository, error) {
			return []*models.Repository{
				{ID: 1, Name: "repo1", FullName: org + "/repo1", Organization: org},
			}, nil
		},
		ListTagsFunc: func(ctx context.Context, owner, repo string) ([]models.Tag, error) {
			return []models.Tag{}, nil
		},
	}

	esClient := &MockESClient{}

	cfg := &config.Config{
		GitHubOrgs: []string{"org1", "org2", "org3"},
	}

	indexer := NewIndexer(vcsClient, esClient, cfg)
	result, err := indexer.Run(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 3, result.TotalOrgs)
	assert.Equal(t, 3, result.TotalRepos)
	assert.Len(t, esClient.IndexedRepos, 3)
}

func TestRun_EnsureIndexError(t *testing.T) {
	vcsClient := &MockVCSClient{}
	esClient := &MockESClient{
		EnsureIndexFunc: func(ctx context.Context) error {
			return errors.New("index creation failed")
		},
	}

	cfg := &config.Config{
		GitHubOrgs: []string{"testorg"},
	}

	indexer := NewIndexer(vcsClient, esClient, cfg)
	result, err := indexer.Run(context.Background())

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to ensure index exists")
}

func TestSyncOrganization_Success(t *testing.T) {
	vcsClient := &MockVCSClient{
		ListRepositoriesFunc: func(ctx context.Context, org string) ([]*models.Repository, error) {
			return []*models.Repository{
				{ID: 1, Name: "repo1", FullName: "testorg/repo1", Organization: org},
			}, nil
		},
		ListTagsFunc: func(ctx context.Context, owner, repo string) ([]models.Tag, error) {
			return []models.Tag{
				{Name: "v1.0.0", CommitSHA: "abc123"},
				{Name: "v2.0.0", CommitSHA: "def456"},
			}, nil
		},
	}

	esClient := &MockESClient{}

	cfg := &config.Config{
		GitHubOrgs: []string{"testorg"},
	}

	indexer := NewIndexer(vcsClient, esClient, cfg)
	result, err := indexer.SyncOrganization(context.Background(), "testorg")

	require.NoError(t, err)
	assert.Equal(t, "testorg", result.Organization)
	assert.Equal(t, 1, result.ReposIndexed)
	assert.Equal(t, 2, result.TotalTags)
	assert.Empty(t, result.Errors)
	assert.Len(t, esClient.IndexedRepos, 1)
	assert.Len(t, esClient.IndexedRepos[0].Tags, 2)
}

func TestSyncOrganization_ListRepositoriesError(t *testing.T) {
	vcsClient := &MockVCSClient{
		ListRepositoriesFunc: func(ctx context.Context, org string) ([]*models.Repository, error) {
			return nil, errors.New("API error")
		},
	}

	esClient := &MockESClient{}

	cfg := &config.Config{
		GitHubOrgs: []string{"testorg"},
	}

	indexer := NewIndexer(vcsClient, esClient, cfg)
	result, err := indexer.SyncOrganization(context.Background(), "testorg")

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to list repositories")
}

func TestSyncOrganization_ListTagsError(t *testing.T) {
	vcsClient := &MockVCSClient{
		ListRepositoriesFunc: func(ctx context.Context, org string) ([]*models.Repository, error) {
			return []*models.Repository{
				{ID: 1, Name: "repo1", FullName: "testorg/repo1", Organization: org},
			}, nil
		},
		ListTagsFunc: func(ctx context.Context, owner, repo string) ([]models.Tag, error) {
			return nil, errors.New("tags API error")
		},
	}

	esClient := &MockESClient{}

	cfg := &config.Config{
		GitHubOrgs: []string{"testorg"},
	}

	indexer := NewIndexer(vcsClient, esClient, cfg)
	result, err := indexer.SyncOrganization(context.Background(), "testorg")

	require.NoError(t, err) // Should not fail, just log error
	assert.Equal(t, 1, result.ReposIndexed)
	assert.Equal(t, 0, result.TotalTags) // No tags due to error
	assert.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0], "failed to fetch tags")
}

func TestSyncOrganization_BulkIndexError(t *testing.T) {
	vcsClient := &MockVCSClient{
		ListRepositoriesFunc: func(ctx context.Context, org string) ([]*models.Repository, error) {
			return []*models.Repository{
				{ID: 1, Name: "repo1", FullName: "testorg/repo1", Organization: org},
				{ID: 2, Name: "repo2", FullName: "testorg/repo2", Organization: org},
			}, nil
		},
		ListTagsFunc: func(ctx context.Context, owner, repo string) ([]models.Tag, error) {
			return []models.Tag{}, nil
		},
	}

	esClient := &MockESClient{
		BulkIndexFunc: func(ctx context.Context, repos []*models.Repository) error {
			return errors.New("bulk indexing failed")
		},
	}

	cfg := &config.Config{
		GitHubOrgs: []string{"testorg"},
	}

	indexer := NewIndexer(vcsClient, esClient, cfg)
	result, err := indexer.SyncOrganization(context.Background(), "testorg")

	require.NoError(t, err)                 // Should not fail, just log error
	assert.Equal(t, 0, result.ReposIndexed) // No repos indexed due to bulk error
	assert.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0], "failed to bulk index")
}

func TestSyncOrganization_EmptyOrganization(t *testing.T) {
	vcsClient := &MockVCSClient{
		ListRepositoriesFunc: func(ctx context.Context, org string) ([]*models.Repository, error) {
			return []*models.Repository{}, nil
		},
	}

	esClient := &MockESClient{}

	cfg := &config.Config{
		GitHubOrgs: []string{"emptyorg"},
	}

	indexer := NewIndexer(vcsClient, esClient, cfg)
	result, err := indexer.SyncOrganization(context.Background(), "emptyorg")

	require.NoError(t, err)
	assert.Equal(t, "emptyorg", result.Organization)
	assert.Equal(t, 0, result.ReposIndexed)
	assert.Equal(t, 0, result.TotalTags)
	assert.Empty(t, result.Errors)
}

func TestRun_ContextCancellation(t *testing.T) {
	vcsClient := &MockVCSClient{
		ListRepositoriesFunc: func(ctx context.Context, org string) ([]*models.Repository, error) {
			// Simulate slow operation
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(100 * time.Millisecond):
				return []*models.Repository{
					{ID: 1, Name: "repo1", FullName: org + "/repo1", Organization: org},
				}, nil
			}
		},
	}

	esClient := &MockESClient{}

	cfg := &config.Config{
		GitHubOrgs: []string{"org1", "org2", "org3"},
	}

	indexer := NewIndexer(vcsClient, esClient, cfg)

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel after a short delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	result, err := indexer.Run(ctx)

	// Should return with context error
	require.Error(t, err)
	assert.Equal(t, context.Canceled, err)
	// Result may be partial
	assert.NotNil(t, result)
}

func TestSyncOrganization_ContextCancellation(t *testing.T) {
	repoCount := 0
	vcsClient := &MockVCSClient{
		ListRepositoriesFunc: func(ctx context.Context, org string) ([]*models.Repository, error) {
			return []*models.Repository{
				{ID: 1, Name: "repo1", FullName: "testorg/repo1", Organization: org},
				{ID: 2, Name: "repo2", FullName: "testorg/repo2", Organization: org},
				{ID: 3, Name: "repo3", FullName: "testorg/repo3", Organization: org},
			}, nil
		},
		ListTagsFunc: func(ctx context.Context, owner, repo string) ([]models.Tag, error) {
			repoCount++
			// Simulate slow tag fetching
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(100 * time.Millisecond):
				return []models.Tag{}, nil
			}
		},
	}

	esClient := &MockESClient{}

	cfg := &config.Config{
		GitHubOrgs: []string{"testorg"},
	}

	indexer := NewIndexer(vcsClient, esClient, cfg)

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel after processing first repo
	go func() {
		time.Sleep(150 * time.Millisecond)
		cancel()
	}()

	result, err := indexer.SyncOrganization(ctx, "testorg")

	require.Error(t, err)
	assert.Equal(t, context.Canceled, err)
	// Should have partial results
	assert.NotNil(t, result)
}

func TestRun_OrgFailureDoesNotStopOthers(t *testing.T) {
	callCount := 0
	vcsClient := &MockVCSClient{
		ListRepositoriesFunc: func(ctx context.Context, org string) ([]*models.Repository, error) {
			callCount++
			if org == "failorg" {
				return nil, errors.New("org not found")
			}
			return []*models.Repository{
				{ID: int64(callCount), Name: "repo", FullName: org + "/repo", Organization: org},
			}, nil
		},
		ListTagsFunc: func(ctx context.Context, owner, repo string) ([]models.Tag, error) {
			return []models.Tag{}, nil
		},
	}

	esClient := &MockESClient{}

	cfg := &config.Config{
		GitHubOrgs: []string{"org1", "failorg", "org3"},
	}

	indexer := NewIndexer(vcsClient, esClient, cfg)
	result, err := indexer.Run(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 3, result.TotalOrgs)
	assert.Equal(t, 2, result.TotalRepos) // org1 and org3 succeeded
	assert.Equal(t, 1, result.TotalErrors)
	assert.Equal(t, 3, callCount) // All orgs were attempted
}
