package web

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/alexisbouchez/wikigo/db"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static/*
var staticFS embed.FS

// PackageDoc represents complete documentation for a Go package
type PackageDoc struct {
	ImportPath       string     `json:"import_path"`
	Name             string     `json:"name"`
	Doc              string     `json:"doc"`
	Synopsis         string     `json:"synopsis"`
	Version          string     `json:"version,omitempty"`
	Versions         []string   `json:"versions,omitempty"`
	IsTagged         bool       `json:"is_tagged,omitempty"`
	IsStable         bool       `json:"is_stable,omitempty"`
	PublishedAt      string     `json:"published_at,omitempty"`
	License          string     `json:"license,omitempty"`
	LicenseText      string     `json:"license_text,omitempty"`
	Redistributable  bool       `json:"redistributable,omitempty"`
	Repository       string     `json:"repository,omitempty"`
	HasValidMod      bool       `json:"has_valid_mod,omitempty"`
	GoVersion        string     `json:"go_version,omitempty"`
	ModulePath       string     `json:"module_path,omitempty"`
	GoModContent     string     `json:"gomod_content,omitempty"`
	GOOS             []string   `json:"goos,omitempty"`
	GOARCH           []string   `json:"goarch,omitempty"`
	Constants        []Constant `json:"constants"`
	Variables        []Variable `json:"variables"`
	Functions        []Function `json:"functions"`
	Types            []Type     `json:"types"`
	Examples         []Example  `json:"examples"`
	Imports          []string   `json:"imports"`
	Filenames        []string   `json:"filenames"`
}

// Subdirectory represents a child package
type Subdirectory struct {
	Name     string
	Path     string
	Synopsis string
}

// Constant represents a documented constant
type Constant struct {
	Names []string `json:"names"`
	Doc   string   `json:"doc"`
	Decl  string   `json:"decl"`
}

// Variable represents a documented variable
type Variable struct {
	Names []string `json:"names"`
	Doc   string   `json:"doc"`
	Decl  string   `json:"decl"`
}

// Function represents a documented function
type Function struct {
	Name       string    `json:"name"`
	Doc        string    `json:"doc"`
	Signature  string    `json:"signature"`
	Recv       string    `json:"recv,omitempty"`
	Filename   string    `json:"filename,omitempty"`
	Line       int       `json:"line,omitempty"`
	Deprecated bool      `json:"deprecated,omitempty"`
	Examples   []Example `json:"examples,omitempty"`
}

// Type represents a documented type
type Type struct {
	Name       string     `json:"name"`
	Doc        string     `json:"doc"`
	Decl       string     `json:"decl"`
	Filename   string     `json:"filename,omitempty"`
	Line       int        `json:"line,omitempty"`
	Deprecated bool       `json:"deprecated,omitempty"`
	Constants  []Constant `json:"constants,omitempty"`
	Variables  []Variable `json:"variables,omitempty"`
	Functions  []Function `json:"funcs,omitempty"`
	Methods    []Function `json:"methods,omitempty"`
	Examples   []Example  `json:"examples,omitempty"`
}

// Example represents a runnable example
type Example struct {
	Name   string `json:"name"`
	Doc    string `json:"doc"`
	Code   string `json:"code"`
	Output string `json:"output,omitempty"`
}

// Server represents the documentation web server
type Server struct {
	packages  map[string]*PackageDoc
	templates *template.Template
	dataDir   string
	db        *db.DB // optional database for indexing
}

// NewServer creates a new documentation server
func NewServer(dataDir string) (*Server, error) {
	return NewServerWithDB(dataDir, "")
}

// NewServerWithDB creates a new documentation server with optional SQLite database
func NewServerWithDB(dataDir, dbPath string) (*Server, error) {
	s := &Server{
		packages: make(map[string]*PackageDoc),
		dataDir:  dataDir,
	}

	// Open database if path provided
	if dbPath != "" {
		database, err := db.Open(dbPath)
		if err != nil {
			return nil, fmt.Errorf("opening database: %w", err)
		}
		s.db = database
		log.Printf("Opened database: %s", dbPath)
	}

	// Parse templates
	funcMap := template.FuncMap{
		"formatDoc":      formatDoc,
		"formatDocHTML":  formatDocHTML,
		"shortDoc":       shortDoc,
		"baseName":       filepath.Base,
		"hasPrefix":      strings.HasPrefix,
		"trimPrefix":     strings.TrimPrefix,
		"join":           strings.Join,
		"lower":          strings.ToLower,
		"anchorName":     anchorName,
		"sourceLink":     sourceLink,
		"split":          strings.Split,
		"sub":            func(a, b int) int { return a - b },
		"cond":           func(cond bool, t, f string) string { if cond { return t }; return f },
		"highlightQuery": highlightQuery,
	}

	tmpl, err := template.New("").Funcs(funcMap).ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	s.templates = tmpl

	// Load all JSON files from data directory
	if err := s.loadPackages(); err != nil {
		return nil, err
	}

	return s, nil
}

