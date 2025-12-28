package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/alexisbouchez/wikigo/db"
)

func main() {
	dbPath := flag.String("db", "wikigo.db", "Database path")
	pkgName := flag.String("pkg", "", "Package name to query")
	flag.Parse()

	database, err := db.Open(*dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	if *pkgName != "" {
		// Query specific package
		pkg, err := database.GetJSPackage(*pkgName)
		if err != nil {
			log.Fatalf("Failed to get package: %v", err)
		}

		fmt.Printf("Package: %s\n", pkg.Name)
		fmt.Printf("Version: %s\n", pkg.Version)
		fmt.Printf("Description: %s\n", pkg.Description)
		fmt.Printf("License: %s\n", pkg.License)
		fmt.Printf("Author: %s\n", pkg.Author)
		fmt.Printf("\n")

		// Query symbols
		symbols, err := database.SearchJSSymbols(pkg.Name, 100)
		if err != nil {
			log.Fatalf("Failed to search symbols: %v", err)
		}

		fmt.Printf("Symbols (%d):\n", len(symbols))
		for _, sym := range symbols {
			exportedStr := ""
			if sym.Exported {
				exportedStr = " [exported]"
			}
			fmt.Printf("  %s %s (line %d)%s\n", sym.Kind, sym.Name, sym.Line, exportedStr)
		}
	} else {
		// List all packages
		packages, err := database.SearchJSPackages("", 100)
		if err != nil {
			log.Fatalf("Failed to search packages: %v", err)
		}

		fmt.Printf("JavaScript/TypeScript Packages (%d):\n\n", len(packages))
		for _, pkg := range packages {
			fmt.Printf("  %s@%s\n", pkg.Name, pkg.Version)
			if pkg.Description != "" {
				fmt.Printf("    %s\n", pkg.Description)
			}
			fmt.Println()
		}
	}
}
