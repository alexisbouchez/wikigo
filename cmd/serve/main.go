package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/alexisbouchez/wikigo/web"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP server address")
	dataDir := flag.String("data", ".", "Directory containing JSON documentation files")
	dbPath := flag.String("db", "", "SQLite database path (enables indexing features)")
	flag.Parse()

	if _, err := os.Stat(*dataDir); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: data directory %q does not exist\n", *dataDir)
		os.Exit(1)
	}

	server, err := web.NewServerWithDB(*dataDir, *dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating server: %v\n", err)
		os.Exit(1)
	}
	defer server.Close()

	// Handle shutdown gracefully
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		fmt.Println("\nShutting down...")
		server.Close()
		os.Exit(0)
	}()

	fmt.Printf("Starting wikigo server at http://localhost%s\n", *addr)
	fmt.Printf("Data directory: %s\n", *dataDir)
	if *dbPath != "" {
		pkgCount, symCount, impCount := server.GetDBStats()
		fmt.Printf("Database: %s (%d packages, %d symbols, %d imports)\n", *dbPath, pkgCount, symCount, impCount)
	}

	if err := server.ListenAndServe(*addr); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting server: %v\n", err)
		os.Exit(1)
	}
}
