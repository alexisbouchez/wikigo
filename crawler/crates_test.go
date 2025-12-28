package crawler

import (
	"testing"
)

func TestFetchCrate(t *testing.T) {
	crawler, err := NewCratesCrawler(nil)
	if err != nil {
		t.Fatalf("NewCratesCrawler() error = %v", err)
	}
	defer crawler.Close()

	// Test with a well-known crate
	metadata, err := crawler.FetchCrate("serde")
	if err != nil {
		t.Skipf("Skipping test: %v (network may be unavailable)", err)
	}

	if metadata.Crate.Name != "serde" {
		t.Errorf("Expected crate name 'serde', got '%s'", metadata.Crate.Name)
	}

	if metadata.Crate.MaxVersion == "" {
		t.Error("Expected max version to be set")
	}

	if metadata.Crate.Description == "" {
		t.Error("Expected description to be set")
	}

	if len(metadata.Versions) == 0 {
		t.Error("Expected at least one version")
	}
}

func TestSearchCrates(t *testing.T) {
	crawler, err := NewCratesCrawler(nil)
	if err != nil {
		t.Fatalf("NewCratesCrawler() error = %v", err)
	}
	defer crawler.Close()

	crates, err := crawler.SearchCrates("tokio", 10)
	if err != nil {
		t.Skipf("Skipping test: %v (network may be unavailable)", err)
	}

	if len(crates) == 0 {
		t.Error("Expected at least one crate in search results")
	}

	// Check if tokio is in the results
	found := false
	for _, crate := range crates {
		if crate == "tokio" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected 'tokio' to be in search results")
	}
}

func TestDownloadCrate(t *testing.T) {
	t.Skip("Skipping download test to avoid network dependency")

	crawler, err := NewCratesCrawler(nil)
	if err != nil {
		t.Fatalf("NewCratesCrawler() error = %v", err)
	}
	defer crawler.Close()

	// Download a small crate
	dir, err := crawler.DownloadCrate("lazy_static", "1.4.0")
	if err != nil {
		t.Fatalf("DownloadCrate() error = %v", err)
	}

	if dir == "" {
		t.Error("Expected directory path to be returned")
	}
}
