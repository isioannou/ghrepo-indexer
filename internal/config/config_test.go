package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, "github-repos", cfg.IndexName)
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectedErr error
	}{
		{
			name: "valid config with API key",
			config: &Config{
				GitHubToken:         "ghp_token123",
				GitHubOrgs:          []string{"elastic", "kubernetes"},
				ElasticsearchURL:    "https://localhost:9200",
				ElasticsearchAPIKey: "api-key-123",
				IndexName:           "github-repos",
			},
			expectedErr: nil,
		},
		{
			name: "missing github token",
			config: &Config{
				GitHubOrgs:          []string{"elastic"},
				ElasticsearchURL:    "https://localhost:9200",
				ElasticsearchAPIKey: "api-key-123",
			},
			expectedErr: ErrMissingGitHubToken,
		},
		{
			name: "empty github token",
			config: &Config{
				GitHubToken:         "",
				GitHubOrgs:          []string{"elastic"},
				ElasticsearchURL:    "https://localhost:9200",
				ElasticsearchAPIKey: "api-key-123",
			},
			expectedErr: ErrMissingGitHubToken,
		},
		{
			name: "missing github orgs",
			config: &Config{
				GitHubToken:         "ghp_token123",
				GitHubOrgs:          nil,
				ElasticsearchURL:    "https://localhost:9200",
				ElasticsearchAPIKey: "api-key-123",
			},
			expectedErr: ErrMissingGitHubOrgs,
		},
		{
			name: "empty github orgs slice",
			config: &Config{
				GitHubToken:         "ghp_token123",
				GitHubOrgs:          []string{},
				ElasticsearchURL:    "https://localhost:9200",
				ElasticsearchAPIKey: "api-key-123",
			},
			expectedErr: ErrMissingGitHubOrgs,
		},
		{
			name: "github orgs with only whitespace",
			config: &Config{
				GitHubToken:         "ghp_token123",
				GitHubOrgs:          []string{"  ", "   "},
				ElasticsearchURL:    "https://localhost:9200",
				ElasticsearchAPIKey: "api-key-123",
			},
			expectedErr: ErrMissingGitHubOrgs,
		},
		{
			name: "missing elasticsearch URL",
			config: &Config{
				GitHubToken:         "ghp_token123",
				GitHubOrgs:          []string{"elastic"},
				ElasticsearchURL:    "",
				ElasticsearchAPIKey: "api-key-123",
			},
			expectedErr: ErrMissingElasticsearchURL,
		},
		{
			name: "invalid elasticsearch URL",
			config: &Config{
				GitHubToken:         "ghp_token123",
				GitHubOrgs:          []string{"elastic"},
				ElasticsearchURL:    "not-a-valid-url",
				ElasticsearchAPIKey: "api-key-123",
			},
			expectedErr: ErrInvalidElasticsearchURL,
		},
		{
			name: "missing elasticsearch API key",
			config: &Config{
				GitHubToken:      "ghp_token123",
				GitHubOrgs:       []string{"elastic"},
				ElasticsearchURL: "https://localhost:9200",
			},
			expectedErr: ErrMissingElasticsearchAPIKey,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectedErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.expectedErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestConfig_Validate_TrimsWhitespaceFromOrgs(t *testing.T) {
	cfg := &Config{
		GitHubToken:         "ghp_token123",
		GitHubOrgs:          []string{"  elastic  ", "kubernetes", "  golang  "},
		ElasticsearchURL:    "https://localhost:9200",
		ElasticsearchAPIKey: "api-key-123",
	}

	err := cfg.Validate()
	require.NoError(t, err)

	assert.Equal(t, []string{"elastic", "kubernetes", "golang"}, cfg.GitHubOrgs)
}

func TestParseOrganizations(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "single org",
			input:    "elastic",
			expected: []string{"elastic"},
		},
		{
			name:     "multiple orgs",
			input:    "elastic,kubernetes,golang",
			expected: []string{"elastic", "kubernetes", "golang"},
		},
		{
			name:     "orgs with whitespace",
			input:    "  elastic  ,  kubernetes  ,  golang  ",
			expected: []string{"elastic", "kubernetes", "golang"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:     "only commas",
			input:    ",,,",
			expected: []string{},
		},
		{
			name:     "orgs with empty entries",
			input:    "elastic,,kubernetes,",
			expected: []string{"elastic", "kubernetes"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseOrganizations(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
