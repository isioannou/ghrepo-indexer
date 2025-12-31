package elasticsearch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/user/ghrepo-indexer/internal/config"
	"github.com/user/ghrepo-indexer/internal/models"
)

func setESHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Elastic-Product", "Elasticsearch")
}

func withInfoHandler(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setESHeaders(w)
		// Handle the Info endpoint that NewClient calls to test connection
		if r.Method == "GET" && r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"version": {"number": "8.0.0"}}`))
			return
		}
		handler(w, r)
	}
}

func newTestESClient(t *testing.T, server *httptest.Server) *Client {
	cfg := &config.Config{
		ElasticsearchURL: server.URL,
		IndexName:        "test-index",
	}
	client, err := NewClient(cfg)
	require.NoError(t, err)
	return client
}

func TestEnsureIndex_IndexExists(t *testing.T) {
	server := httptest.NewServer(withInfoHandler(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" && strings.Contains(r.URL.Path, "test-index") {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := newTestESClient(t, server)
	err := client.EnsureIndex(context.Background())

	require.NoError(t, err)
}

func TestEnsureIndex_CreateIndex(t *testing.T) {
	indexCreated := false
	server := httptest.NewServer(withInfoHandler(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" && strings.Contains(r.URL.Path, "test-index") {
			// Index doesn't exist
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method == "PUT" && strings.Contains(r.URL.Path, "test-index") {
			indexCreated = true
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(`{"acknowledged": true}`))
			require.NoError(t, err)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := newTestESClient(t, server)
	err := client.EnsureIndex(context.Background())

	require.NoError(t, err)
	assert.True(t, indexCreated)
}

func TestEnsureIndex_CreateIndexError(t *testing.T) {
	server := httptest.NewServer(withInfoHandler(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method == "PUT" {
			w.WriteHeader(http.StatusBadRequest)
			_, err := w.Write([]byte(`{"error": {"type": "resource_already_exists_exception", "reason": "index already exists"}}`))
			require.NoError(t, err)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := newTestESClient(t, server)
	err := client.EnsureIndex(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create index")
}

func TestBulkIndex_Success(t *testing.T) {
	bulkCalled := false
	server := httptest.NewServer(withInfoHandler(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && strings.Contains(r.URL.Path, "_bulk") {
			bulkCalled = true
			w.WriteHeader(http.StatusOK)
			response := `{
				"took": 30,
				"errors": false,
				"items": [
					{"index": {"_id": "1", "status": 201}},
					{"index": {"_id": "2", "status": 201}}
				]
			}`
			_, err := w.Write([]byte(response))
			require.NoError(t, err)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := newTestESClient(t, server)

	repos := []*models.Repository{
		{
			ID:       1,
			Name:     "repo1",
			FullName: "org/repo1",
		},
		{
			ID:       2,
			Name:     "repo2",
			FullName: "org/repo2",
		},
	}

	err := client.BulkIndex(context.Background(), repos)

	require.NoError(t, err)
	assert.True(t, bulkCalled)
}

func TestBulkIndex_EmptyList(t *testing.T) {
	server := httptest.NewServer(withInfoHandler(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Server should not be called for empty list")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := newTestESClient(t, server)

	err := client.BulkIndex(context.Background(), []*models.Repository{})

	require.NoError(t, err)
}

func TestBulkIndex_PartialErrors(t *testing.T) {
	server := httptest.NewServer(withInfoHandler(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && strings.Contains(r.URL.Path, "_bulk") {
			w.WriteHeader(http.StatusOK)
			response := `{
				"took": 30,
				"errors": true,
				"items": [
					{"index": {"_id": "1", "status": 201}},
					{"index": {"_id": "2", "status": 400, "error": {"type": "mapper_parsing_exception", "reason": "failed to parse"}}}
				]
			}`
			_, err := w.Write([]byte(response))
			require.NoError(t, err)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := newTestESClient(t, server)

	repos := []*models.Repository{
		{ID: 1, Name: "repo1", FullName: "org/repo1"},
		{ID: 2, Name: "repo2", FullName: "org/repo2"},
	}

	err := client.BulkIndex(context.Background(), repos)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "bulk indexing errors")
	assert.Contains(t, err.Error(), "mapper_parsing_exception")
}

func TestBulkIndex_RequestError(t *testing.T) {
	server := httptest.NewServer(withInfoHandler(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte(`{"error": "server error"}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	client := newTestESClient(t, server)

	repos := []*models.Repository{
		{ID: 1, Name: "repo1", FullName: "org/repo1"},
	}

	err := client.BulkIndex(context.Background(), repos)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "bulk request failed")
}
