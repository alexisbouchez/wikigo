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
		dbPath  = flag.String("db", "wikigo.db", "Database path")
		pkg     = flag.String("package", "", "Python package name to index")
	)
	flag.Parse()

	if *pkg == "" {
		fmt.Println("Usage: crawlpy -package <package-name>")
		fmt.Println("  -package string")
		fmt.Println("        Python package name to index")
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
	log.Printf("Indexing Python package: %s", *pkg)
	pypiCrawler, err := crawler.NewPyPICrawler(database)
	if err != nil {
		log.Fatalf("Failed to create PyPI crawler: %v", err)
	}
	defer pypiCrawler.Close()

	if err := pypiCrawler.IndexPackage(*pkg); err != nil {
		log.Fatalf("Failed to index package: %v", err)
	}

	log.Printf("Successfully indexed %s", *pkg)
}
