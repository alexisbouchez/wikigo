package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alexisbouchez/wikigo/crawler"
)

func main() {
	dbPath := flag.String("db", "wikigo.db", "SQLite database path")
	workers := flag.Int("workers", 4, "Number of concurrent workers")
	rateLimit := flag.Duration("rate", 100*time.Millisecond, "Rate limit between requests per worker")
	sinceStr := flag.String("since", "", "Only fetch modules updated since this time (RFC3339 format)")
	maxModules := flag.Int("max", 0, "Maximum number of modules to process (0 = unlimited)")
	tempDir := flag.String("temp", "", "Temporary directory for downloads (default: system temp)")
	flag.Parse()

	var since time.Time
	if *sinceStr != "" {
		var err error
		since, err = time.Parse(time.RFC3339, *sinceStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing since time: %v\n", err)
			fmt.Fprintf(os.Stderr, "Expected format: 2024-01-01T00:00:00Z\n")
			os.Exit(1)
		}
	}

	cfg := crawler.Config{
		DBPath:     *dbPath,
		Workers:    *workers,
		RateLimit:  *rateLimit,
		Since:      since,
		MaxModules: *maxModules,
		TempDir:    *tempDir,
	}

	c, err := crawler.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating crawler: %v\n", err)
		os.Exit(1)
	}
	defer c.Close()

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nReceived interrupt, shutting down gracefully...")
		cancel()
	}()

	fmt.Println("=== wikigo Crawler ===")
	fmt.Printf("Database: %s\n", *dbPath)
	fmt.Printf("Workers: %d\n", *workers)
	fmt.Printf("Rate limit: %v\n", *rateLimit)
	if !since.IsZero() {
		fmt.Printf("Since: %s\n", since.Format(time.RFC3339))
	}
	if *maxModules > 0 {
		fmt.Printf("Max modules: %d\n", *maxModules)
	}
	fmt.Println()

	if err := c.Run(ctx, since); err != nil {
		if err == context.Canceled {
			fmt.Println("Crawl cancelled")
		} else {
			fmt.Fprintf(os.Stderr, "Error running crawler: %v\n", err)
			os.Exit(1)
		}
	}
}
