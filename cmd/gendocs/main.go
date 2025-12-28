package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path/filepath"

	"github.com/alexisbouchez/wikigo/ai"
	"github.com/alexisbouchez/wikigo/db"
)

func main() {
	var (
		dbPath      = flag.String("db", "wikigo.db", "Path to SQLite database")
		packagePath = flag.String("pkg", "", "Package import path")
		sourceDir   = flag.String("src", "", "Source directory to analyze")
		dryRun      = flag.Bool("dry-run", false, "Print results without saving to database")
	)
	flag.Parse()

	if *packagePath == "" || *sourceDir == "" {
		fmt.Fprintf(os.Stderr, "Usage: gendocs -pkg <import-path> -src <source-dir> [-db <db-path>] [-dry-run]\n")
		os.Exit(1)
	}

	// Initialize AI service
	service := ai.NewServiceFromEnv()
	service.SetBudget(5.0, 100.0) // $5/day, $100/month

	// Enable auto-comment feature
	service.IsEnabled(ai.FlagAutoComments)

	// Open database
	database, err := db.Open(*dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	// Parse source directory
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, *sourceDir, nil, parser.ParseComments)
	if err != nil {
		log.Fatalf("Failed to parse directory: %v", err)
	}

	if len(pkgs) == 0 {
		log.Fatalf("No Go packages found in %s", *sourceDir)
	}

	// Process each package
	for pkgName, pkg := range pkgs {
		// Skip test packages
		if filepath.Ext(pkgName) == "_test" {
			continue
		}

		log.Printf("Analyzing package: %s", pkgName)

		// Convert map to slice
		var files []*ast.File
		for _, file := range pkg.Files {
			files = append(files, file)
		}

		analyzer := ai.NewDocumentationAnalyzer(fset)

		// Check if package needs synopsis
		needsSynopsis := analyzer.FindUncommentedPackage(files)
		if needsSynopsis {
			log.Printf("Package %s needs synopsis", pkgName)
			exportedSymbols := analyzer.ExtractExportedSymbols(files)

			synopsis, err := service.GeneratePackageSynopsis(pkgName, exportedSymbols)
			if err != nil {
				log.Printf("Error generating synopsis for package %s: %v", pkgName, err)
			} else {
				if *dryRun {
					fmt.Printf("\n// Package %s synopsis:\n// %s\n\n", pkgName, synopsis)
				} else {
					aiDoc := &db.AIDoc{
						SymbolName:   pkgName,
						SymbolKind:   "package",
						ImportPath:   *packagePath,
						GeneratedDoc: synopsis,
					}
					if err := database.UpsertAIDoc(aiDoc); err != nil {
						log.Printf("Error saving synopsis for %s: %v", pkgName, err)
					} else {
						log.Printf("✓ Saved synopsis for package %s", pkgName)
					}
				}
			}
		}

		// Find uncommented symbols
		functions := analyzer.FindUncommentedFunctions(pkgName, files)
		types := analyzer.FindUncommentedTypes(files)
		methods := analyzer.FindUncommentedMethods(files)

		log.Printf("Found %d uncommented functions, %d types, %d methods",
			len(functions), len(types), len(methods))

		// Generate documentation for functions
		for _, fn := range functions {
			log.Printf("Generating doc for function %s...", fn.Name)

			doc, err := service.GenerateFunctionComment(fn.Signature, fn.Body)
			if err != nil {
				log.Printf("Error generating doc for %s: %v", fn.Name, err)
				continue
			}

			if *dryRun {
				fmt.Printf("\n// %s\n%s\n", doc, fn.Signature)
			} else {
				aiDoc := &db.AIDoc{
					SymbolName:   fn.Name,
					SymbolKind:   "func",
					ImportPath:   *packagePath,
					GeneratedDoc: doc,
				}
				if err := database.UpsertAIDoc(aiDoc); err != nil {
					log.Printf("Error saving doc for %s: %v", fn.Name, err)
				} else {
					log.Printf("✓ Saved doc for %s", fn.Name)
				}
			}
		}

		// Generate documentation for types
		for _, typ := range types {
			log.Printf("Generating doc for type %s...", typ.Name)

			doc, err := service.GenerateTypeComment(typ.Name, typ.Body)
			if err != nil {
				log.Printf("Error generating doc for %s: %v", typ.Name, err)
				continue
			}

			if *dryRun {
				fmt.Printf("\n// %s\ntype %s %s\n", doc, typ.Name, typ.Body)
			} else {
				aiDoc := &db.AIDoc{
					SymbolName:   typ.Name,
					SymbolKind:   "type",
					ImportPath:   *packagePath,
					GeneratedDoc: doc,
				}
				if err := database.UpsertAIDoc(aiDoc); err != nil {
					log.Printf("Error saving doc for %s: %v", typ.Name, err)
				} else {
					log.Printf("✓ Saved doc for %s", typ.Name)
				}
			}
		}

		// Generate documentation for methods
		for _, method := range methods {
			log.Printf("Generating doc for method %s...", method.Name)

			doc, err := service.GenerateMethodComment("", method.Name, method.Signature, method.Body)
			if err != nil {
				log.Printf("Error generating doc for %s: %v", method.Name, err)
				continue
			}

			if *dryRun {
				fmt.Printf("\n// %s\n%s\n", doc, method.Signature)
			} else {
				aiDoc := &db.AIDoc{
					SymbolName:   method.Name,
					SymbolKind:   "method",
					ImportPath:   *packagePath,
					GeneratedDoc: doc,
				}
				if err := database.UpsertAIDoc(aiDoc); err != nil {
					log.Printf("Error saving doc for %s: %v", method.Name, err)
				} else {
					log.Printf("✓ Saved doc for %s", method.Name)
				}
			}
		}
	}

	// Print statistics
	stats := service.GetStats()
	fmt.Printf("\n=== Statistics ===\n")
	fmt.Printf("Total requests: %v\n", stats["total_requests"])
	fmt.Printf("Total cost: $%.4f\n", stats["total_cost_usd"])
	fmt.Printf("Cache hit rate: %.1f%%\n", stats["cache_hit_rate"])
	fmt.Printf("Budget used (daily): $%.4f / $%.2f\n", stats["budget_daily_used"], stats["budget_daily_max"])

	if !*dryRun {
		totalDocs, approvedDocs, flaggedDocs, totalCost, err := database.GetAIDocStats()
		if err != nil {
			log.Printf("Error getting doc stats: %v", err)
		} else {
			fmt.Printf("\n=== Database Stats ===\n")
			fmt.Printf("Total AI docs: %d\n", totalDocs)
			fmt.Printf("Approved: %d\n", approvedDocs)
			fmt.Printf("Flagged: %d\n", flaggedDocs)
			fmt.Printf("Total cost in DB: $%.4f\n", totalCost)
		}
	}
}