// Close closes the server and its resources
func (s *Server) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// IndexPackage indexes a package into the database
func (s *Server) IndexPackage(pkg *PackageDoc) error {
	if s.db == nil {
		return fmt.Errorf("database not configured")
	}

	// Convert PackageDoc to JSON for storage
	docJSON, err := json.Marshal(pkg)
	if err != nil {
		return fmt.Errorf("marshaling package: %w", err)
	}

	// Create database package
	dbPkg := &db.Package{
		ImportPath:      pkg.ImportPath,
		Name:            pkg.Name,
		Synopsis:        pkg.Synopsis,
		Doc:             pkg.Doc,
		Version:         pkg.Version,
		Versions:        pkg.Versions,
		IsTagged:        pkg.IsTagged,
		IsStable:        pkg.IsStable,
		License:         pkg.License,
		LicenseText:     pkg.LicenseText,
		Redistributable: pkg.Redistributable,
		Repository:      pkg.Repository,
		HasValidMod:     pkg.HasValidMod,
		GoVersion:       pkg.GoVersion,
		ModulePath:      pkg.ModulePath,
		GoModContent:    pkg.GoModContent,
		GOOS:            pkg.GOOS,
		GOARCH:          pkg.GOARCH,
		DocJSON:         string(docJSON),
	}

	// Upsert package
	pkgID, err := s.db.UpsertPackage(dbPkg)
	if err != nil {
		return fmt.Errorf("upserting package: %w", err)
	}

	// Delete old symbols
	if err := s.db.DeletePackageSymbols(pkgID); err != nil {
		return fmt.Errorf("deleting old symbols: %w", err)
	}

	// Index symbols
	for _, fn := range pkg.Functions {
		sym := &db.Symbol{
			Name:       fn.Name,
			Kind:       "func",
			PackageID:  pkgID,
			ImportPath: pkg.ImportPath,
			Synopsis:   shortDoc(fn.Doc),
			Deprecated: fn.Deprecated,
		}
		if err := s.db.UpsertSymbol(sym); err != nil {
			log.Printf("Warning: failed to index symbol %s: %v", fn.Name, err)
		}
	}

	for _, t := range pkg.Types {
		// Index type
		sym := &db.Symbol{
			Name:       t.Name,
			Kind:       "type",
			PackageID:  pkgID,
			ImportPath: pkg.ImportPath,
			Synopsis:   shortDoc(t.Doc),
			Deprecated: t.Deprecated,
		}
		if err := s.db.UpsertSymbol(sym); err != nil {
			log.Printf("Warning: failed to index type %s: %v", t.Name, err)
		}

		// Index methods
		for _, m := range t.Methods {
			sym := &db.Symbol{
				Name:       t.Name + "." + m.Name,
				Kind:       "method",
				PackageID:  pkgID,
				ImportPath: pkg.ImportPath,
				Synopsis:   shortDoc(m.Doc),
				Deprecated: m.Deprecated,
			}
			if err := s.db.UpsertSymbol(sym); err != nil {
				log.Printf("Warning: failed to index method %s: %v", m.Name, err)
			}
		}

		// Index type functions (constructors)
		for _, fn := range t.Functions {
			sym := &db.Symbol{
				Name:       fn.Name,
				Kind:       "func",
				PackageID:  pkgID,
				ImportPath: pkg.ImportPath,
				Synopsis:   shortDoc(fn.Doc),
				Deprecated: fn.Deprecated,
			}
			if err := s.db.UpsertSymbol(sym); err != nil {
				log.Printf("Warning: failed to index func %s: %v", fn.Name, err)
			}
		}
	}

	// Index constants
	for _, c := range pkg.Constants {
		for _, name := range c.Names {
			sym := &db.Symbol{
				Name:       name,
				Kind:       "const",
				PackageID:  pkgID,
				ImportPath: pkg.ImportPath,
				Synopsis:   shortDoc(c.Doc),
			}
			if err := s.db.UpsertSymbol(sym); err != nil {
				log.Printf("Warning: failed to index const %s: %v", name, err)
			}
		}
	}

	// Index variables
	for _, v := range pkg.Variables {
		for _, name := range v.Names {
			sym := &db.Symbol{
				Name:       name,
				Kind:       "var",
				PackageID:  pkgID,
				ImportPath: pkg.ImportPath,
				Synopsis:   shortDoc(v.Doc),
			}
			if err := s.db.UpsertSymbol(sym); err != nil {
				log.Printf("Warning: failed to index var %s: %v", name, err)
			}
		}
	}

	// Index imports
	for _, imp := range pkg.Imports {
		if err := s.db.AddImport(pkg.ImportPath, imp, pkg.ModulePath); err != nil {
			log.Printf("Warning: failed to index import %s: %v", imp, err)
		}
	}

	log.Printf("Indexed package: %s", pkg.ImportPath)
	return nil
}

// GetImportedByCount returns the count of packages that import the given package
func (s *Server) GetImportedByCount(importPath string) int {
	if s.db == nil {
		return 0
	}
	count, err := s.db.GetImportedByCount(importPath)
	if err != nil {
		log.Printf("Error getting imported by count: %v", err)
		return 0
	}
	return count
}

// GetDBStats returns database statistics
func (s *Server) GetDBStats() (packageCount, symbolCount, importCount int) {
	if s.db == nil {
		return len(s.packages), 0, 0
	}
	packageCount, symbolCount, importCount, err := s.db.GetStats()
	if err != nil {
		log.Printf("Error getting database stats: %v", err)
	}
	return
}

// loadPackages loads all package documentation from JSON files
func (s *Server) loadPackages() error {
	return filepath.Walk(s.dataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			log.Printf("Warning: could not read %s: %v", path, err)
			return nil
		}

		var pkg PackageDoc
		if err := json.Unmarshal(data, &pkg); err != nil {
			log.Printf("Warning: could not parse %s: %v", path, err)
			return nil
		}

		s.packages[pkg.ImportPath] = &pkg
		log.Printf("Loaded package: %s", pkg.ImportPath)

		// Index into database if available
		if s.db != nil {
			if err := s.IndexPackage(&pkg); err != nil {
				log.Printf("Warning: could not index %s: %v", pkg.ImportPath, err)
			}
		}

		return nil
	})
}

// ListenAndServe starts the HTTP server
func (s *Server) ListenAndServe(addr string) error {
	mux := http.NewServeMux()

	// Static files
	staticContent, err := fs.Sub(staticFS, "static")
	if err != nil {
		return err
	}
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticContent))))

	// Routes
	mux.HandleFunc("/", s.handleHome)
	mux.HandleFunc("/search", s.handleSearch)
	mux.HandleFunc("/api/", s.handleAPI)
	mux.HandleFunc("/badge/", s.handleBadge)
	mux.HandleFunc("/license/", s.handleLicense)
	mux.HandleFunc("/imports/", s.handleImports)
	mux.HandleFunc("/mod/", s.handleModule)
	mux.HandleFunc("/versions/", s.handleVersions)
	mux.HandleFunc("/importedby/", s.handleImportedBy)
	mux.HandleFunc("/symbols", s.handleSymbolSearch)
	mux.HandleFunc("/diff/", s.handleDiff)
	mux.HandleFunc("/compare/", s.handleCompare)

	log.Printf("Starting server on %s", addr)
	return http.ListenAndServe(addr, mux)
}

