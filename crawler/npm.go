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
	"github.com/alexisbouchez/wikigo/jsparser"
)

const (
	NPMRegistryURL = "https://registry.npmjs.org"
	NPMSearchURL   = "https://registry.npmjs.org/-/v1/search"
)

// cleanRepoURL normalizes git repository URLs for web display
func cleanRepoURL(url string) string {
	url = strings.TrimPrefix(url, "git+")
	url = strings.TrimSuffix(url, ".git")
	return url
}

// NPMPackage represents npm package metadata
type NPMPackage struct {
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Description string            `json:"description"`
	Keywords    []string          `json:"keywords"`
	Author      NPMPerson         `json:"author"`
	License     string            `json:"license"`
	Repository  NPMRepository     `json:"repository"`
	Homepage    string            `json:"homepage"`
	Main        string            `json:"main"`
	Types       string            `json:"types"`
	TypeScript  bool              `json:"-"`
	Dist        NPMDist           `json:"dist"`
	Dependencies map[string]string `json:"dependencies"`
}

// NPMPerson represents package author/maintainer
type NPMPerson struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// NPMRepository represents repository info
type NPMRepository struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

// NPMDist represents distribution info
type NPMDist struct {
	Tarball string `json:"tarball"`
}

// NPMCrawler fetches and indexes NPM packages
type NPMCrawler struct {
	db        *db.DB
	client    *http.Client
	parser    *jsparser.Parser
	tempDir   string
	rateLimit time.Duration
}

// NewNPMCrawler creates a new NPM package crawler
func NewNPMCrawler(database *db.DB) (*NPMCrawler, error) {
	tempDir, err := os.MkdirTemp("", "npm-crawler-*")
	if err != nil {
		return nil, fmt.Errorf("creating temp dir: %w", err)
	}

	return &NPMCrawler{
		db:        database,
		client:    &http.Client{Timeout: 30 * time.Second},
		parser:    jsparser.NewParser(),
		tempDir:   tempDir,
		rateLimit: 100 * time.Millisecond, // npm rate limiting
	}, nil
}

// Close cleans up resources
func (c *NPMCrawler) Close() error {
	return os.RemoveAll(c.tempDir)
}

