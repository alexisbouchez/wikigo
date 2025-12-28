package crawler

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/doc"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/alexisbouchez/wikigo/db"
)

const (
	ProxyURL = "https://proxy.golang.org"
	IndexURL = "https://index.golang.org/index"
)

// Crawler fetches and indexes Go modules from proxy.golang.org
type Crawler struct {
	db         *db.DB
	client     *http.Client
	workers    int
	rateLimit  time.Duration
	tempDir    string
	stats      Stats
	statsMu    sync.Mutex
	maxModules int // 0 = unlimited
}

// Stats tracks crawling statistics
type Stats struct {
	ModulesProcessed int
	ModulesSucceeded int
	ModulesFailed    int
	SymbolsIndexed   int
	StartTime        time.Time
}

// ModuleVersion represents a module version from the index
type ModuleVersion struct {
	Path      string    `json:"Path"`
	Version   string    `json:"Version"`
	Timestamp time.Time `json:"Timestamp"`
}

// Config holds crawler configuration
type Config struct {
	DBPath     string
	Workers    int
	RateLimit  time.Duration
	Since      time.Time
	MaxModules int
	TempDir    string
}

// New creates a new crawler
func New(cfg Config) (*Crawler, error) {
	database, err := db.Open(cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	if cfg.Workers <= 0 {
		cfg.Workers = 4
	}
	if cfg.RateLimit <= 0 {
		cfg.RateLimit = 100 * time.Millisecond
	}
	if cfg.TempDir == "" {
		cfg.TempDir = os.TempDir()
	}

	return &Crawler{
		db:         database,
		client:     &http.Client{Timeout: 60 * time.Second},
		workers:    cfg.Workers,
		rateLimit:  cfg.RateLimit,
		tempDir:    cfg.TempDir,
		maxModules: cfg.MaxModules,
	}, nil
}

// Close closes the crawler and its resources
func (c *Crawler) Close() error {
	return c.db.Close()
}

// Run starts the crawling process
func (c *Crawler) Run(ctx context.Context, since time.Time) error {
	c.stats.StartTime = time.Now()

	log.Printf("Starting crawler with %d workers, rate limit %v", c.workers, c.rateLimit)

	// Create work channel
	modules := make(chan ModuleVersion, 100)

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < c.workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			c.worker(ctx, workerID, modules)
		}(i)
	}

	// Fetch module index
	go func() {
		defer close(modules)
		if err := c.fetchIndex(ctx, since, modules); err != nil {
			log.Printf("Error fetching index: %v", err)
		}
	}()

	// Wait for workers to finish
	wg.Wait()

	// Print final stats
	c.printStats()

	// Save crawl time to database
	if err := c.db.SetLastCrawlTime(time.Now()); err != nil {
		log.Printf("Warning: failed to save crawl time: %v", err)
	}

	return nil
}

// RunWithSchedule runs the crawler on a schedule
func (c *Crawler) RunWithSchedule(ctx context.Context, interval time.Duration) error {
	log.Printf("Starting scheduled crawler with interval %v", interval)

	// Run immediately on startup
	if err := c.runIncrementalCrawl(ctx); err != nil {
		if err == context.Canceled {
			return nil
		}
		log.Printf("Initial crawl failed: %v", err)
	}

	// Create ticker for scheduled runs
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Scheduler stopped")
			return nil
		case <-ticker.C:
			log.Printf("Starting scheduled crawl at %s", time.Now().Format(time.RFC3339))
			if err := c.runIncrementalCrawl(ctx); err != nil {
				if err == context.Canceled {
					return nil
				}
				log.Printf("Scheduled crawl failed: %v", err)
			}
		}
	}
}

// runIncrementalCrawl runs a crawl using the last crawl time from the database
func (c *Crawler) runIncrementalCrawl(ctx context.Context) error {
	// Get last crawl time from database
	since, err := c.db.GetLastCrawlTime()
	if err != nil {
		log.Printf("Warning: failed to get last crawl time: %v", err)
		// Continue with full crawl
	}

	if since.IsZero() {
		log.Println("No previous crawl found, starting full crawl")
	} else {
		log.Printf("Incremental crawl since %s", since.Format(time.RFC3339))
	}

	return c.Run(ctx, since)
}

