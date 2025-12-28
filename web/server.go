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
	License          string     `json:"license,omitempty"`
	Redistributable  bool       `json:"redistributable,omitempty"`
	Repository       string     `json:"repository,omitempty"`
	HasValidMod      bool       `json:"has_valid_mod,omitempty"`
	GoVersion        string     `json:"go_version,omitempty"`
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
	Name      string    `json:"name"`
	Doc       string    `json:"doc"`
	Signature string    `json:"signature"`
	Recv      string    `json:"recv,omitempty"`
	Filename  string    `json:"filename,omitempty"`
	Line      int       `json:"line,omitempty"`
	Examples  []Example `json:"examples,omitempty"`
}

// Type represents a documented type
type Type struct {
	Name      string     `json:"name"`
	Doc       string     `json:"doc"`
	Decl      string     `json:"decl"`
	Filename  string     `json:"filename,omitempty"`
	Line      int        `json:"line,omitempty"`
	Constants []Constant `json:"constants,omitempty"`
	Variables []Variable `json:"variables,omitempty"`
	Functions []Function `json:"funcs,omitempty"`
	Methods   []Function `json:"methods,omitempty"`
	Examples  []Example  `json:"examples,omitempty"`
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
}

// NewServer creates a new documentation server
func NewServer(dataDir string) (*Server, error) {
	s := &Server{
		packages: make(map[string]*PackageDoc),
		dataDir:  dataDir,
	}

	// Parse templates
	funcMap := template.FuncMap{
		"formatDoc":     formatDoc,
		"formatDocHTML": formatDocHTML,
		"shortDoc":      shortDoc,
		"baseName":      filepath.Base,
		"hasPrefix":     strings.HasPrefix,
		"trimPrefix":    strings.TrimPrefix,
		"join":          strings.Join,
		"lower":         strings.ToLower,
		"anchorName":    anchorName,
		"sourceLink":    sourceLink,
		"split":         strings.Split,
		"sub":           func(a, b int) int { return a - b },
		"cond":          func(cond bool, t, f string) string { if cond { return t }; return f },
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

	data := struct {
		Title          string
		SearchQuery    string
		Pkg            *PackageDoc
		Subdirectories []Subdirectory
	}{
		Title:          pkg.Name + " package - " + pkg.ImportPath + " - Go Packages",
		SearchQuery:    "",
		Pkg:            pkg,
		Subdirectories: subdirs,
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

// Template helper functions

func formatDoc(doc string) string {
	return strings.TrimSpace(doc)
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
