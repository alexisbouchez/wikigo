package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/alexisbouchez/wikigo/web"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP server address")
	dataDir := flag.String("data", ".", "Directory containing JSON documentation files")
	flag.Parse()

	if _, err := os.Stat(*dataDir); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: data directory %q does not exist\n", *dataDir)
		os.Exit(1)
	}

	server, err := web.NewServer(*dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating server: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Starting wikigo server at http://localhost%s\n", *addr)
	fmt.Printf("Data directory: %s\n", *dataDir)

	if err := server.ListenAndServe(*addr); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting server: %v\n", err)
		os.Exit(1)
	}
}
