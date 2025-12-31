package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v9"
	"github.com/user/ghrepo-indexer/internal/config"
	"github.com/user/ghrepo-indexer/internal/models"
)

type Client struct {
	client    *elasticsearch.Client
	indexName string
}

func NewClient(cfg *config.Config) (*Client, error) {
	esConfig := elasticsearch.Config{
		Addresses: []string{cfg.ElasticsearchURL},
		APIKey:    cfg.ElasticsearchAPIKey,
	}

	client, err := elasticsearch.NewClient(esConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create elasticsearch client: %w", err)
	}

	res, err := client.Info()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to elasticsearch: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("elasticsearch connection error: %s", res.String())
	}

	return &Client{
		client:    client,
		indexName: cfg.IndexName,
	}, nil
}

func (c *Client) EnsureIndex(ctx context.Context) error {
	res, err := c.client.Indices.Exists(
		[]string{c.indexName},
		c.client.Indices.Exists.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("failed to check index existence: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusOK {
		log.Printf("Index %s already exists", c.indexName)
		return nil
	}

	// Create index with mappings
	mapping := `{
		"mappings": {
			"properties": {
				"id": { "type": "long" },
				"name": { "type": "keyword" },
				"full_name": { "type": "keyword" },
				"organization": { "type": "keyword" },
				"description": { "type": "text" },
				"url": { "type": "keyword" },
				"tags": {
					"type": "nested",
					"properties": {
						"name": { "type": "keyword" },
						"commit_sha": { "type": "keyword" },
						"commit_url": { "type": "keyword" }
					}
				},
				"tag_count": { "type": "integer" },
				"indexed_at": { "type": "date" }
			}
		}
	}`

	res, err = c.client.Indices.Create(
		c.indexName,
		c.client.Indices.Create.WithContext(ctx),
		c.client.Indices.Create.WithBody(strings.NewReader(mapping)),
	)
	if err != nil {
		return fmt.Errorf("failed to create index: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("failed to create index: %s - %s", res.Status(), string(body))
	}

	log.Printf("Created index %s", c.indexName)
	return nil
}

func (c *Client) BulkIndex(ctx context.Context, repos []*models.Repository) error {
	if len(repos) == 0 {
		return nil
	}

	var buf bytes.Buffer
	for _, repo := range repos {
		repo.TagCount = len(repo.Tags)
		repo.IndexedAt = time.Now().UTC()

		meta := map[string]interface{}{
			"index": map[string]interface{}{
				"_index": c.indexName,
				"_id":    strconv.FormatInt(repo.ID, 10),
			},
		}
		if err := json.NewEncoder(&buf).Encode(meta); err != nil {
			return fmt.Errorf("failed to encode bulk metadata: %w", err)
		}

		if err := json.NewEncoder(&buf).Encode(repo); err != nil {
			return fmt.Errorf("failed to encode repository: %w", err)
		}
	}

	res, err := c.client.Bulk(
		bytes.NewReader(buf.Bytes()),
		c.client.Bulk.WithContext(ctx),
		c.client.Bulk.WithRefresh("false"),
	)
	if err != nil {
		return fmt.Errorf("failed to execute bulk request: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("bulk request failed: %s - %s", res.Status(), string(body))
	}

	// Parse response to check for individual errors
	var bulkResponse struct {
		Errors bool `json:"errors"`
		Items  []struct {
			Index struct {
				ID     string `json:"_id"`
				Status int    `json:"status"`
				Error  struct {
					Type   string `json:"type"`
					Reason string `json:"reason"`
				} `json:"error"`
			} `json:"index"`
		} `json:"items"`
	}

	if err := json.NewDecoder(res.Body).Decode(&bulkResponse); err != nil {
		return fmt.Errorf("failed to parse bulk response: %w", err)
	}

	if bulkResponse.Errors {
		var errorMsgs []string
		for _, item := range bulkResponse.Items {
			if item.Index.Status >= 400 {
				errorMsgs = append(errorMsgs, fmt.Sprintf("doc %s: %s - %s",
					item.Index.ID, item.Index.Error.Type, item.Index.Error.Reason))
			}
		}
		if len(errorMsgs) > 0 {
			return fmt.Errorf("bulk indexing errors: %s", strings.Join(errorMsgs, "; "))
		}
	}

	return nil
}
