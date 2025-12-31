# GitHub Repository Indexer

A Go CLI tool that synchronizes GitHub organization repository data (names and tags) to Elasticsearch. It supports indexing multiple organizations.

## Features

- Index repositories and their tags from multiple GitHub organizations
- Store repository metadata in Elasticsearch for easy searching
- Rate limit handling for GitHub API
- Pagination support for large organizations

## Prerequisites

- Go 1.21 or later
- GitHub Personal Access Token with `repo` scope (for private repos) or `public_repo` scope (for public repos only)
- Elasticsearch 9.x cluster (can use [Elastic Cloud free trial](https://www.elastic.co/cloud/elasticsearch-service/signup))

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/user/ghrepo-indexer.git
cd ghrepo-indexer

# Build the binary
go build -o indexer .

# Or install directly
go install .
```

## Configuration

The indexer can be configured using a YAML config file, command-line flags, or environment variables.

### Configuration File (Recommended)

Create a `config.yaml`:

```yaml
github_token: ghp_xxxxxxxxxxxx
github_orgs:
  - elastic
  - kubernetes
  - golang
elasticsearch_url: https://your-cluster.es.cloud.es.io:443
elasticsearch_api_key: your-api-key
es_index_name: github-repos
```

Run with:

```bash
./indexer --config config.yaml
```

### Command-Line Flags

```
--config          Path to YAML config file
--github-token    GitHub personal access token
--orgs            GitHub organizations (comma-separated)
--es-url          Elasticsearch URL
--es-api-key      Elasticsearch API key
--index           Elasticsearch index name (default: github-repos)
```

### Environment Variables

| Variable | Description | Required |
|----------|-------------|----------|
| `GITHUB_TOKEN` | GitHub personal access token | Yes |
| `GITHUB_ORGS` | Comma-separated list of organizations | Yes |
| `ELASTICSEARCH_URL` | Elasticsearch cluster URL | Yes |
| `ELASTICSEARCH_API_KEY` | Elasticsearch API key | Yes |
| `ES_INDEX_NAME` | Elasticsearch index name | No (default: `github-repos`) |

### Configuration Priority

Configuration priority (highest to lowest):
1. Command-line flags
2. Environment variables
3. Configuration file (YAML)

## Usage

### Using Config File (Recommended)

```bash
./indexer --config config.yaml
```

### Using Command-Line Flags

```bash
# Single organization
./indexer --orgs elastic \
  --github-token ghp_xxxx \
  --es-url https://your-cluster.es.cloud.es.io:443 \
  --es-api-key your-api-key

# Multiple organizations
./indexer --orgs elastic,kubernetes,golang \
  --github-token ghp_xxxx \
  --es-url https://your-cluster.es.cloud.es.io:443 \
  --es-api-key your-api-key
```

### Using Environment Variables

```bash
export GITHUB_TOKEN=ghp_xxxx
export GITHUB_ORGS=elastic,kubernetes
export ELASTICSEARCH_URL=https://your-cluster.es.cloud.es.io:443
export ELASTICSEARCH_API_KEY=your-api-key

./indexer
```

## Elasticsearch Index Structure

Each repository is stored as a document with the following structure:

```json
{
  "id": 123456789,
  "name": "elasticsearch",
  "full_name": "elastic/elasticsearch",
  "organization": "elastic",
  "description": "Free and Open, Distributed, RESTful Search Engine",
  "url": "https://github.com/elastic/elasticsearch",
  "tags": [
    {
      "name": "v8.11.0",
      "commit_sha": "abc123...",
      "commit_url": "https://api.github.com/repos/elastic/elasticsearch/commits/abc123..."
    }
  ],
  "tag_count": 150,
  "indexed_at": "2024-01-15T10:30:00Z"
}
```

### Example Queries

After indexing, you can query the data using Elasticsearch:

```bash
# Get all repositories for an organization
GET /github-repos/_search
{
  "query": {
    "term": { "organization": "elastic" }
  }
}

# Find repositories with a specific tag
GET /github-repos/_search
{
  "query": {
    "nested": {
      "path": "tags",
      "query": {
        "term": { "tags.name": "v8.11.0" }
      }
    }
  }
}

# Get repositories with most tags
GET /github-repos/_search
{
  "sort": [
    { "tag_count": "desc" }
  ],
  "size": 10
}

# Search repositories by name
GET /github-repos/_search
{
  "query": {
    "wildcard": { "name": "*search*" }
  }
}
```
