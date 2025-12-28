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
		crate  = flag.String("crate", "", "Crate name to index")
	)
	flag.Parse()

	if *crate == "" {
		fmt.Println("Usage: crawlrs -crate <crate-name>")
		fmt.Println("  -crate string")
		fmt.Println("        Crate name to index")
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

	// Index crate
	log.Printf("Indexing Rust crate: %s", *crate)
	cratesCrawler, err := crawler.NewCratesCrawler(database)
	if err != nil {
		log.Fatalf("Failed to create crates crawler: %v", err)
	}
	defer cratesCrawler.Close()

	if err := cratesCrawler.IndexCrate(*crate); err != nil {
		log.Fatalf("Failed to index crate: %v", err)
	}

	log.Printf("Successfully indexed %s", *crate)
}
