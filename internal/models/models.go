package models

import "time"

// Tag represents a Git tag in a repository.
type Tag struct {
	Name      string `json:"name"`
	CommitSHA string `json:"commit_sha"`
	CommitURL string `json:"commit_url"`
}

// Repository represents a GitHub repository with its tags.
type Repository struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	FullName     string    `json:"full_name"`
	Organization string    `json:"organization"`
	Description  string    `json:"description"`
	URL          string    `json:"url"`
	Tags         []Tag     `json:"tags"`
	TagCount     int       `json:"tag_count"`
	IndexedAt    time.Time `json:"indexed_at"`
}

// SyncResult represents the result of a synchronization operation.
type SyncResult struct {
	Organization string        `json:"organization"`
	ReposIndexed int           `json:"repos_indexed"`
	TotalTags    int           `json:"total_tags"`
	Errors       []string      `json:"errors,omitempty"`
	Duration     time.Duration `json:"duration"`
	StartedAt    time.Time     `json:"started_at"`
	CompletedAt  time.Time     `json:"completed_at"`
}

// OverallSyncResult represents the aggregated result of syncing multiple organizations.
type OverallSyncResult struct {
	Organizations []SyncResult  `json:"organizations"`
	TotalOrgs     int           `json:"total_orgs"`
	TotalRepos    int           `json:"total_repos"`
	TotalTags     int           `json:"total_tags"`
	TotalErrors   int           `json:"total_errors"`
	Duration      time.Duration `json:"duration"`
	StartedAt     time.Time     `json:"started_at"`
	CompletedAt   time.Time     `json:"completed_at"`
}