// FetchPackage fetches package metadata from npm registry
func (c *NPMCrawler) FetchPackage(name string) (*NPMPackage, error) {
	time.Sleep(c.rateLimit)

	url := fmt.Sprintf("%s/%s", NPMRegistryURL, name)
	resp, err := c.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetching package metadata: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("package not found: %s", name)
	}

	var data struct {
		DistTags map[string]string          `json:"dist-tags"`
		Versions map[string]json.RawMessage `json:"versions"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	// Get latest version
	latestVersion := data.DistTags["latest"]
	if latestVersion == "" {
		return nil, fmt.Errorf("no latest version found")
	}

	// Parse version metadata
	var pkg NPMPackage
	if err := json.Unmarshal(data.Versions[latestVersion], &pkg); err != nil {
		return nil, fmt.Errorf("parsing version metadata: %w", err)
	}

	// Check if package has TypeScript support
	pkg.TypeScript = pkg.Types != "" || strings.HasSuffix(pkg.Main, ".ts")

	return &pkg, nil
}

// DownloadPackage downloads and extracts package tarball
func (c *NPMCrawler) DownloadPackage(pkg *NPMPackage) (string, error) {
	time.Sleep(c.rateLimit)

	resp, err := c.client.Get(pkg.Dist.Tarball)
	if err != nil {
		return "", fmt.Errorf("downloading tarball: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed: status %d", resp.StatusCode)
	}

	// Extract to temp directory
	extractDir := filepath.Join(c.tempDir, pkg.Name+"@"+pkg.Version)
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

		// Remove "package/" prefix from tar paths
		targetPath := filepath.Join(extractDir, strings.TrimPrefix(header.Name, "package/"))

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

// ParsePackageSymbols parses JavaScript/TypeScript files and extracts symbols
func (c *NPMCrawler) ParsePackageSymbols(pkgDir string) ([]jsparser.Symbol, error) {
	var allSymbols []jsparser.Symbol

	err := filepath.Walk(pkgDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip node_modules and test directories
		if info.IsDir() {
			if info.Name() == "node_modules" || info.Name() == "test" || info.Name() == "__tests__" {
				return filepath.SkipDir
			}
			return nil
		}

		// Parse JS/TS files
		ext := filepath.Ext(path)
		if ext == ".js" || ext == ".ts" || ext == ".jsx" || ext == ".tsx" {
			symbols, err := c.parser.ParseFile(path)
			if err != nil {
				log.Printf("Warning: failed to parse %s: %v", path, err)
				return nil
			}
			allSymbols = append(allSymbols, symbols...)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walking package directory: %w", err)
	}

	return allSymbols, nil
}

// IndexPackage indexes an NPM package into the database
func (c *NPMCrawler) IndexPackage(name string) error {
	log.Printf("Indexing NPM package: %s", name)

	// Fetch metadata
	pkg, err := c.FetchPackage(name)
	if err != nil {
		return fmt.Errorf("fetching package: %w", err)
	}

	// Download and extract
	pkgDir, err := c.DownloadPackage(pkg)
	if err != nil {
		return fmt.Errorf("downloading package: %w", err)
	}
	defer os.RemoveAll(pkgDir)

	// Parse symbols
	symbols, err := c.ParsePackageSymbols(pkgDir)
	if err != nil {
		return fmt.Errorf("parsing symbols: %w", err)
	}

	log.Printf("Found %d symbols in %s", len(symbols), name)

	// Store package in database
	if c.db != nil {
		dbPkg := &db.JSPackage{
			Name:          pkg.Name,
			Version:       pkg.Version,
			Description:   pkg.Description,
			Author:        pkg.Author.Name,
			License:       pkg.License,
			RepositoryURL: cleanRepoURL(pkg.Repository.URL),
			Homepage:      pkg.Homepage,
			NPMURL:        fmt.Sprintf("https://www.npmjs.com/package/%s", pkg.Name),
			MainFile:      pkg.Main,
			TypesFile:     pkg.Types,
			HasTypeScript: pkg.TypeScript,
			Keywords:      pkg.Keywords,
			Dependencies:  pkg.Dependencies,
		}

		pkgID, err := c.db.UpsertJSPackage(dbPkg)
		if err != nil {
			return fmt.Errorf("storing package: %w", err)
		}

		// Delete old symbols
		if err := c.db.DeleteJSPackageSymbols(pkgID); err != nil {
			return fmt.Errorf("deleting old symbols: %w", err)
		}

		// Store symbols
		exportedCount := 0
		for _, sym := range symbols {
			dbSym := &db.JSSymbol{
				Name:        sym.Name,
				Kind:        sym.Kind,
				Signature:   sym.Signature,
				PackageID:   pkgID,
				PackageName: pkg.Name,
				FilePath:    sym.FilePath,
				Line:        sym.Line,
				Exported:    sym.Exported,
				Doc:         sym.Doc,
			}

			if err := c.db.UpsertJSSymbol(dbSym); err != nil {
				log.Printf("Warning: failed to store symbol %s: %v", sym.Name, err)
			}

			if sym.Exported {
				exportedCount++
			}
		}

		log.Printf("Stored %d symbols (%d exported) in database", len(symbols), exportedCount)
	}

	return nil
}

// SearchPackages searches npm registry for packages
func (c *NPMCrawler) SearchPackages(query string, limit int) ([]string, error) {
	time.Sleep(c.rateLimit)

	url := fmt.Sprintf("%s?text=%s&size=%d", NPMSearchURL, query, limit)
	resp, err := c.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("searching packages: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Objects []struct {
			Package struct {
				Name string `json:"name"`
			} `json:"package"`
		} `json:"objects"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding search results: %w", err)
	}

	var packages []string
	for _, obj := range result.Objects {
		packages = append(packages, obj.Package.Name)
	}

	return packages, nil
}
