package crawler

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alexisbouchez/wikigo/db"
	"github.com/alexisbouchez/wikigo/rsparser"
)

const (
	CratesIOAPI = "https://crates.io/api/v1"
)

// CrateMetadata represents crate metadata from crates.io
type CrateMetadata struct {
	Crate struct {
		Name         string    `json:"name"`
		Description  string    `json:"description"`
		Documentation string   `json:"documentation"`
		Homepage     string    `json:"homepage"`
		Repository   string    `json:"repository"`
		MaxVersion   string    `json:"max_version"`
		Downloads    int       `json:"downloads"`
		RecentDownloads int    `json:"recent_downloads"`
		CreatedAt    time.Time `json:"created_at"`
		UpdatedAt    time.Time `json:"updated_at"`
	} `json:"crate"`
	Versions []struct {
		Num        string    `json:"num"`
		DL_Path    string    `json:"dl_path"`
		Downloads  int       `json:"downloads"`
		Yanked     bool      `json:"yanked"`
		License    string    `json:"license"`
		CreatedAt  time.Time `json:"created_at"`
	} `json:"versions"`
}

// CargoToml represents a simplified Cargo.toml
type CargoToml struct {
	Package struct {
		Name        string   `toml:"name"`
		Version     string   `toml:"version"`
		Authors     []string `toml:"authors"`
		Description string   `toml:"description"`
		License     string   `toml:"license"`
		Repository  string   `toml:"repository"`
		Homepage    string   `toml:"homepage"`
		Documentation string `toml:"documentation"`
		Keywords    []string `toml:"keywords"`
		Categories  []string `toml:"categories"`
	} `toml:"package"`
	Dependencies map[string]interface{} `toml:"dependencies"`
}

// CratesCrawler fetches and indexes crates from crates.io
type CratesCrawler struct {
	db        *db.DB
	client    *http.Client
	parser    *rsparser.Parser
	tempDir   string
	rateLimit time.Duration
}

// NewCratesCrawler creates a new crates.io crawler
func NewCratesCrawler(database *db.DB) (*CratesCrawler, error) {
	tempDir, err := os.MkdirTemp("", "crates-crawler-*")
	if err != nil {
		return nil, fmt.Errorf("creating temp dir: %w", err)
	}

	return &CratesCrawler{
		db:        database,
		client:    &http.Client{Timeout: 60 * time.Second},
		parser:    rsparser.NewParser(),
		tempDir:   tempDir,
		rateLimit: 200 * time.Millisecond, // crates.io rate limiting
	}, nil
}

// Close cleans up resources
func (c *CratesCrawler) Close() error {
	return os.RemoveAll(c.tempDir)
}

// FetchCrate fetches crate metadata from crates.io
func (c *CratesCrawler) FetchCrate(name string) (*CrateMetadata, error) {
	time.Sleep(c.rateLimit)

	url := fmt.Sprintf("%s/crates/%s", CratesIOAPI, name)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// crates.io requires User-Agent header
	req.Header.Set("User-Agent", "wikigo-crawler (github.com/alexisbouchez/wikigo)")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching crate metadata: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("crate not found: %s (status %d)", name, resp.StatusCode)
	}

	var metadata CrateMetadata
	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &metadata, nil
}

// DownloadCrate downloads and extracts a crate
func (c *CratesCrawler) DownloadCrate(name, version string) (string, error) {
	time.Sleep(c.rateLimit)

	// crates.io download URL format
	url := fmt.Sprintf("https://crates.io/api/v1/crates/%s/%s/download", name, version)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", "wikigo-crawler (github.com/alexisbouchez/wikigo)")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("downloading crate: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed: status %d", resp.StatusCode)
	}

	// Extract to temp directory
	extractDir := filepath.Join(c.tempDir, name+"-"+version)
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		return "", fmt.Errorf("creating extract dir: %w", err)
	}

	// Decompress gzip
	gzr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return "", fmt.Errorf("creating gzip reader: %w", err)
	}
	defer gzr.Close()

	// Extract tar
	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("reading tar: %w", err)
		}

		// Skip non-files
		if header.Typeflag != tar.TypeReg {
			continue
		}

		// Remove "cratename-version/" prefix from tar paths
		parts := strings.SplitN(header.Name, "/", 2)
		targetPath := filepath.Join(extractDir, parts[len(parts)-1])

		// Create parent directories
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return "", fmt.Errorf("creating directories: %w", err)
		}

		// Write file
		outFile, err := os.Create(targetPath)
		if err != nil {
			return "", fmt.Errorf("creating file: %w", err)
		}

		if _, err := io.Copy(outFile, tr); err != nil {
			outFile.Close()
			return "", fmt.Errorf("writing file: %w", err)
		}
		outFile.Close()
	}

	return extractDir, nil
}