// GetDB returns the database connection (for external access)
func (c *Crawler) GetDB() *db.DB {
	return c.db
}

// fetchIndex fetches the module index from index.golang.org
func (c *Crawler) fetchIndex(ctx context.Context, since time.Time, modules chan<- ModuleVersion) error {
	url := IndexURL
	if !since.IsZero() {
		url = fmt.Sprintf("%s?since=%s", IndexURL, since.Format(time.RFC3339))
	}

	log.Printf("Fetching index from %s", url)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("index returned status %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	count := 0

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Text()
		if line == "" {
			continue
		}

		var mv ModuleVersion
		if err := json.Unmarshal([]byte(line), &mv); err != nil {
			log.Printf("Warning: failed to parse index line: %v", err)
			continue
		}

		// Skip internal/test modules
		if shouldSkipModule(mv.Path) {
			continue
		}

		select {
		case modules <- mv:
			count++
			if c.maxModules > 0 && count >= c.maxModules {
				log.Printf("Reached max modules limit: %d", c.maxModules)
				return nil
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	log.Printf("Fetched %d modules from index", count)
	return scanner.Err()
}

// shouldSkipModule returns true if the module should be skipped
func shouldSkipModule(path string) bool {
	// Skip test modules
	if strings.HasSuffix(path, ".test") {
		return true
	}
	// Skip vendor paths
	if strings.Contains(path, "/vendor/") {
		return true
	}
	// Skip internal packages from other modules
	if strings.Contains(path, "/internal/") && !strings.HasPrefix(path, "golang.org/x/") {
		return true
	}
	return false
}

// worker processes modules from the channel
func (c *Crawler) worker(ctx context.Context, id int, modules <-chan ModuleVersion) {
	rateLimiter := time.NewTicker(c.rateLimit)
	defer rateLimiter.Stop()

	for mv := range modules {
		select {
		case <-ctx.Done():
			return
		case <-rateLimiter.C:
		}

		if err := c.processModule(ctx, mv); err != nil {
			log.Printf("[Worker %d] Failed %s@%s: %v", id, mv.Path, mv.Version, err)
			c.recordFailure()
		} else {
			log.Printf("[Worker %d] Indexed %s@%s", id, mv.Path, mv.Version)
			c.recordSuccess()
		}
	}
}

// processModule fetches and indexes a single module
func (c *Crawler) processModule(ctx context.Context, mv ModuleVersion) error {
	c.statsMu.Lock()
	c.stats.ModulesProcessed++
	c.statsMu.Unlock()

	// Create temp directory for this module
	tempDir, err := os.MkdirTemp(c.tempDir, "wikigo-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Download and extract module
	if err := c.downloadModule(ctx, mv, tempDir); err != nil {
		return fmt.Errorf("downloading module: %w", err)
	}

	// Find the extracted directory
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		return fmt.Errorf("reading temp dir: %w", err)
	}
	if len(entries) == 0 {
		return fmt.Errorf("no files extracted")
	}

	moduleDir := filepath.Join(tempDir, entries[0].Name())

	// Extract and index packages
	return c.indexModule(ctx, mv, moduleDir)
}

// downloadModule downloads and extracts a module zip
func (c *Crawler) downloadModule(ctx context.Context, mv ModuleVersion, destDir string) error {
	// Escape module path for URL
	escapedPath := escapeModulePath(mv.Path)
	url := fmt.Sprintf("%s/%s/@v/%s.zip", ProxyURL, escapedPath, mv.Version)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	// Read zip into memory (modules are usually small)
	data, err := io.ReadAll(io.LimitReader(resp.Body, 100*1024*1024)) // 100MB limit
	if err != nil {
		return fmt.Errorf("reading zip: %w", err)
	}

	// Extract zip
	zipReader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("opening zip: %w", err)
	}

	for _, f := range zipReader.File {
		if err := extractZipFile(f, destDir); err != nil {
			return fmt.Errorf("extracting %s: %w", f.Name, err)
		}
	}

	return nil
}

// extractZipFile extracts a single file from a zip
func extractZipFile(f *zip.File, destDir string) error {
	destPath := filepath.Join(destDir, f.Name)

	// Check for path traversal
	if !strings.HasPrefix(destPath, filepath.Clean(destDir)+string(os.PathSeparator)) {
		return fmt.Errorf("invalid file path: %s", f.Name)
	}

	if f.FileInfo().IsDir() {
		return os.MkdirAll(destPath, 0755)
	}

	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}

	// Extract file
	src, err := f.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, io.LimitReader(src, 10*1024*1024)) // 10MB per file limit
	return err
}

