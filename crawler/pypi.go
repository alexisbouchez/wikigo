package crawler

import (
	"archive/tar"
	"archive/zip"
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
	"github.com/alexisbouchez/wikigo/pyparser"
)

const (
	PyPIRegistryURL = "https://pypi.org/pypi"
)

// cleanLicense extracts a short license name from the license field or classifiers
func cleanPyPILicense(license string, classifiers []string) string {
	// First try to extract from classifiers (more reliable)
	for _, c := range classifiers {
		if strings.HasPrefix(c, "License :: OSI Approved :: ") {
			// Extract license name from classifier
			name := strings.TrimPrefix(c, "License :: OSI Approved :: ")
			// Shorten common license names
			name = strings.TrimSuffix(name, " License")
			return name
		}
	}

	// Fall back to license field, but clean it up
	if license == "" {
		return ""
	}

	// Take only the first line
	if idx := strings.Index(license, "\n"); idx != -1 {
		license = strings.TrimSpace(license[:idx])
	}

	// Truncate if too long (likely full license text)
	if len(license) > 50 {
		// Try to find a common license name at the start
		commonLicenses := []string{"MIT", "BSD", "Apache", "GPL", "LGPL", "MPL", "ISC", "Unlicense"}
		for _, l := range commonLicenses {
			if strings.Contains(strings.ToUpper(license), l) {
				return l
			}
		}
		return license[:50] + "..."
	}

	return license
}

// PyPIPackageInfo represents the info section of PyPI JSON API response
type PyPIPackageInfo struct {
	Name            string            `json:"name"`
	Version         string            `json:"version"`
	Summary         string            `json:"summary"`
	Description     string            `json:"description"`
	Author          string            `json:"author"`
	AuthorEmail     string            `json:"author_email"`
	License         string            `json:"license"`
	HomePage        string            `json:"home_page"`
	ProjectURL      string            `json:"project_url"`
	RequiresPython  string            `json:"requires_python"`
	Keywords        string            `json:"keywords"`
	Classifiers     []string          `json:"classifiers"`
	ProjectURLs     map[string]string `json:"project_urls"`
	RequiresDist    []string          `json:"requires_dist"`
}

// PyPIRelease represents a release file from PyPI
type PyPIRelease struct {
	Filename    string `json:"filename"`
	URL         string `json:"url"`
	PackageType string `json:"packagetype"`
	Size        int64  `json:"size"`
}

// PyPIResponse represents the PyPI JSON API response
type PyPIResponse struct {
	Info     PyPIPackageInfo        `json:"info"`
	Releases map[string][]PyPIRelease `json:"releases"`
	URLs     []PyPIRelease          `json:"urls"`
}

// PyPICrawler fetches and indexes packages from PyPI
type PyPICrawler struct {
	db        *db.DB
	client    *http.Client
	parser    *pyparser.Parser
	tempDir   string
	rateLimit time.Duration
}

// NewPyPICrawler creates a new PyPI crawler
func NewPyPICrawler(database *db.DB) (*PyPICrawler, error) {
	tempDir, err := os.MkdirTemp("", "pypi-crawler-*")
	if err != nil {
		return nil, fmt.Errorf("creating temp dir: %w", err)
	}

	return &PyPICrawler{
		db:        database,
		client:    &http.Client{Timeout: 60 * time.Second},
		parser:    pyparser.NewParser(),
		tempDir:   tempDir,
		rateLimit: 200 * time.Millisecond,
	}, nil
}

// Close cleans up resources
func (c *PyPICrawler) Close() error {
	return os.RemoveAll(c.tempDir)
}

// FetchPackage fetches package metadata from PyPI
func (c *PyPICrawler) FetchPackage(name string) (*PyPIResponse, error) {
	time.Sleep(c.rateLimit)

	url := fmt.Sprintf("%s/%s/json", PyPIRegistryURL, name)
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

	var pypiResp PyPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&pypiResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &pypiResp, nil
}

// DownloadPackage downloads and extracts a package source distribution
func (c *PyPICrawler) DownloadPackage(pkg *PyPIResponse) (string, error) {
	// Find the source distribution (sdist)
	var sdistURL string
	var filename string

	for _, release := range pkg.URLs {
		if release.PackageType == "sdist" {
			sdistURL = release.URL
			filename = release.Filename
			break
		}
	}

	// If no sdist in current version, try to find any .tar.gz
	if sdistURL == "" {
		for _, release := range pkg.URLs {
			if strings.HasSuffix(release.Filename, ".tar.gz") {
				sdistURL = release.URL
				filename = release.Filename
				break
			}
		}
	}

	if sdistURL == "" {
		return "", fmt.Errorf("no source distribution found for %s", pkg.Info.Name)
	}

	time.Sleep(c.rateLimit)

	req, err := http.NewRequest("GET", sdistURL, nil)
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

	extractDir := filepath.Join(c.tempDir, pkg.Info.Name+"-"+pkg.Info.Version)
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		return "", fmt.Errorf("creating extract dir: %w", err)
	}

	// Extract based on file type
	if strings.HasSuffix(filename, ".tar.gz") || strings.HasSuffix(filename, ".tgz") {
		if err := c.extractTarGz(resp.Body, extractDir); err != nil {
			return "", fmt.Errorf("extracting tar.gz: %w", err)
		}
	} else if strings.HasSuffix(filename, ".zip") {
		// For zip files, we need to save first then extract
		tmpFile := filepath.Join(c.tempDir, filename)
		f, err := os.Create(tmpFile)
		if err != nil {
			return "", fmt.Errorf("creating temp file: %w", err)
		}
		if _, err := io.Copy(f, resp.Body); err != nil {
			f.Close()
			return "", fmt.Errorf("saving zip: %w", err)
		}
		f.Close()

		if err := c.extractZip(tmpFile, extractDir); err != nil {
			return "", fmt.Errorf("extracting zip: %w", err)
		}
		os.Remove(tmpFile)
	} else {
		return "", fmt.Errorf("unsupported archive format: %s", filename)
	}

	return extractDir, nil
}

