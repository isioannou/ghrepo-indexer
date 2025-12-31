package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-github/v80/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestClient creates a GitHub client configured to use the test server.
func newTestClient(server *httptest.Server) *Client {
	client := github.NewClient(nil)
	client.BaseURL, _ = client.BaseURL.Parse(server.URL + "/")

	return &Client{
		client: client,
	}
}

func TestListRepositories_SinglePage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/orgs/testorg/repos", r.URL.Path)
		assert.Equal(t, "all", r.URL.Query().Get("type"))
		assert.Equal(t, "100", r.URL.Query().Get("per_page"))

		repos := []*github.Repository{
			{
				ID:          github.Ptr(int64(1)),
				Name:        github.Ptr("repo1"),
				FullName:    github.Ptr("testorg/repo1"),
				Description: github.Ptr("Test repo 1"),
				HTMLURL:     github.Ptr("https://github.com/testorg/repo1"),
			},
			{
				ID:          github.Ptr(int64(2)),
				Name:        github.Ptr("repo2"),
				FullName:    github.Ptr("testorg/repo2"),
				Description: github.Ptr("Test repo 2"),
				HTMLURL:     github.Ptr("https://github.com/testorg/repo2"),
			},
		}

		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(repos)
		require.NoError(t, err)
	}))
	defer server.Close()

	client := newTestClient(server)
	repos, err := client.ListRepositories(context.Background(), "testorg")

	require.NoError(t, err)
	require.Len(t, repos, 2)

	assert.Equal(t, int64(1), repos[0].ID)
	assert.Equal(t, "repo1", repos[0].Name)
	assert.Equal(t, "testorg/repo1", repos[0].FullName)
	assert.Equal(t, "testorg", repos[0].Organization)
	assert.Equal(t, "Test repo 1", repos[0].Description)

	assert.Equal(t, int64(2), repos[1].ID)
	assert.Equal(t, "repo2", repos[1].Name)
}

func TestListRepositories_Pagination(t *testing.T) {
	callCount := 0
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		var repos []*github.Repository
		if callCount == 1 {
			// First page
			repos = []*github.Repository{
				{
					ID:       github.Ptr(int64(1)),
					Name:     github.Ptr("repo1"),
					FullName: github.Ptr("testorg/repo1"),
					HTMLURL:  github.Ptr("https://github.com/testorg/repo1"),
				},
			}
			// Link header for pagination
			w.Header().Set("Link", `<`+server.URL+`/orgs/testorg/repos?page=2>; rel="next"`)
		} else {
			// Second page
			repos = []*github.Repository{
				{
					ID:       github.Ptr(int64(2)),
					Name:     github.Ptr("repo2"),
					FullName: github.Ptr("testorg/repo2"),
					HTMLURL:  github.Ptr("https://github.com/testorg/repo2"),
				},
			}
		}

		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(repos)
		require.NoError(t, err)
	}))
	defer server.Close()

	client := newTestClient(server)
	repos, err := client.ListRepositories(context.Background(), "testorg")

	require.NoError(t, err)
	require.Len(t, repos, 2)
	assert.Equal(t, 2, callCount)
}

func TestListRepositories_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		resp := github.ErrorResponse{
			Message: "Not Found",
		}
		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(resp)
		require.NoError(t, err)
	}))
	defer server.Close()

	client := newTestClient(server)
	repos, err := client.ListRepositories(context.Background(), "nonexistent")

	require.Error(t, err)
	assert.Nil(t, repos)
	assert.Contains(t, err.Error(), "failed to list repositories")
}

func TestListTags_SinglePage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/repos/testorg/testrepo/tags", r.URL.Path)
		assert.Equal(t, "100", r.URL.Query().Get("per_page"))

		tags := []*github.RepositoryTag{
			{
				Name: github.Ptr("v1.0.0"),
				Commit: &github.Commit{
					SHA: github.Ptr("abc123"),
					URL: github.Ptr("https://api.github.com/commits/abc123"),
				},
			},
			{
				Name: github.Ptr("v2.0.0"),
				Commit: &github.Commit{
					SHA: github.Ptr("def456"),
					URL: github.Ptr("https://api.github.com/commits/def456"),
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(tags)
		require.NoError(t, err)
	}))
	defer server.Close()

	client := newTestClient(server)
	tags, err := client.ListTags(context.Background(), "testorg", "testrepo")

	require.NoError(t, err)
	require.Len(t, tags, 2)

	assert.Equal(t, "v1.0.0", tags[0].Name)
	assert.Equal(t, "abc123", tags[0].CommitSHA)
	assert.Equal(t, "https://api.github.com/commits/abc123", tags[0].CommitURL)

	assert.Equal(t, "v2.0.0", tags[1].Name)
	assert.Equal(t, "def456", tags[1].CommitSHA)
}

func TestListTags_Pagination(t *testing.T) {
	callCount := 0
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		var tags []*github.RepositoryTag
		if callCount == 1 {
			tags = []*github.RepositoryTag{
				{
					Name:   github.Ptr("v1.0.0"),
					Commit: &github.Commit{SHA: github.Ptr("abc123")},
				},
			}
			w.Header().Set("Link", `<`+server.URL+`/repos/testorg/testrepo/tags?page=2>; rel="next"`)
		} else {
			tags = []*github.RepositoryTag{
				{
					Name:   github.Ptr("v2.0.0"),
					Commit: &github.Commit{SHA: github.Ptr("def456")},
				},
			}
		}

		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(tags)
		require.NoError(t, err)
	}))
	defer server.Close()

	client := newTestClient(server)
	tags, err := client.ListTags(context.Background(), "testorg", "testrepo")

	require.NoError(t, err)
	require.Len(t, tags, 2)
	assert.Equal(t, 2, callCount)
}

func TestListTags_EmptyRepository(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode([]*github.RepositoryTag{})
		require.NoError(t, err)
	}))
	defer server.Close()

	client := newTestClient(server)
	tags, err := client.ListTags(context.Background(), "testorg", "testrepo")

	require.NoError(t, err)
	assert.Empty(t, tags)
}

func TestListTags_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		resp := github.ErrorResponse{
			Message: "Not Found",
		}
		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(resp)
		require.NoError(t, err)
	}))
	defer server.Close()

	client := newTestClient(server)
	tags, err := client.ListTags(context.Background(), "testorg", "nonexistent")

	require.Error(t, err)
	assert.Nil(t, tags)
	assert.Contains(t, err.Error(), "failed to list tags")
}

func TestListRepositories_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response
		repos := []*github.Repository{
			{
				ID:       github.Ptr(int64(1)),
				Name:     github.Ptr("repo1"),
				FullName: github.Ptr("testorg/repo1"),
				HTMLURL:  github.Ptr("https://github.com/testorg/repo1"),
			},
		}
		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(repos)
		require.NoError(t, err)
	}))
	defer server.Close()

	client := newTestClient(server)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	repos, err := client.ListRepositories(ctx, "testorg")

	require.Error(t, err)
	assert.Nil(t, repos)
}

func TestConvertTag_NilCommit(t *testing.T) {
	tag := &github.RepositoryTag{
		Name:   github.Ptr("v1.0.0"),
		Commit: nil,
	}

	result := convertTag(tag)

	assert.Equal(t, "v1.0.0", result.Name)
	assert.Empty(t, result.CommitSHA)
	assert.Empty(t, result.CommitURL)
}
