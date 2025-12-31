package config

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// Config holds all application configuration.
type Config struct {
	GitHubToken         string   `mapstructure:"github_token"`
	GitHubOrgs          []string `mapstructure:"github_orgs"`
	ElasticsearchURL    string   `mapstructure:"elasticsearch_url"`
	ElasticsearchAPIKey string   `mapstructure:"elasticsearch_api_key"`
	IndexName           string   `mapstructure:"es_index_name"`
}

func DefaultConfig() *Config {
	return &Config{
		IndexName: "github-repos",
	}
}

var (
	ErrMissingGitHubToken         = errors.New("github token is required")
	ErrMissingGitHubOrgs          = errors.New("at least one github organization is required")
	ErrMissingElasticsearchURL    = errors.New("elasticsearch URL is required")
	ErrInvalidElasticsearchURL    = errors.New("elasticsearch URL is invalid")
	ErrMissingElasticsearchAPIKey = errors.New("elasticsearch API key is required")
)

// Validate checks if the configuration is valid.
func (c *Config) Validate() error {
	if c.GitHubToken == "" {
		return ErrMissingGitHubToken
	}

	if len(c.GitHubOrgs) == 0 {
		return ErrMissingGitHubOrgs
	}

	var validOrgs []string
	for _, org := range c.GitHubOrgs {
		org = strings.TrimSpace(org)
		if org != "" {
			validOrgs = append(validOrgs, org)
		}
	}
	if len(validOrgs) == 0 {
		return ErrMissingGitHubOrgs
	}
	c.GitHubOrgs = validOrgs

	if c.ElasticsearchURL == "" {
		return ErrMissingElasticsearchURL
	}

	if _, err := url.ParseRequestURI(c.ElasticsearchURL); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidElasticsearchURL, err)
	}

	if c.ElasticsearchAPIKey == "" {
		return ErrMissingElasticsearchAPIKey
	}

	return nil
}

// ParseOrganizations parses a comma-separated string of organizations into a slice.
func ParseOrganizations(orgsStr string) []string {
	if orgsStr == "" {
		return nil
	}

	parts := strings.Split(orgsStr, ",")
	orgs := make([]string, 0, len(parts))
	for _, part := range parts {
		org := strings.TrimSpace(part)
		if org != "" {
			orgs = append(orgs, org)
		}
	}
	return orgs
}