// extractTarGz extracts a .tar.gz archive
func (c *PyPICrawler) extractTarGz(r io.Reader, dest string) error {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	var totalSize int64
	const maxTotalSize = 100 * 1024 * 1024 // 100MB limit

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Security check: prevent path traversal
		target := filepath.Join(dest, header.Name)
		if !strings.HasPrefix(target, filepath.Clean(dest)+string(os.PathSeparator)) {
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			// Size limits
			if header.Size > 10*1024*1024 { // 10MB per file
				continue
			}
			totalSize += header.Size
			if totalSize > maxTotalSize {
				return fmt.Errorf("archive too large")
			}

			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}

			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}

			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		}
	}

	return nil
}

// extractZip extracts a .zip archive
func (c *PyPICrawler) extractZip(src, dest string) error {
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

// ParsePackageSymbols parses Python files for symbols
func (c *PyPICrawler) ParsePackageSymbols(pkgDir string) ([]pyparser.Symbol, error) {
	// Find the actual package directory
	// Python packages can have different layouts:
	// 1. package-version/package/*.py (standard)
	// 2. package-version/src/package/*.py (src layout)
	// 3. package-version/*.py (flat)

	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		return nil, err
	}

	// If there's only one directory entry, go into it
	if len(entries) == 1 && entries[0].IsDir() {
		pkgDir = filepath.Join(pkgDir, entries[0].Name())
	}

	// Check for src layout
	srcDir := filepath.Join(pkgDir, "src")
	if info, err := os.Stat(srcDir); err == nil && info.IsDir() {
		pkgDir = srcDir
	}

	return c.parser.ParseDirectory(pkgDir)
}

// IndexPackage indexes a package from PyPI
func (c *PyPICrawler) IndexPackage(name string) error {
	log.Printf("Fetching package metadata: %s", name)

	pkg, err := c.FetchPackage(name)
	if err != nil {
		return fmt.Errorf("fetching package: %w", err)
	}

	log.Printf("Downloading package: %s@%s", pkg.Info.Name, pkg.Info.Version)

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

	// Extract repository URL from project_urls
	repoURL := ""
	docURL := ""
	if pkg.Info.ProjectURLs != nil {
		for key, url := range pkg.Info.ProjectURLs {
			keyLower := strings.ToLower(key)
			if strings.Contains(keyLower, "repository") || strings.Contains(keyLower, "source") ||
				strings.Contains(keyLower, "github") || strings.Contains(keyLower, "gitlab") {
				repoURL = url
			}
			if strings.Contains(keyLower, "documentation") || strings.Contains(keyLower, "docs") {
				docURL = url
			}
		}
	}

	// Parse keywords
	var keywords []string
	if pkg.Info.Keywords != "" {
		for _, kw := range strings.Split(pkg.Info.Keywords, ",") {
			kw = strings.TrimSpace(kw)
			if kw != "" {
				keywords = append(keywords, kw)
			}
		}
	}

	// Store package in database
	dbPkg := &db.PythonPackage{
		Name:             pkg.Info.Name,
		Version:          pkg.Info.Version,
		Summary:          pkg.Info.Summary,
		Author:           pkg.Info.Author,
		AuthorEmail:      pkg.Info.AuthorEmail,
		License:          cleanPyPILicense(pkg.Info.License, pkg.Info.Classifiers),
		HomePage:         pkg.Info.HomePage,
		ProjectURL:       pkg.Info.ProjectURL,
		PyPIURL:          fmt.Sprintf("https://pypi.org/project/%s/", pkg.Info.Name),
		RepositoryURL:    repoURL,
		DocumentationURL: docURL,
		RequiresPython:   pkg.Info.RequiresPython,
		Keywords:         keywords,
		Classifiers:      pkg.Info.Classifiers,
		Dependencies:     pkg.Info.RequiresDist,
	}

	pkgID, err := c.db.UpsertPythonPackage(dbPkg)
	if err != nil {
		return fmt.Errorf("storing package: %w", err)
	}

	// Delete old symbols
	if err := c.db.DeletePythonPackageSymbols(pkgID); err != nil {
		return fmt.Errorf("deleting old symbols: %w", err)
	}

	// Store symbols
	publicCount := 0
	for _, sym := range symbols {
		if !sym.Public {
			continue
		}

		dbSym := &db.PythonSymbol{
			Name:        sym.Name,
			Kind:        sym.Kind,
			Signature:   sym.Signature,
			PackageID:   pkgID,
			PackageName: pkg.Info.Name,
			FilePath:    sym.FilePath,
			Line:        sym.Line,
			Public:      sym.Public,
			Doc:         sym.Doc,
		}

		if err := c.db.UpsertPythonSymbol(dbSym); err != nil {
			log.Printf("Warning: failed to store symbol %s: %v", sym.Name, err)
		} else {
			publicCount++
		}
	}

	log.Printf("Indexed %s: %d symbols (%d public)", pkg.Info.Name, len(symbols), publicCount)

	return nil
}
