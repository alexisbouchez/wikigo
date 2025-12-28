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
		dbPath = flag.String("db", "wikigo.db", "Database path")
		pkg    = flag.String("package", "", "PHP package name to index (vendor/package)")
	)
	flag.Parse()

	if *pkg == "" {
		fmt.Println("Usage: crawlphp -package <vendor/package>")
		fmt.Println("  -package string")
		fmt.Println("        PHP package name to index (e.g., laravel/framework)")
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

	// Index package
	log.Printf("Indexing PHP package: %s", *pkg)
	packagistCrawler, err := crawler.NewPackagistCrawler(database)
	if err != nil {
		log.Fatalf("Failed to create Packagist crawler: %v", err)
	}
	defer packagistCrawler.Close()

	if err := packagistCrawler.IndexPackage(*pkg); err != nil {
		log.Fatalf("Failed to index package: %v", err)
	}

	log.Printf("Successfully indexed %s", *pkg)
}