// handleHome handles the home page and package documentation pages
func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")

	if path == "" {
		s.renderHome(w, r)
		return
	}

	// Try to find package
	pkg, ok := s.packages[path]
	if !ok {
		// Try with common prefixes
		for importPath, p := range s.packages {
			if strings.HasSuffix(importPath, "/"+path) || importPath == path {
				pkg = p
				ok = true
				break
			}
		}
	}

	if !ok {
		http.NotFound(w, r)
		return
	}

	s.renderPackage(w, r, pkg)
}

// renderHome renders the home page
func (s *Server) renderHome(w http.ResponseWriter, r *http.Request) {
	data := struct {
		Title       string
		SearchQuery string
		Packages    map[string]*PackageDoc
	}{
		Title:       "Go Packages",
		SearchQuery: "",
		Packages:    s.packages,
	}

	if err := s.templates.ExecuteTemplate(w, "home.html", data); err != nil {
		log.Printf("Error rendering home: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// getSubdirectories returns subdirectories for a package
func (s *Server) getSubdirectories(importPath string) []Subdirectory {
	var subdirs []Subdirectory
	prefix := importPath + "/"

	for path, pkg := range s.packages {
		if strings.HasPrefix(path, prefix) {
			rest := strings.TrimPrefix(path, prefix)
			// Only include direct children (no further slashes)
			if !strings.Contains(rest, "/") {
				subdirs = append(subdirs, Subdirectory{
					Name:     rest,
					Path:     path,
					Synopsis: pkg.Synopsis,
				})
			}
		}
	}
	return subdirs
}

// renderPackage renders a package documentation page
func (s *Server) renderPackage(w http.ResponseWriter, r *http.Request, pkg *PackageDoc) {
	subdirs := s.getSubdirectories(pkg.ImportPath)
	importedByCount := s.GetImportedByCount(pkg.ImportPath)

	data := struct {
		Title           string
		SearchQuery     string
		Pkg             *PackageDoc
		Subdirectories  []Subdirectory
		ImportedByCount int
	}{
		Title:           pkg.Name + " package - " + pkg.ImportPath + " - Go Packages",
		SearchQuery:     "",
		Pkg:             pkg,
		Subdirectories:  subdirs,
		ImportedByCount: importedByCount,
	}

	if err := s.templates.ExecuteTemplate(w, "package.html", data); err != nil {
		log.Printf("Error rendering package: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleSearch handles search requests
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	queryLower := strings.ToLower(query)
	var results []*PackageDoc
	for _, pkg := range s.packages {
		if strings.Contains(strings.ToLower(pkg.ImportPath), queryLower) ||
			strings.Contains(strings.ToLower(pkg.Name), queryLower) ||
			strings.Contains(strings.ToLower(pkg.Synopsis), queryLower) {
			results = append(results, pkg)
		}
	}

	data := struct {
		Title       string
		SearchQuery string
		Query       string
		Results     []*PackageDoc
	}{
		Title:       "Search Results - " + query + " - Go Packages",
		SearchQuery: query,
		Query:       query,
		Results:     results,
	}

	if err := s.templates.ExecuteTemplate(w, "search.html", data); err != nil {
		log.Printf("Error rendering search: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleAPI handles JSON API requests
func (s *Server) handleAPI(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/")

	if path == "" || path == "packages" {
		// List all packages
		w.Header().Set("Content-Type", "application/json")
		var pkgList []map[string]string
		for importPath, pkg := range s.packages {
			pkgList = append(pkgList, map[string]string{
				"import_path": importPath,
				"name":        pkg.Name,
				"synopsis":    pkg.Synopsis,
			})
		}
		json.NewEncoder(w).Encode(pkgList)
		return
	}

	if path == "search" {
		query := r.URL.Query().Get("q")
		w.Header().Set("Content-Type", "application/json")
		if query == "" {
			json.NewEncoder(w).Encode([]map[string]string{})
			return
		}
		queryLower := strings.ToLower(query)
		var results []map[string]string
		for _, pkg := range s.packages {
			if strings.Contains(strings.ToLower(pkg.ImportPath), queryLower) ||
				strings.Contains(strings.ToLower(pkg.Name), queryLower) ||
				strings.Contains(strings.ToLower(pkg.Synopsis), queryLower) {
				results = append(results, map[string]string{
					"import_path": pkg.ImportPath,
					"name":        pkg.Name,
					"synopsis":    pkg.Synopsis,
				})
			}
		}
		json.NewEncoder(w).Encode(results)
		return
	}

	// Try to find package
	pkg, ok := s.packages[path]
	if !ok {
		for importPath, p := range s.packages {
			if strings.HasSuffix(importPath, "/"+path) || importPath == path {
				pkg = p
				ok = true
				break
			}
		}
	}

	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "package not found"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pkg)
}

// handleBadge handles badge generation (shields.io compatible)
func (s *Server) handleBadge(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/badge/")
	if path == "" {
		http.Error(w, "package path required", http.StatusBadRequest)
		return
	}

	// Parse badge type from query param (default: go-version)
	badgeType := r.URL.Query().Get("type")
	if badgeType == "" {
		badgeType = "go-version"
	}

	// Find package
	pkg, ok := s.packages[path]
	if !ok {
		for importPath, p := range s.packages {
			if strings.HasSuffix(importPath, "/"+path) || importPath == path {
				pkg = p
				ok = true
				break
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "max-age=3600")

	if !ok {
		// Return unknown badge
		json.NewEncoder(w).Encode(map[string]interface{}{
			"schemaVersion": 1,
			"label":         "go",
			"message":       "unknown",
			"color":         "lightgrey",
		})
		return
	}

	// Generate badge based on type
	var badge map[string]interface{}
	switch badgeType {
	case "go-version":
		version := pkg.GoVersion
		if version == "" {
			version = "unknown"
		}
		badge = map[string]interface{}{
			"schemaVersion": 1,
			"label":         "go",
			"message":       version,
			"color":         "00add8",
		}
	case "license":
		license := pkg.License
		color := "blue"
		if license == "" {
			license = "unknown"
			color = "lightgrey"
		}
		badge = map[string]interface{}{
			"schemaVersion": 1,
			"label":         "license",
			"message":       license,
			"color":         color,
		}
	case "valid-mod":
		msg := "yes"
		color := "brightgreen"
		if !pkg.HasValidMod {
			msg = "no"
			color = "red"
		}
		badge = map[string]interface{}{
			"schemaVersion": 1,
			"label":         "go.mod",
			"message":       msg,
			"color":         color,
		}
	default:
		badge = map[string]interface{}{
			"schemaVersion": 1,
			"label":         "wikigo",
			"message":       pkg.Name,
			"color":         "00add8",
		}
	}

	json.NewEncoder(w).Encode(badge)
}

// handleLicense handles the license full text page
func (s *Server) handleLicense(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/license/")
	if path == "" {
		http.NotFound(w, r)
		return
	}

	// Find package
	pkg, ok := s.packages[path]
	if !ok {
		for importPath, p := range s.packages {
			if strings.HasSuffix(importPath, "/"+path) || importPath == path {
				pkg = p
				ok = true
				break
			}
		}
	}

	if !ok || pkg.LicenseText == "" {
		http.NotFound(w, r)
		return
	}

	data := struct {
		Title       string
		SearchQuery string
		Pkg         *PackageDoc
	}{
		Title:       "License - " + pkg.ImportPath + " - Go Packages",
		SearchQuery: "",
		Pkg:         pkg,
	}

	if err := s.templates.ExecuteTemplate(w, "license.html", data); err != nil {
		log.Printf("Error rendering license: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleImports handles the imports list page
func (s *Server) handleImports(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/imports/")
	if path == "" {
		http.NotFound(w, r)
		return
	}

	// Find package
	pkg, ok := s.packages[path]
	if !ok {
		for importPath, p := range s.packages {
			if strings.HasSuffix(importPath, "/"+path) || importPath == path {
				pkg = p
				ok = true
				break
			}
		}
	}

	if !ok {
		http.NotFound(w, r)
		return
	}

	// Group imports by category
	type ImportGroup struct {
		Name    string
		Imports []string
	}

	var stdLib, external []string
	for _, imp := range pkg.Imports {
		if !strings.Contains(imp, ".") {
			stdLib = append(stdLib, imp)
		} else {
			external = append(external, imp)
		}
	}

	var groups []ImportGroup
	if len(stdLib) > 0 {
		groups = append(groups, ImportGroup{Name: "Standard Library", Imports: stdLib})
	}
	if len(external) > 0 {
		groups = append(groups, ImportGroup{Name: "External", Imports: external})
	}

	data := struct {
		Title        string
		SearchQuery  string
		Pkg          *PackageDoc
		ImportGroups []ImportGroup
	}{
		Title:        "Imports - " + pkg.ImportPath + " - Go Packages",
		SearchQuery:  "",
		Pkg:          pkg,
		ImportGroups: groups,
	}

	if err := s.templates.ExecuteTemplate(w, "imports.html", data); err != nil {
		log.Printf("Error rendering imports: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// SymbolResult represents a search result for a symbol
type SymbolResult struct {
	Name       string
	Kind       string // "func", "type", "method", "const", "var"
	Package    string
	ImportPath string
	Synopsis   string
	Deprecated bool
}

// handleSymbolSearch handles symbol search across all packages
func (s *Server) handleSymbolSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	kind := r.URL.Query().Get("kind") // func, type, method, const, var

	var results []SymbolResult

	if query != "" {
		queryLower := strings.ToLower(query)

		for _, pkg := range s.packages {
			// Search functions
			if kind == "" || kind == "func" {
				for _, fn := range pkg.Functions {
					if strings.Contains(strings.ToLower(fn.Name), queryLower) {
						results = append(results, SymbolResult{
							Name:       fn.Name,
							Kind:       "func",
							Package:    pkg.Name,
							ImportPath: pkg.ImportPath,
							Synopsis:   shortDoc(fn.Doc),
							Deprecated: fn.Deprecated,
						})
					}
				}
			}

			// Search types
			for _, t := range pkg.Types {
				if kind == "" || kind == "type" {
					if strings.Contains(strings.ToLower(t.Name), queryLower) {
						results = append(results, SymbolResult{
							Name:       t.Name,
							Kind:       "type",
							Package:    pkg.Name,
							ImportPath: pkg.ImportPath,
							Synopsis:   shortDoc(t.Doc),
							Deprecated: t.Deprecated,
						})
					}
				}

				// Search methods
				if kind == "" || kind == "method" {
					for _, m := range t.Methods {
						if strings.Contains(strings.ToLower(m.Name), queryLower) {
							results = append(results, SymbolResult{
								Name:       t.Name + "." + m.Name,
								Kind:       "method",
								Package:    pkg.Name,
								ImportPath: pkg.ImportPath,
								Synopsis:   shortDoc(m.Doc),
								Deprecated: m.Deprecated,
							})
						}
					}
				}

				// Search type funcs (constructors)
				if kind == "" || kind == "func" {
					for _, fn := range t.Functions {
						if strings.Contains(strings.ToLower(fn.Name), queryLower) {
							results = append(results, SymbolResult{
								Name:       fn.Name,
								Kind:       "func",
								Package:    pkg.Name,
								ImportPath: pkg.ImportPath,
								Synopsis:   shortDoc(fn.Doc),
								Deprecated: fn.Deprecated,
							})
						}
					}
				}
			}

			// Search constants
			if kind == "" || kind == "const" {
				for _, c := range pkg.Constants {
					for _, name := range c.Names {
						if strings.Contains(strings.ToLower(name), queryLower) {
							results = append(results, SymbolResult{
								Name:       name,
								Kind:       "const",
								Package:    pkg.Name,
								ImportPath: pkg.ImportPath,
								Synopsis:   shortDoc(c.Doc),
							})
						}
					}
				}
			}

			// Search variables
			if kind == "" || kind == "var" {
				for _, v := range pkg.Variables {
					for _, name := range v.Names {
						if strings.Contains(strings.ToLower(name), queryLower) {
							results = append(results, SymbolResult{
								Name:       name,
								Kind:       "var",
								Package:    pkg.Name,
								ImportPath: pkg.ImportPath,
								Synopsis:   shortDoc(v.Doc),
							})
						}
					}
				}
			}
		}
	}

	// Limit results
	if len(results) > 100 {
		results = results[:100]
	}

	data := struct {
		Title       string
		SearchQuery string
		Query       string
		Kind        string
		Results     []SymbolResult
	}{
		Title:       "Symbol Search - Go Packages",
		SearchQuery: query,
		Query:       query,
		Kind:        kind,
		Results:     results,
	}

	if err := s.templates.ExecuteTemplate(w, "symbols.html", data); err != nil {
		log.Printf("Error rendering symbols: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleModule handles the module info page
func (s *Server) handleModule(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/mod/")
	if path == "" {
		http.NotFound(w, r)
		return
	}

	// Find package
	pkg, ok := s.packages[path]
	if !ok {
		for importPath, p := range s.packages {
			if strings.HasSuffix(importPath, "/"+path) || importPath == path {
				pkg = p
				ok = true
				break
			}
		}
	}

	if !ok || pkg.GoModContent == "" {
		http.NotFound(w, r)
		return
	}

	data := struct {
		Title       string
		SearchQuery string
		Pkg         *PackageDoc
	}{
		Title:       "Module - " + pkg.ModulePath + " - Go Packages",
		SearchQuery: "",
		Pkg:         pkg,
	}

	if err := s.templates.ExecuteTemplate(w, "module.html", data); err != nil {
		log.Printf("Error rendering module: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleVersions handles the versions list page
// VersionInfo represents version information for display
type VersionInfo struct {
	Version   string
	Timestamp string
	IsTagged  bool
	IsStable  bool
	Retracted bool
	IsCurrent bool
}

func (s *Server) handleVersions(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/versions/")
	if path == "" {
		http.NotFound(w, r)
		return
	}

	// Find package
	pkg, ok := s.packages[path]
	if !ok {
		for importPath, p := range s.packages {
			if strings.HasSuffix(importPath, "/"+path) || importPath == path {
				pkg = p
				ok = true
				break
			}
		}
	}

	if !ok {
		http.NotFound(w, r)
		return
	}

	// Get version history from database if available
	var versions []VersionInfo
	if s.db != nil {
		dbVersions, err := s.db.GetModuleVersions(pkg.ModulePath)
		if err == nil && len(dbVersions) > 0 {
			for _, v := range dbVersions {
				vi := VersionInfo{
					Version:   v.Version,
					IsTagged:  v.IsTagged,
					IsStable:  v.IsStable,
					Retracted: v.Retracted,
					IsCurrent: v.Version == pkg.Version,
				}
				if !v.Timestamp.IsZero() {
					vi.Timestamp = v.Timestamp.Format("Jan 2, 2006")
				}
				versions = append(versions, vi)
			}
		}
	}

	// Fall back to package's Versions field if no database versions
	if len(versions) == 0 && len(pkg.Versions) > 0 {
		for _, v := range pkg.Versions {
			versions = append(versions, VersionInfo{
				Version:   v,
				IsCurrent: v == pkg.Version,
			})
		}
	}

	data := struct {
		Title       string
		SearchQuery string
		Pkg         *PackageDoc
		Versions    []VersionInfo
	}{
		Title:       "Versions - " + pkg.ImportPath + " - Go Packages",
		SearchQuery: "",
		Pkg:         pkg,
		Versions:    versions,
	}

	if err := s.templates.ExecuteTemplate(w, "versions.html", data); err != nil {
		log.Printf("Error rendering versions: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// ImportedByPackage represents a package that imports another package
type ImportedByPackage struct {
	ImportPath string
	Name       string
	Synopsis   string
	Module     string
}

// handleImportedBy handles the imported-by list page
func (s *Server) handleImportedBy(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/importedby/")
	if path == "" {
		http.NotFound(w, r)
		return
	}

	// Find package
	pkg, ok := s.packages[path]
	if !ok {
		for importPath, p := range s.packages {
			if strings.HasSuffix(importPath, "/"+path) || importPath == path {
				pkg = p
				path = importPath
				ok = true
				break
			}
		}
	}

	if !ok {
		http.NotFound(w, r)
		return
	}

	// Get pagination params
	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if n, err := fmt.Sscanf(p, "%d", &page); err != nil || n != 1 || page < 1 {
			page = 1
		}
	}
	perPage := 50
	offset := (page - 1) * perPage

	var importers []ImportedByPackage
	var total int

	if s.db != nil {
		// Get from database
		dbPkgs, count, err := s.db.GetImportedBy(path, perPage, offset)
		if err != nil {
			log.Printf("Error getting imported by: %v", err)
		} else {
			total = count
			for _, p := range dbPkgs {
				importers = append(importers, ImportedByPackage{
					ImportPath: p.ImportPath,
					Name:       p.Name,
					Synopsis:   p.Synopsis,
					Module:     p.ModulePath,
				})
			}
		}
	}

	totalPages := (total + perPage - 1) / perPage
	if totalPages < 1 {
		totalPages = 1
	}

	data := struct {
		Title       string
		SearchQuery string
		Pkg         *PackageDoc
		Importers   []ImportedByPackage
		Total       int
		Page        int
		TotalPages  int
		PerPage     int
		HasPrev     bool
		HasNext     bool
	}{
		Title:       "Imported By - " + pkg.ImportPath + " - Go Packages",
		SearchQuery: "",
		Pkg:         pkg,
		Importers:   importers,
		Total:       total,
		Page:        page,
		TotalPages:  totalPages,
		PerPage:     perPage,
		HasPrev:     page > 1,
		HasNext:     page < totalPages,
	}

	if err := s.templates.ExecuteTemplate(w, "importedby.html", data); err != nil {
		log.Printf("Error rendering imported by: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// Template helper functions

func formatDoc(doc string) string {
	return strings.TrimSpace(doc)
}

func firstLine(s string) string {
	if idx := strings.Index(s, "\n"); idx != -1 {
		return strings.TrimSpace(s[:idx])
	}
	return strings.TrimSpace(s)
}

func formatDocHTML(doc string) template.HTML {
	if doc == "" {
		return ""
	}

	// Convert doc comments to HTML
	lines := strings.Split(doc, "\n")
	var result strings.Builder
	var codeBlock strings.Builder
	inPre := false

	for _, line := range lines {
		// Detect code blocks (lines starting with tab or spaces)
		if strings.HasPrefix(line, "\t") || strings.HasPrefix(line, "    ") {
			if !inPre {
				inPre = true
				codeBlock.Reset()
			}
			// Remove leading tab/spaces for display
			line = strings.TrimPrefix(line, "\t")
			line = strings.TrimPrefix(line, "    ")
			codeBlock.WriteString(line)
			codeBlock.WriteString("\n")
		} else {
			if inPre {
				result.WriteString(`<pre><code class="language-go">`)
				result.WriteString(template.HTMLEscapeString(codeBlock.String()))
				result.WriteString("</code></pre>")
				inPre = false
			}
			if line == "" {
				result.WriteString("<p></p>")
			} else if strings.HasPrefix(line, "# ") {
				// Go 1.19+ doc headers
				header := strings.TrimPrefix(line, "# ")
				result.WriteString("<h3 class=\"Documentation-header\">")
				result.WriteString(template.HTMLEscapeString(header))
				result.WriteString("</h3>")
			} else if strings.HasPrefix(line, "## ") {
				header := strings.TrimPrefix(line, "## ")
				result.WriteString("<h4 class=\"Documentation-subheader\">")
				result.WriteString(template.HTMLEscapeString(header))
				result.WriteString("</h4>")
			} else {
				// Convert [Name] references to links
				processed := processDocLinks(line)
				result.WriteString("<p>")
				result.WriteString(processed)
				result.WriteString("</p>")
			}
		}
	}

	if inPre {
		result.WriteString(`<pre><code class="language-go">`)
		result.WriteString(template.HTMLEscapeString(codeBlock.String()))
		result.WriteString("</code></pre>")
	}

	return template.HTML(result.String())
}

func processDocLinks(text string) string {
	// First, escape HTML but preserve our special markers
	escaped := template.HTMLEscapeString(text)

	// Process URLs first (before other processing)
	escaped = autoLinkURLs(escaped)

	// Process cross-package type references (e.g., io.Reader, http.Handler)
	escaped = linkCrossPackageTypes(escaped)

	// Process [Name] references
	var result strings.Builder
	i := 0
	for i < len(escaped) {
		if escaped[i] == '[' {
			// Find closing bracket
			j := i + 1
			for j < len(escaped) && escaped[j] != ']' {
				j++
			}
			if j < len(escaped) {
				name := escaped[i+1 : j]
				// Create anchor link
				result.WriteString(`<a href="#`)
				result.WriteString(anchorName(name))
				result.WriteString(`">`)
				result.WriteString(name)
				result.WriteString(`</a>`)
				i = j + 1
				continue
			}
		}
		result.WriteByte(escaped[i])
		i++
	}
	return result.String()
}

// linkCrossPackageTypes detects and links cross-package type references
// like io.Reader, http.Handler, context.Context
func linkCrossPackageTypes(text string) string {
	// Standard library packages that are commonly referenced
	stdPkgs := map[string]bool{
		"bufio": true, "bytes": true, "context": true, "crypto": true,
		"encoding": true, "errors": true, "fmt": true, "hash": true,
		"io": true, "log": true, "math": true, "net": true, "os": true,
		"path": true, "reflect": true, "regexp": true, "runtime": true,
		"sort": true, "strconv": true, "strings": true, "sync": true,
		"syscall": true, "testing": true, "time": true, "unicode": true,
		"http": true, "url": true, "json": true, "xml": true, "sql": true,
		"template": true, "exec": true, "filepath": true, "zip": true,
		"tar": true, "gzip": true, "heap": true, "list": true, "ring": true,
	}

	var result strings.Builder
	i := 0
	for i < len(text) {
		// Check if we're at a word boundary and have a lowercase letter
		if i == 0 || !isIdentChar(text[i-1]) {
			// Try to match pattern: lowercase_identifier.UppercaseIdentifier
			j := i
			for j < len(text) && isLowerIdentChar(text[j]) {
				j++
			}
			if j > i && j < len(text) && text[j] == '.' {
				pkgName := text[i:j]
				if stdPkgs[pkgName] {
					k := j + 1
					// Type name must start with uppercase
					if k < len(text) && text[k] >= 'A' && text[k] <= 'Z' {
						for k < len(text) && isIdentChar(text[k]) {
							k++
						}
						typeName := text[j+1 : k]
						// Build the link
						pkgPath := getStdPkgPath(pkgName)
						result.WriteString(`<a href="/`)
						result.WriteString(pkgPath)
						result.WriteString(`#`)
						result.WriteString(typeName)
						result.WriteString(`" class="TypeLink">`)
						result.WriteString(pkgName)
						result.WriteString(".")
						result.WriteString(typeName)
						result.WriteString(`</a>`)
						i = k
						continue
					}
				}
			}
		}
		result.WriteByte(text[i])
		i++
	}
	return result.String()
}

func isIdentChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}

func isLowerIdentChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_'
}

// getStdPkgPath returns the full import path for common std lib short names
func getStdPkgPath(short string) string {
	paths := map[string]string{
		"http":     "net/http",
		"url":      "net/url",
		"json":     "encoding/json",
		"xml":      "encoding/xml",
		"sql":      "database/sql",
		"template": "text/template",
		"exec":     "os/exec",
		"filepath": "path/filepath",
		"zip":      "archive/zip",
		"tar":      "archive/tar",
		"gzip":     "compress/gzip",
		"heap":     "container/heap",
		"list":     "container/list",
		"ring":     "container/ring",
	}
	if full, ok := paths[short]; ok {
		return full
	}
	return short
}

func autoLinkURLs(text string) string {
	// Simple URL detection and auto-linking
	var result strings.Builder
	i := 0
	for i < len(text) {
		// Check for RFC references (RFC 1234)
		if i+4 < len(text) && text[i:i+4] == "RFC " {
			j := i + 4
			// Find RFC number
			for j < len(text) && text[j] >= '0' && text[j] <= '9' {
				j++
			}
			if j > i+4 {
				rfcNum := text[i+4 : j]
				result.WriteString(`<a href="https://www.rfc-editor.org/rfc/rfc`)
				result.WriteString(rfcNum)
				result.WriteString(`" target="_blank">RFC `)
				result.WriteString(rfcNum)
				result.WriteString(`</a>`)
				i = j
				continue
			}
		}
		// Check for http:// or https://
		if i+7 < len(text) && (text[i:i+7] == "http://" || (i+8 < len(text) && text[i:i+8] == "https://")) {
			// Find end of URL
			j := i
			for j < len(text) && !isURLTerminator(text[j]) {
				j++
			}
			// Trim trailing punctuation
			for j > i && (text[j-1] == '.' || text[j-1] == ',' || text[j-1] == ')' || text[j-1] == ';') {
				j--
			}
			url := text[i:j]
			result.WriteString(`<a href="`)
			result.WriteString(url)
			result.WriteString(`" target="_blank">`)
			result.WriteString(url)
			result.WriteString(`</a>`)
			i = j
			continue
		}
		result.WriteByte(text[i])
		i++
	}
	return result.String()
}

func isURLTerminator(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '<' || c == '>'
}

func shortDoc(doc string) string {
	// Return first sentence or first line
	if idx := strings.Index(doc, "."); idx != -1 {
		return strings.TrimSpace(doc[:idx+1])
	}
	if idx := strings.Index(doc, "\n"); idx != -1 {
		return strings.TrimSpace(doc[:idx])
	}
	return strings.TrimSpace(doc)
}

func anchorName(name string) string {
	// Convert name to valid HTML anchor
	return strings.ReplaceAll(name, " ", "-")
}

func sourceLink(importPath, filename string, line int) string {
	// Generate link to source code on Go's source browser
	// For standard library packages
	if !strings.Contains(importPath, ".") {
		if filename != "" && line > 0 {
			return fmt.Sprintf("https://cs.opensource.google/go/go/+/refs/tags/go1.23.0:src/%s/%s;l=%d", importPath, filename, line)
		}
		return "https://cs.opensource.google/go/go/+/refs/tags/go1.23.0:src/" + importPath + "/"
	}
	// For third-party packages, link to pkg.go.dev
	return "https://pkg.go.dev/" + importPath + "#section-sourcefiles"
}

func highlightQuery(text, query string) template.HTML {
	if query == "" {
		return template.HTML(template.HTMLEscapeString(text))
	}
	escaped := template.HTMLEscapeString(text)
	queryLower := strings.ToLower(query)
	textLower := strings.ToLower(escaped)

	var result strings.Builder
	i := 0
	for i < len(escaped) {
		idx := strings.Index(textLower[i:], queryLower)
		if idx == -1 {
			result.WriteString(escaped[i:])
			break
		}
		result.WriteString(escaped[i : i+idx])
		result.WriteString("<mark>")
		result.WriteString(escaped[i+idx : i+idx+len(query)])
		result.WriteString("</mark>")
		i = i + idx + len(query)
	}
	return template.HTML(result.String())
}

// DiffEntry represents a single API change between versions
type DiffEntry struct {
	Kind      string // "added", "removed", "changed"
	Type      string // "func", "type", "method", "const", "var"
	Name      string
	OldDecl   string
	NewDecl   string
	Synopsis  string
}

// handleDiff handles the API diff between two versions of a package
func (s *Server) handleDiff(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/diff/")
	if path == "" {
		http.NotFound(w, r)
		return
	}

	v1 := r.URL.Query().Get("v1")
	v2 := r.URL.Query().Get("v2")

	// Find package
	pkg, ok := s.packages[path]
	if !ok {
		for importPath, p := range s.packages {
			if strings.HasSuffix(importPath, "/"+path) || importPath == path {
				pkg = p
				ok = true
				break
			}
		}
	}

	if !ok {
		http.NotFound(w, r)
		return
	}

	// Get available versions
	var versions []VersionInfo
	if s.db != nil {
		dbVersions, err := s.db.GetModuleVersions(pkg.ModulePath)
		if err == nil {
			for _, v := range dbVersions {
				vi := VersionInfo{
					Version:   v.Version,
					IsTagged:  v.IsTagged,
					IsStable:  v.IsStable,
					IsCurrent: v.Version == pkg.Version,
				}
				if !v.Timestamp.IsZero() {
					vi.Timestamp = v.Timestamp.Format("Jan 2, 2006")
				}
				versions = append(versions, vi)
			}
		}
	}

	// Calculate diff if both versions are provided
	var diff []DiffEntry
	if v1 != "" && v2 != "" {
		diff = s.calculateDiff(pkg, v1, v2)
	}

	data := struct {
		Title       string
		SearchQuery string
		Pkg         *PackageDoc
		Versions    []VersionInfo
		V1          string
		V2          string
		Diff        []DiffEntry
		HasDiff     bool
	}{
		Title:       "API Diff - " + pkg.ImportPath + " - Go Packages",
		SearchQuery: "",
		Pkg:         pkg,
		Versions:    versions,
		V1:          v1,
		V2:          v2,
		Diff:        diff,
		HasDiff:     v1 != "" && v2 != "",
	}

	if err := s.templates.ExecuteTemplate(w, "diff.html", data); err != nil {
		log.Printf("Error rendering diff: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// calculateDiff calculates the API difference between two versions
func (s *Server) calculateDiff(pkg *PackageDoc, v1, v2 string) []DiffEntry {
	var diff []DiffEntry

	// For now, we compare the current package documentation
	// In a full implementation, we would fetch both versions from proxy.golang.org
	// and compare their symbols

	// Get symbols from current package as a baseline
	currentSymbols := make(map[string]string)

	for _, f := range pkg.Functions {
		currentSymbols["func:"+f.Name] = f.Signature
	}
	for _, t := range pkg.Types {
		currentSymbols["type:"+t.Name] = t.Decl
		for _, m := range t.Methods {
			currentSymbols["method:"+t.Name+"."+m.Name] = m.Signature
		}
		for _, f := range t.Functions {
			currentSymbols["func:"+f.Name] = f.Signature
		}
	}
	for _, c := range pkg.Constants {
		for _, name := range c.Names {
			currentSymbols["const:"+name] = ""
		}
	}
	for _, v := range pkg.Variables {
		for _, name := range v.Names {
			currentSymbols["var:"+name] = ""
		}
	}

	// Since we only have current version data, show it as informational
	// In production, this would compare actual version-specific data
	if v1 != v2 {
		diff = append(diff, DiffEntry{
			Kind:     "info",
			Type:     "note",
			Name:     "Version Comparison",
			Synopsis: fmt.Sprintf("Comparing %s to %s. Full diff requires version-specific symbol storage.", v1, v2),
		})

		// Show current symbols as reference
		for _, f := range pkg.Functions {
			diff = append(diff, DiffEntry{
				Kind:     "unchanged",
				Type:     "func",
				Name:     f.Name,
				NewDecl:  f.Signature,
				Synopsis: firstLine(f.Doc),
			})
		}

		for _, t := range pkg.Types {
			diff = append(diff, DiffEntry{
				Kind:     "unchanged",
				Type:     "type",
				Name:     t.Name,
				NewDecl:  t.Decl,
				Synopsis: firstLine(t.Doc),
			})
		}
	}

	return diff
}

// handleCompare handles the package comparison view
func (s *Server) handleCompare(w http.ResponseWriter, r *http.Request) {
	pkg1Path := r.URL.Query().Get("pkg1")
	pkg2Path := r.URL.Query().Get("pkg2")

	var pkg1, pkg2 *PackageDoc

	if pkg1Path != "" {
		if p, ok := s.packages[pkg1Path]; ok {
			pkg1 = p
		}
	}

	if pkg2Path != "" {
		if p, ok := s.packages[pkg2Path]; ok {
			pkg2 = p
		}
	}

	// Get list of all packages for selection
	var allPackages []string
	for path := range s.packages {
		allPackages = append(allPackages, path)
	}

	// Compare packages if both are selected
	var comparison []DiffEntry
	if pkg1 != nil && pkg2 != nil {
		comparison = s.comparePackages(pkg1, pkg2)
	}

	data := struct {
		Title       string
		SearchQuery string
		AllPackages []string
		Pkg1Path    string
		Pkg2Path    string
		Pkg1        *PackageDoc
		Pkg2        *PackageDoc
		Comparison  []DiffEntry
		HasCompare  bool
	}{
		Title:       "Compare Packages - Go Packages",
		SearchQuery: "",
		AllPackages: allPackages,
		Pkg1Path:    pkg1Path,
		Pkg2Path:    pkg2Path,
		Pkg1:        pkg1,
		Pkg2:        pkg2,
		Comparison:  comparison,
		HasCompare:  pkg1 != nil && pkg2 != nil,
	}

	if err := s.templates.ExecuteTemplate(w, "compare.html", data); err != nil {
		log.Printf("Error rendering compare: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// comparePackages compares the APIs of two packages
func (s *Server) comparePackages(pkg1, pkg2 *PackageDoc) []DiffEntry {
	var diff []DiffEntry

	// Build symbol maps for both packages
	pkg1Symbols := make(map[string]string)
	pkg2Symbols := make(map[string]string)

	// Package 1 symbols
	for _, f := range pkg1.Functions {
		pkg1Symbols["func:"+f.Name] = f.Signature
	}
	for _, t := range pkg1.Types {
		pkg1Symbols["type:"+t.Name] = t.Decl
		for _, m := range t.Methods {
			pkg1Symbols["method:"+t.Name+"."+m.Name] = m.Signature
		}
	}

	// Package 2 symbols
	for _, f := range pkg2.Functions {
		pkg2Symbols["func:"+f.Name] = f.Signature
	}
	for _, t := range pkg2.Types {
		pkg2Symbols["type:"+t.Name] = t.Decl
		for _, m := range t.Methods {
			pkg2Symbols["method:"+t.Name+"."+m.Name] = m.Signature
		}
	}

	// Find symbols only in pkg1
	for key, decl := range pkg1Symbols {
		parts := strings.SplitN(key, ":", 2)
		if _, exists := pkg2Symbols[key]; !exists {
			diff = append(diff, DiffEntry{
				Kind:    "only-left",
				Type:    parts[0],
				Name:    parts[1],
				OldDecl: decl,
			})
		}
	}

	// Find symbols only in pkg2 or changed
	for key, decl := range pkg2Symbols {
		parts := strings.SplitN(key, ":", 2)
		if oldDecl, exists := pkg1Symbols[key]; !exists {
			diff = append(diff, DiffEntry{
				Kind:    "only-right",
				Type:    parts[0],
				Name:    parts[1],
				NewDecl: decl,
			})
		} else if oldDecl != decl {
			diff = append(diff, DiffEntry{
				Kind:    "changed",
				Type:    parts[0],
				Name:    parts[1],
				OldDecl: oldDecl,
				NewDecl: decl,
			})
		}
	}

	return diff
}
