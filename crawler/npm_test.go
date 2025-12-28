package crawler

import (
	"testing"
)

func TestFetchPackage(t *testing.T) {
	crawler, err := NewNPMCrawler(nil)
	if err != nil {
		t.Fatalf("NewNPMCrawler() error = %v", err)
	}
	defer crawler.Close()

	// Test with a well-known package
	pkg, err := crawler.FetchPackage("express")
	if err != nil {
		t.Skipf("Skipping test: %v (network may be unavailable)", err)
	}

	if pkg.Name != "express" {
		t.Errorf("Expected package name 'express', got '%s'", pkg.Name)
	}

	if pkg.Version == "" {
		t.Error("Expected version to be set")
	}

	if pkg.Description == "" {
		t.Error("Expected description to be set")
	}

	if pkg.License == "" {
		t.Error("Expected license to be set")
	}
}

func TestSearchPackages(t *testing.T) {
	crawler, err := NewNPMCrawler(nil)
	if err != nil {
		t.Fatalf("NewNPMCrawler() error = %v", err)
	}
	defer crawler.Close()

	packages, err := crawler.SearchPackages("react", 10)
	if err != nil {
		t.Skipf("Skipping test: %v (network may be unavailable)", err)
	}

	if len(packages) == 0 {
		t.Error("Expected at least one package in search results")
	}

	// Check if react is in the results
	found := false
	for _, pkg := range packages {
		if pkg == "react" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected 'react' to be in search results")
	}
}

func TestDownloadPackage(t *testing.T) {
	t.Skip("Skipping download test to avoid network dependency")

	crawler, err := NewNPMCrawler(nil)
	if err != nil {
		t.Fatalf("NewNPMCrawler() error = %v", err)
	}
	defer crawler.Close()

	// Fetch a small package
	pkg, err := crawler.FetchPackage("is-number")
	if err != nil {
		t.Fatalf("FetchPackage() error = %v", err)
	}

	// Download it
	dir, err := crawler.DownloadPackage(pkg)
	if err != nil {
		t.Fatalf("DownloadPackage() error = %v", err)
	}

	if dir == "" {
		t.Error("Expected directory path to be returned")
	}
}
