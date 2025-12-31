package indexer

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/user/ghrepo-indexer/internal/config"
	"github.com/user/ghrepo-indexer/internal/indexer"
)

var (
	cfgFile string
	cfg     *config.Config
)

var rootCmd = &cobra.Command{
	Use:   "indexer",
	Short: "GitHub Repository Indexer",
	Long: `A CLI tool that synchronizes GitHub organization repository data 
(repository names and tags) to Elasticsearch.`,
	SilenceUsage: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return initConfig()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return runIndexer()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "path to YAML config file")
	rootCmd.PersistentFlags().String("github-token", "", "GitHub personal access token (required)")
	rootCmd.PersistentFlags().StringSlice("orgs", nil, "GitHub organizations to index (comma-separated, required)")
	rootCmd.PersistentFlags().String("es-url", "", "Elasticsearch URL (required)")
	rootCmd.PersistentFlags().String("es-api-key", "", "Elasticsearch API key (required)")
	rootCmd.PersistentFlags().String("index", "github-repos", "Elasticsearch index name")

	// Bind flags to viper
	viper.BindPFlag("github_token", rootCmd.PersistentFlags().Lookup("github-token"))
	viper.BindPFlag("github_orgs", rootCmd.PersistentFlags().Lookup("orgs"))
	viper.BindPFlag("elasticsearch_url", rootCmd.PersistentFlags().Lookup("es-url"))
	viper.BindPFlag("elasticsearch_api_key", rootCmd.PersistentFlags().Lookup("es-api-key"))
	viper.BindPFlag("es_index_name", rootCmd.PersistentFlags().Lookup("index"))
}

func initConfig() error {
	viper.AutomaticEnv()

	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
		viper.SetConfigType("yaml")
		if err := viper.ReadInConfig(); err != nil {
			return fmt.Errorf("failed to read config file: %w", err)
		}
		log.Printf("Using config file: %s", viper.ConfigFileUsed())
	}

	// Build configuration
	cfg = config.DefaultConfig()
	cfg.GitHubToken = viper.GetString("github_token")
	if orgs := viper.GetStringSlice("github_orgs"); len(orgs) > 0 {
		cfg.GitHubOrgs = orgs
	} else if orgsStr := viper.GetString("github_orgs"); orgsStr != "" {
		cfg.GitHubOrgs = config.ParseOrganizations(orgsStr)
	}
	cfg.ElasticsearchURL = viper.GetString("elasticsearch_url")
	cfg.ElasticsearchAPIKey = viper.GetString("elasticsearch_api_key")
	if indexName := viper.GetString("es_index_name"); indexName != "" {
		cfg.IndexName = indexName
	}

	return nil
}

func runIndexer() error {
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}
	log.Printf("Configuration:")
	log.Printf("  Organizations: %v", cfg.GitHubOrgs)
	log.Printf("  Elasticsearch URL: %s", cfg.ElasticsearchURL)
	log.Printf("  Index Name: %s", cfg.IndexName)

	idx, err := indexer.NewIndexerFromConfig(cfg)
	if err != nil {
		return fmt.Errorf("failed to create indexer: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Printf("Received signal %v, shutting down...", sig)
		cancel()
	}()

	log.Println("Starting sync...")
	result, err := idx.Run(ctx)
	if err != nil {
		return err
	}
	fmt.Println("\n=== Sync Summary ===")
	fmt.Printf("Organizations: %d\n", result.TotalOrgs)
	fmt.Printf("Repositories:  %d\n", result.TotalRepos)
	fmt.Printf("Tags:          %d\n", result.TotalTags)
	fmt.Printf("Errors:        %d\n", result.TotalErrors)
	fmt.Printf("Duration:      %v\n", result.Duration)

	if result.TotalErrors > 0 {
		fmt.Println("\nErrors:")
		for _, org := range result.Organizations {
			for _, errMsg := range org.Errors {
				fmt.Printf("  - [%s] %s\n", org.Organization, errMsg)
			}
		}
	}

	return nil
}