// indexModule indexes all packages in a module
func (c *Crawler) indexModule(ctx context.Context, mv ModuleVersion, moduleDir string) error {
	// Find all Go packages in the module
	var packages []string

	err := filepath.Walk(moduleDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if info.IsDir() {
			// Skip hidden, vendor, testdata directories
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "vendor" || name == "testdata" {
				return filepath.SkipDir
			}
			// Check if directory contains Go files
			hasGo, _ := filepath.Glob(filepath.Join(path, "*.go"))
			if len(hasGo) > 0 {
				packages = append(packages, path)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Index each package
	for _, pkgDir := range packages {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := c.indexPackage(ctx, mv, moduleDir, pkgDir); err != nil {
			// Log but continue with other packages
			log.Printf("Warning: failed to index package in %s: %v", pkgDir, err)
		}
	}

	return nil
}

// indexPackage indexes a single package
func (c *Crawler) indexPackage(ctx context.Context, mv ModuleVersion, moduleDir, pkgDir string) error {
	// Calculate import path
	relPath, err := filepath.Rel(moduleDir, pkgDir)
	if err != nil {
		return err
	}
	importPath := mv.Path
	if relPath != "." {
		importPath = mv.Path + "/" + filepath.ToSlash(relPath)
	}

	// Parse package
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, pkgDir, func(fi os.FileInfo) bool {
		name := fi.Name()
		return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
	}, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("parsing package: %w", err)
	}

	// Find the main package (not _test)
	var astPkg *ast.Package
	for name, pkg := range pkgs {
		if !strings.HasSuffix(name, "_test") {
			astPkg = pkg
			break
		}
	}
	if astPkg == nil {
		return nil // No main package found
	}

	// Create doc package
	var files []*ast.File
	for _, f := range astPkg.Files {
		files = append(files, f)
	}
	docPkg, err := doc.NewFromFiles(fset, files, importPath, doc.AllDecls|doc.AllMethods)
	if err != nil {
		return fmt.Errorf("creating doc: %w", err)
	}

	// Read go.mod if at module root
	var goModContent, goVersion, modulePath string
	if relPath == "." {
		if data, err := os.ReadFile(filepath.Join(pkgDir, "go.mod")); err == nil {
			goModContent = string(data)
			for _, line := range strings.Split(goModContent, "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "module ") {
					modulePath = strings.TrimSpace(strings.TrimPrefix(line, "module "))
				}
				if strings.HasPrefix(line, "go ") {
					goVersion = strings.TrimSpace(strings.TrimPrefix(line, "go "))
				}
			}
		}
	}
	if modulePath == "" {
		modulePath = mv.Path
	}

	// Detect license
	license, licenseText := detectLicense(moduleDir)

	// Build database package
	dbPkg := &db.Package{
		ImportPath:      importPath,
		Name:            docPkg.Name,
		Synopsis:        doc.Synopsis(docPkg.Doc),
		Doc:             docPkg.Doc,
		Version:         mv.Version,
		Versions:        []string{mv.Version},
		IsTagged:        isTaggedVersion(mv.Version),
		IsStable:        isStableVersion(mv.Version),
		License:         license,
		LicenseText:     licenseText,
		Redistributable: isRedistributable(license),
		Repository:      moduleToRepoURL(mv.Path),
		HasValidMod:     goModContent != "",
		GoVersion:       goVersion,
		ModulePath:      modulePath,
		GoModContent:    goModContent,
	}

	// Upsert package
	pkgID, err := c.db.UpsertPackage(dbPkg)
	if err != nil {
		return fmt.Errorf("upserting package: %w", err)
	}

	// Delete old symbols
	c.db.DeletePackageSymbols(pkgID)

	// Index symbols
	symbolCount := 0

	// Functions
	for _, fn := range docPkg.Funcs {
		sym := &db.Symbol{
			Name:       fn.Name,
			Kind:       "func",
			PackageID:  pkgID,
			ImportPath: importPath,
			Synopsis:   doc.Synopsis(fn.Doc),
			Deprecated: isDeprecated(fn.Doc),
		}
		if err := c.db.UpsertSymbol(sym); err == nil {
			symbolCount++
		}
	}

	// Types
	for _, t := range docPkg.Types {
		sym := &db.Symbol{
			Name:       t.Name,
			Kind:       "type",
			PackageID:  pkgID,
			ImportPath: importPath,
			Synopsis:   doc.Synopsis(t.Doc),
			Deprecated: isDeprecated(t.Doc),
		}
		if err := c.db.UpsertSymbol(sym); err == nil {
			symbolCount++
		}

		// Methods
		for _, m := range t.Methods {
			sym := &db.Symbol{
				Name:       t.Name + "." + m.Name,
				Kind:       "method",
				PackageID:  pkgID,
				ImportPath: importPath,
				Synopsis:   doc.Synopsis(m.Doc),
				Deprecated: isDeprecated(m.Doc),
			}
			if err := c.db.UpsertSymbol(sym); err == nil {
				symbolCount++
			}
		}

		// Type functions
		for _, fn := range t.Funcs {
			sym := &db.Symbol{
				Name:       fn.Name,
				Kind:       "func",
				PackageID:  pkgID,
				ImportPath: importPath,
				Synopsis:   doc.Synopsis(fn.Doc),
				Deprecated: isDeprecated(fn.Doc),
			}
			if err := c.db.UpsertSymbol(sym); err == nil {
				symbolCount++
			}
		}
	}

	// Constants
	for _, con := range docPkg.Consts {
		for _, name := range con.Names {
			sym := &db.Symbol{
				Name:       name,
				Kind:       "const",
				PackageID:  pkgID,
				ImportPath: importPath,
				Synopsis:   doc.Synopsis(con.Doc),
			}
			if err := c.db.UpsertSymbol(sym); err == nil {
				symbolCount++
			}
		}
	}

	// Variables
	for _, v := range docPkg.Vars {
		for _, name := range v.Names {
			sym := &db.Symbol{
				Name:       name,
				Kind:       "var",
				PackageID:  pkgID,
				ImportPath: importPath,
				Synopsis:   doc.Synopsis(v.Doc),
			}
			if err := c.db.UpsertSymbol(sym); err == nil {
				symbolCount++
			}
		}
	}

	// Index imports
	for _, f := range files {
		for _, imp := range f.Imports {
			if imp.Path != nil {
				impPath := strings.Trim(imp.Path.Value, `"`)
				c.db.AddImport(importPath, impPath, modulePath)
			}
		}
	}

	c.statsMu.Lock()
	c.stats.SymbolsIndexed += symbolCount
	c.statsMu.Unlock()

	return nil
}

func (c *Crawler) recordSuccess() {
	c.statsMu.Lock()
	c.stats.ModulesSucceeded++
	c.statsMu.Unlock()
}

func (c *Crawler) recordFailure() {
	c.statsMu.Lock()
	c.stats.ModulesFailed++
	c.statsMu.Unlock()
}

func (c *Crawler) printStats() {
	c.statsMu.Lock()
	defer c.statsMu.Unlock()

	elapsed := time.Since(c.stats.StartTime)
	log.Printf("=== Crawl Complete ===")
	log.Printf("Duration: %v", elapsed.Round(time.Second))
	log.Printf("Modules processed: %d", c.stats.ModulesProcessed)
	log.Printf("Modules succeeded: %d", c.stats.ModulesSucceeded)
	log.Printf("Modules failed: %d", c.stats.ModulesFailed)
	log.Printf("Symbols indexed: %d", c.stats.SymbolsIndexed)

	if c.stats.ModulesProcessed > 0 {
		rate := float64(c.stats.ModulesProcessed) / elapsed.Seconds()
		log.Printf("Rate: %.2f modules/sec", rate)
	}
}

// escapeModulePath escapes a module path for use in URLs
func escapeModulePath(path string) string {
	var result strings.Builder
	for _, r := range path {
		if r >= 'A' && r <= 'Z' {
			result.WriteByte('!')
			result.WriteRune(r + 32) // lowercase
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// Helper functions

func isTaggedVersion(version string) bool {
	semverRegex := regexp.MustCompile(`^v\d+\.\d+\.\d+(-[a-zA-Z0-9.-]+)?(\+[a-zA-Z0-9.-]+)?$`)
	return semverRegex.MatchString(version)
}

func isStableVersion(version string) bool {
	if !isTaggedVersion(version) {
		return false
	}
	// v0.x.x is not stable
	if strings.HasPrefix(version, "v0.") {
		return false
	}
	// Pre-release versions are not stable
	if strings.Contains(version, "-") {
		return false
	}
	return true
}

func isDeprecated(docText string) bool {
	docText = strings.TrimSpace(docText)
	if strings.HasPrefix(docText, "Deprecated:") {
		return true
	}
	return strings.Contains(docText, "\nDeprecated:") || strings.Contains(docText, "\n\nDeprecated:")
}

func isRedistributable(license string) bool {
	redistributable := map[string]bool{
		"MIT": true, "Apache-2.0": true, "BSD-2-Clause": true, "BSD-3-Clause": true,
		"ISC": true, "MPL-2.0": true, "Unlicense": true, "CC0-1.0": true, "LGPL": true,
	}
	return redistributable[license]
}

func detectLicense(dir string) (licenseType string, licenseText string) {
	licenseFiles := []string{
		"LICENSE", "LICENSE.txt", "LICENSE.md", "LICENCE", "LICENCE.txt",
		"COPYING", "COPYING.txt",
	}

	for _, name := range licenseFiles {
		path := filepath.Join(dir, name)
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		text := string(content)
		return identifyLicense(text), text
	}
	return "", ""
}

func identifyLicense(content string) string {
	content = strings.ToLower(content)
	switch {
	case strings.Contains(content, "apache license") && strings.Contains(content, "version 2.0"):
		return "Apache-2.0"
	case strings.Contains(content, "mit license") || strings.Contains(content, "permission is hereby granted, free of charge"):
		return "MIT"
	case strings.Contains(content, "bsd 3-clause") || (strings.Contains(content, "redistribution and use") && strings.Contains(content, "neither the name")):
		return "BSD-3-Clause"
	case strings.Contains(content, "bsd 2-clause"):
		return "BSD-2-Clause"
	case strings.Contains(content, "gnu general public license") && strings.Contains(content, "version 3"):
		return "GPL-3.0"
	case strings.Contains(content, "gnu general public license") && strings.Contains(content, "version 2"):
		return "GPL-2.0"
	case strings.Contains(content, "mozilla public license") && strings.Contains(content, "2.0"):
		return "MPL-2.0"
	case strings.Contains(content, "unlicense"):
		return "Unlicense"
	case strings.Contains(content, "isc license"):
		return "ISC"
	}
	return "Unknown"
}

func moduleToRepoURL(modulePath string) string {
	parts := strings.Split(modulePath, "/")
	if len(parts) < 2 {
		return ""
	}
	host := parts[0]
	switch {
	case host == "github.com" && len(parts) >= 3:
		return "https://github.com/" + parts[1] + "/" + parts[2]
	case host == "gitlab.com" && len(parts) >= 3:
		return "https://gitlab.com/" + parts[1] + "/" + parts[2]
	case host == "bitbucket.org" && len(parts) >= 3:
		return "https://bitbucket.org/" + parts[1] + "/" + parts[2]
	case strings.HasPrefix(host, "go.googlesource.com"):
		return "https://go.googlesource.com/" + parts[1]
	case host == "golang.org" && len(parts) >= 3 && parts[1] == "x":
		return "https://go.googlesource.com/" + parts[2]
	}
	return ""
}

// formatDecl formats an AST declaration as a string
func formatDecl(fset *token.FileSet, node ast.Node) string {
	if node == nil {
		return ""
	}
	var buf strings.Builder
	if err := format.Node(&buf, fset, node); err != nil {
		return ""
	}
	return buf.String()
}