// ParseCrateSymbols parses Rust files and extracts symbols
func (c *CratesCrawler) ParseCrateSymbols(crateDir string) ([]rsparser.Symbol, error) {
	// Look for src directory
	srcDir := filepath.Join(crateDir, "src")
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		// Try parsing the crate directory directly
		srcDir = crateDir
	}

	symbols, err := c.parser.ParseDirectory(srcDir)
	if err != nil {
		return nil, fmt.Errorf("parsing crate directory: %w", err)
	}

	return symbols, nil
}

// IndexCrate indexes a crate into the database
func (c *CratesCrawler) IndexCrate(name string) error {
	log.Printf("Indexing crate: %s", name)

	// Fetch metadata
	metadata, err := c.FetchCrate(name)
	if err != nil {
		return fmt.Errorf("fetching crate: %w", err)
	}

	// Get latest non-yanked version
	var latestVersion string
	var license string
	for _, v := range metadata.Versions {
		if !v.Yanked {
			if latestVersion == "" {
				latestVersion = v.Num
				license = v.License
			}
		}
	}

	if latestVersion == "" {
		return fmt.Errorf("no non-yanked versions found")
	}

	// Download and extract
	crateDir, err := c.DownloadCrate(name, latestVersion)
	if err != nil {
		return fmt.Errorf("downloading crate: %w", err)
	}
	defer os.RemoveAll(crateDir)

	// Parse symbols
	symbols, err := c.ParseCrateSymbols(crateDir)
	if err != nil {
		return fmt.Errorf("parsing symbols: %w", err)
	}

	log.Printf("Found %d symbols in %s", len(symbols), name)

	// Store in database
	if c.db != nil {
		dbCrate := &db.RustCrate{
			Name:          metadata.Crate.Name,
			Version:       latestVersion,
			Description:   metadata.Crate.Description,
			License:       license,
			Repository:    metadata.Crate.Repository,
			Homepage:      metadata.Crate.Homepage,
			Documentation: metadata.Crate.Documentation,
			Downloads:     metadata.Crate.Downloads,
		}

		crateID, err := c.db.UpsertRustCrate(dbCrate)
		if err != nil {
			return fmt.Errorf("storing crate: %w", err)
		}

		// Delete old symbols
		if err := c.db.DeleteRustCrateSymbols(crateID); err != nil {
			return fmt.Errorf("deleting old symbols: %w", err)
		}

		// Store symbols
		publicCount := 0
		for _, sym := range symbols {
			dbSym := &db.RustSymbol{
				Name:      sym.Name,
				Kind:      sym.Kind,
				Signature: sym.Signature,
				CrateID:   crateID,
				CrateName: name,
				FilePath:  sym.FilePath,
				Line:      sym.Line,
				Public:    sym.Public,
				Doc:       sym.Doc,
			}

			if err := c.db.UpsertRustSymbol(dbSym); err != nil {
				log.Printf("Warning: failed to store symbol %s: %v", sym.Name, err)
			}

			if sym.Public {
				publicCount++
			}
		}

		log.Printf("Stored %d symbols (%d public) in database", len(symbols), publicCount)
	}

	return nil
}

// SearchCrates searches crates.io for crates
func (c *CratesCrawler) SearchCrates(query string, limit int) ([]string, error) {
	time.Sleep(c.rateLimit)

	url := fmt.Sprintf("%s/crates?q=%s&per_page=%d", CratesIOAPI, query, limit)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", "wikigo-crawler (github.com/alexisbouchez/wikigo)")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("searching crates: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Crates []struct {
			Name string `json:"name"`
		} `json:"crates"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding search results: %w", err)
	}

	var crates []string
	for _, crate := range result.Crates {
		crates = append(crates, crate.Name)
	}

	return crates, nil
}
