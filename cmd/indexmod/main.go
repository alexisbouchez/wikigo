package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/alexisbouchez/wikigo/crawler"
)

func main() {
	dbPath := flag.String("db", "wikigo.db", "SQLite database path")
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: indexmod [-db path] <module-path> [version]\n")
		fmt.Fprintf(os.Stderr, "Example: indexmod github.com/valyentdev/ravel v0.7.2\n")
		os.Exit(1)
	}

	modulePath := args[0]
	version := ""
	if len(args) > 1 {
		version = args[1]
	} else {
		// Fetch latest version
		var err error
		version, err = fetchLatestVersion(modulePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error fetching latest version: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Using latest version: %s\n", version)
	}

	cfg := crawler.Config{
		DBPath:    *dbPath,
		Workers:   1,
		RateLimit: 100 * time.Millisecond,
	}

	c, err := crawler.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating crawler: %v\n", err)
		os.Exit(1)
	}
	defer c.Close()

	ctx := context.Background()

	fmt.Printf("Indexing %s@%s...\n", modulePath, version)

	mv := crawler.ModuleVersion{
		Path:      modulePath,
		Version:   version,
		Timestamp: time.Now(),
	}

	if err := c.ProcessModulePublic(ctx, mv); err != nil {
		fmt.Fprintf(os.Stderr, "Error indexing module: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Done!")
}

func fetchLatestVersion(modulePath string) (string, error) {
	url := fmt.Sprintf("https://proxy.golang.org/%s/@latest", escapeModulePath(modulePath))

	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Version string `json:"Version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.Version, nil
}

func escapeModulePath(path string) string {
	var result string
	for _, r := range path {
		if r >= 'A' && r <= 'Z' {
			result += "!" + string(r+32)
		} else {
			result += string(r)
		}
	}
	return result
}
