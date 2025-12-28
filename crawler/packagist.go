package crawler

import (
	"archive/zip"
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
	"github.com/alexisbouchez/wikigo/phpparser"
)

const (
	PackagistAPI = "https://repo.packagist.org/p2"
)

// PackagistPackage represents the Packagist JSON API response
type PackagistPackage struct {
	Packages map[string][]PackagistVersion `json:"packages"`
}

// PackagistVersion represents a specific version of a package
type PackagistVersion struct {
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Description string            `json:"description"`
	Type        string            `json:"type"`
	License     []string          `json:"license"`
	Homepage    string            `json:"homepage"`
	Source      PackagistSource   `json:"source"`
	Dist        PackagistDist     `json:"dist"`
	Authors     []PackagistAuthor `json:"authors"`
	Keywords    []string          `json:"keywords"`
	Require     map[string]string `json:"require"`
	Time        string            `json:"time"`
}

// PackagistSource represents source repository info
type PackagistSource struct {
	Type string `json:"type"`
	URL  string `json:"url"`
	Ref  string `json:"reference"`
}

// PackagistDist represents distribution archive info
type PackagistDist struct {
	Type string `json:"type"`
	URL  string `json:"url"`
	Ref  string `json:"reference"`
}

// PackagistAuthor represents an author
type PackagistAuthor struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// PackagistCrawler fetches and indexes packages from Packagist
type PackagistCrawler struct {
	db        *db.DB
	client    *http.Client
	parser    *phpparser.Parser
	tempDir   string
	rateLimit time.Duration
}

// NewPackagistCrawler creates a new Packagist crawler
func NewPackagistCrawler(database *db.DB) (*PackagistCrawler, error) {
	tempDir, err := os.MkdirTemp("", "packagist-crawler-*")
	if err != nil {
		return nil, fmt.Errorf("creating temp dir: %w", err)
	}

	return &PackagistCrawler{
		db:        database,
		client:    &http.Client{Timeout: 60 * time.Second},
		parser:    phpparser.NewParser(),
		tempDir:   tempDir,
		rateLimit: 200 * time.Millisecond,
	}, nil
}

// Close cleans up resources
func (c *PackagistCrawler) Close() error {
	return os.RemoveAll(c.tempDir)
}

// FetchPackage fetches package metadata from Packagist
func (c *PackagistCrawler) FetchPackage(name string) (*PackagistVersion, error) {
	time.Sleep(c.rateLimit)

	// Packagist API uses vendor/package format
	url := fmt.Sprintf("%s/%s.json", PackagistAPI, name)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("User-Agent", "wikigo-crawler (github.com/alexisbouchez/wikigo)")
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching package metadata: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("package not found: %s (status %d)", name, resp.StatusCode)
	}

	var pkgResp PackagistPackage
	if err := json.NewDecoder(resp.Body).Decode(&pkgResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	// Get the latest stable version
	versions, ok := pkgResp.Packages[name]
	if !ok || len(versions) == 0 {
		return nil, fmt.Errorf("no versions found for %s", name)
	}

	// Packagist API returns versions sorted newest to oldest
	// Find the first non-dev version (which is the latest stable)
	var latest *PackagistVersion
	for i := range versions {
		v := &versions[i]
		// Skip dev versions
		if strings.Contains(v.Version, "dev") {
			continue
		}
		latest = v
		break
	}

	if latest == nil {
		// If no stable version, use the first one
		latest = &versions[0]
	}

	// The name field is only populated in the first entry, so set it from the package key
	if latest.Name == "" {
		latest.Name = name
	}

	return latest, nil
}

// DownloadPackage downloads and extracts a package
func (c *PackagistCrawler) DownloadPackage(pkg *PackagistVersion) (string, error) {
	if pkg.Dist.URL == "" {
		return "", fmt.Errorf("no distribution URL for %s", pkg.Name)
	}

	time.Sleep(c.rateLimit)

	req, err := http.NewRequest("GET", pkg.Dist.URL, nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", "wikigo-crawler (github.com/alexisbouchez/wikigo)")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("downloading package: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed: status %d", resp.StatusCode)
	}

	// Save to temp file
	safeName := strings.ReplaceAll(pkg.Name, "/", "-")
	tmpFile := filepath.Join(c.tempDir, safeName+".zip")
	f, err := os.Create(tmpFile)
	if err != nil {
		return "", fmt.Errorf("creating temp file: %w", err)
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		return "", fmt.Errorf("saving zip: %w", err)
	}
	f.Close()

	// Extract
	extractDir := filepath.Join(c.tempDir, safeName)
	if err := c.extractZip(tmpFile, extractDir); err != nil {
		return "", fmt.Errorf("extracting zip: %w", err)
	}
	os.Remove(tmpFile)

	return extractDir, nil
}

