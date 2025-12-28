package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/alexisbouchez/wikigo/crawler"
	"github.com/alexisbouchez/wikigo/jsparser"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: testparser <npm-package-name>")
		os.Exit(1)
	}

	pkgName := os.Args[1]

	// Create NPM crawler
	npmCrawler, err := crawler.NewNPMCrawler(nil)
	if err != nil {
		log.Fatalf("Failed to create NPM crawler: %v", err)
	}
	defer npmCrawler.Close()

	// Fetch package
	pkg, err := npmCrawler.FetchPackage(pkgName)
	if err != nil {
		log.Fatalf("Failed to fetch package: %v", err)
	}

	fmt.Printf("Package: %s@%s\n", pkg.Name, pkg.Version)
	fmt.Printf("Description: %s\n", pkg.Description)
	fmt.Printf("Main: %s\n", pkg.Main)
	fmt.Printf("Types: %s\n", pkg.Types)

	// Download package
	pkgDir, err := npmCrawler.DownloadPackage(pkg)
	if err != nil {
		log.Fatalf("Failed to download package: %v", err)
	}
	defer os.RemoveAll(pkgDir)

	fmt.Printf("Downloaded to: %s\n", pkgDir)

	// List files
	fmt.Println("\nFiles in package:")
	err = filepath.Walk(pkgDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			ext := filepath.Ext(path)
			if ext == ".js" || ext == ".ts" || ext == ".jsx" || ext == ".tsx" {
				relPath, _ := filepath.Rel(pkgDir, path)
				fmt.Printf("  %s\n", relPath)
			}
		}
		return nil
	})
	if err != nil {
		log.Printf("Error walking directory: %v", err)
	}

	// Parse one JS file manually to test
	parser := jsparser.NewParser()
	var testFile string
	err = filepath.Walk(pkgDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".js" {
			testFile = path
			return filepath.SkipAll
		}
		return nil
	})

	if testFile != "" {
		fmt.Printf("\nParsing test file: %s\n", testFile)

		// Show file content
		content, err := os.ReadFile(testFile)
		if err != nil {
			log.Printf("Failed to read file: %v", err)
		} else {
			fmt.Println("\nFile content:")
			fmt.Println(string(content))
			fmt.Println("\n---")
		}

		symbols, err := parser.ParseFile(testFile)
		if err != nil {
			log.Printf("Parse error: %v", err)
		} else {
			fmt.Printf("Found %d symbols:\n", len(symbols))
			for i, sym := range symbols {
				if i < 10 {
					fmt.Printf("  %s %s (exported: %v)\n", sym.Kind, sym.Name, sym.Exported)
				}
			}
			if len(symbols) > 10 {
				fmt.Printf("  ... and %d more\n", len(symbols)-10)
			}
		}
	}
}
