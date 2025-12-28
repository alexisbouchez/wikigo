package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/alexisbouchez/wikigo/db"
)

func main() {
	dbPath := flag.String("db", "wikigo.db", "Database path")
	crateName := flag.String("crate", "", "Crate name to query")
	flag.Parse()

	database, err := db.Open(*dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	if *crateName != "" {
		// Query specific crate
		crate, err := database.GetRustCrate(*crateName)
		if err != nil {
			log.Fatalf("Failed to get crate: %v", err)
		}

		if crate == nil {
			fmt.Printf("Crate not found: %s\n", *crateName)
			return
		}

		fmt.Printf("Crate: %s\n", crate.Name)
		fmt.Printf("Version: %s\n", crate.Version)
		fmt.Printf("Description: %s\n", crate.Description)
		fmt.Printf("License: %s\n", crate.License)
		fmt.Printf("Downloads: %d\n", crate.Downloads)
		if len(crate.Authors) > 0 {
			fmt.Printf("Authors: %v\n", crate.Authors)
		}
		fmt.Printf("\n")

		// Query symbols
		symbols, err := database.SearchRustSymbols(crate.Name, 100)
		if err != nil {
			log.Fatalf("Failed to search symbols: %v", err)
		}

		fmt.Printf("Symbols (%d):\n", len(symbols))
		for _, sym := range symbols {
			publicStr := ""
			if sym.Public {
				publicStr = " [public]"
			}
			fmt.Printf("  %s %s (line %d)%s\n", sym.Kind, sym.Name, sym.Line, publicStr)
		}
	} else {
		// List all crates
		crates, err := database.SearchRustCrates("", 100)
		if err != nil {
			log.Fatalf("Failed to search crates: %v", err)
		}

		fmt.Printf("Rust Crates (%d):\n\n", len(crates))
		for _, crate := range crates {
			fmt.Printf("  %s@%s\n", crate.Name, crate.Version)
			if crate.Description != "" {
				fmt.Printf("    %s\n", crate.Description)
			}
			if crate.Downloads > 0 {
				fmt.Printf("    Downloads: %d\n", crate.Downloads)
			}
			fmt.Println()
		}
	}
}