// extractZip extracts a .zip archive
func (c *PackagistCrawler) extractZip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	var totalSize int64
	const maxTotalSize = 100 * 1024 * 1024

	for _, f := range r.File {
		target := filepath.Join(dest, f.Name)
		if !strings.HasPrefix(target, filepath.Clean(dest)+string(os.PathSeparator)) {
			continue
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(target, 0755)
			continue
		}

		if f.UncompressedSize64 > 10*1024*1024 {
			continue
		}
		totalSize += int64(f.UncompressedSize64)
		if totalSize > maxTotalSize {
			return fmt.Errorf("archive too large")
		}

		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}

		outFile, err := os.Create(target)
		if err != nil {
			rc.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

// ParsePackageSymbols parses PHP files for symbols
func (c *PackagistCrawler) ParsePackageSymbols(pkgDir string) ([]phpparser.Symbol, error) {
	// Find the actual package directory (often nested in vendor/package-hash/)
	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		return nil, err
	}

	// If there's only one directory entry, go into it
	if len(entries) == 1 && entries[0].IsDir() {
		pkgDir = filepath.Join(pkgDir, entries[0].Name())
	}

	// Look for src directory
	srcDir := filepath.Join(pkgDir, "src")
	if info, err := os.Stat(srcDir); err == nil && info.IsDir() {
		pkgDir = srcDir
	}

	return c.parser.ParseDirectory(pkgDir)
}

// IndexPackage indexes a package from Packagist
func (c *PackagistCrawler) IndexPackage(name string) error {
	log.Printf("Fetching package metadata: %s", name)

	pkg, err := c.FetchPackage(name)
	if err != nil {
		return fmt.Errorf("fetching package: %w", err)
	}

	log.Printf("Downloading package: %s@%s", pkg.Name, pkg.Version)

	pkgDir, err := c.DownloadPackage(pkg)
	if err != nil {
		return fmt.Errorf("downloading package: %w", err)
	}
	defer os.RemoveAll(pkgDir)

	log.Printf("Parsing symbols...")

	symbols, err := c.ParsePackageSymbols(pkgDir)
	if err != nil {
		log.Printf("Warning: error parsing symbols: %v", err)
		symbols = nil
	}

	// Extract repository URL
	repoURL := pkg.Source.URL
	if repoURL != "" {
		repoURL = cleanRepoURL(repoURL)
	}

	// Extract license
	license := ""
	if len(pkg.License) > 0 {
		license = pkg.License[0]
	}

	// Extract authors
	var authors []string
	for _, a := range pkg.Authors {
		authors = append(authors, a.Name)
	}

	// Store package in database
	dbPkg := &db.PHPPackage{
		Name:          pkg.Name,
		Version:       pkg.Version,
		Description:   pkg.Description,
		Type:          pkg.Type,
		License:       license,
		Homepage:      pkg.Homepage,
		RepositoryURL: repoURL,
		PackagistURL:  fmt.Sprintf("https://packagist.org/packages/%s", pkg.Name),
		Authors:       authors,
		Keywords:      pkg.Keywords,
		Require:       pkg.Require,
	}

	pkgID, err := c.db.UpsertPHPPackage(dbPkg)
	if err != nil {
		return fmt.Errorf("storing package: %w", err)
	}

	// Delete old symbols
	if err := c.db.DeletePHPPackageSymbols(pkgID); err != nil {
		return fmt.Errorf("deleting old symbols: %w", err)
	}

	// Store symbols
	publicCount := 0
	for _, sym := range symbols {
		if !sym.Public {
			continue
		}

		dbSym := &db.PHPSymbol{
			Name:        sym.Name,
			Kind:        sym.Kind,
			Signature:   sym.Signature,
			PackageID:   pkgID,
			PackageName: pkg.Name,
			FilePath:    sym.FilePath,
			Line:        sym.Line,
			Public:      sym.Public,
			Doc:         sym.Doc,
		}

		if err := c.db.UpsertPHPSymbol(dbSym); err != nil {
			log.Printf("Warning: failed to store symbol %s: %v", sym.Name, err)
		} else {
			publicCount++
		}
	}

	log.Printf("Indexed %s: %d symbols (%d public)", pkg.Name, len(symbols), publicCount)

	return nil
}
