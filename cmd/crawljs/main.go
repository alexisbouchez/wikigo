package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/alexisbouchez/wikigo/crawler"
	"github.com/alexisbouchez/wikigo/db"
)

func main() {
	var (
		dbPath      = flag.String("db", "wikigo.db", "Database path")
		npmPackage  = flag.String("npm", "", "NPM package name to index")
		githubRepo  = flag.String("github", "", "GitHub repository (owner/repo) to index")
		githubToken = flag.String("token", os.Getenv("GITHUB_TOKEN"), "GitHub API token")
	)
	flag.Parse()

	if *npmPackage == "" && *githubRepo == "" {
		fmt.Println("Usage: crawljs -npm <package> OR -github <owner/repo>")
		fmt.Println("  -npm string")
		fmt.Println("        NPM package name to index")
		fmt.Println("  -github string")
		fmt.Println("        GitHub repository (owner/repo) to index")
		fmt.Println("  -token string")
		fmt.Println("        GitHub API token (default: $GITHUB_TOKEN)")
		fmt.Println("  -db string")
		fmt.Println("        Database path (default: wikigo.db)")
		os.Exit(1)
	}

	// Open database
	database, err := db.Open(*dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	if *npmPackage != "" {
		// Index NPM package
		log.Printf("Indexing NPM package: %s", *npmPackage)
		npmCrawler, err := crawler.NewNPMCrawler(database)
		if err != nil {
			log.Fatalf("Failed to create NPM crawler: %v", err)
		}
		defer npmCrawler.Close()

		if err := npmCrawler.IndexPackage(*npmPackage); err != nil {
			log.Fatalf("Failed to index package: %v", err)
		}

		log.Printf("Successfully indexed %s", *npmPackage)
	}

	if *githubRepo != "" {
		// Index GitHub repository
		log.Printf("Indexing GitHub repository: %s", *githubRepo)

		// Parse owner/repo
		var owner, repo string
		if _, err := fmt.Sscanf(*githubRepo, "%[^/]/%s", &owner, &repo); err != nil {
			log.Fatalf("Invalid repository format (expected owner/repo): %v", err)
		}

		githubCrawler, err := crawler.NewGitHubCrawler(database, *githubToken)
		if err != nil {
			log.Fatalf("Failed to create GitHub crawler: %v", err)
		}
		defer githubCrawler.Close()

		if err := githubCrawler.IndexRepository(owner, repo); err != nil {
			log.Fatalf("Failed to index repository: %v", err)
		}

		log.Printf("Successfully indexed %s/%s", owner, repo)
	}
}
