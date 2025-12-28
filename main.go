package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/doc"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"golang.org/x/tools/go/packages"
)

// PackageDoc represents complete documentation for a Go package
type PackageDoc struct {
	ImportPath       string     `json:"import_path"`
	Name             string     `json:"name"`
	Doc              string     `json:"doc"`
	Synopsis         string     `json:"synopsis"`
	Version          string     `json:"version,omitempty"`
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

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: wikigo <package-path>")
		fmt.Fprintln(os.Stderr, "Example: wikigo net/http")
		os.Exit(1)
	}

	pkgPath := os.Args[1]

	pkgDoc, err := ExtractPackageDoc(pkgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error extracting package: %v\n", err)
		os.Exit(1)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(pkgDoc); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
		os.Exit(1)
	}
}

// ExtractPackageDoc extracts all documentation from a Go package
func ExtractPackageDoc(pkgPath string) (*PackageDoc, error) {
	// Use our own FileSet for consistency
	fset := token.NewFileSet()

	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedImports,
	}

	pkgs, err := packages.Load(cfg, pkgPath)
	if err != nil {
		return nil, fmt.Errorf("loading package: %w", err)
	}

	if len(pkgs) == 0 {
		return nil, fmt.Errorf("no packages found for %s", pkgPath)
	}

	pkg := pkgs[0]
	if len(pkg.Errors) > 0 {
		return nil, fmt.Errorf("package errors: %v", pkg.Errors[0])
	}

	// Parse all Go files in the package directory
	var files []*ast.File
	var testFiles []*ast.File
	var filenames []string

	// Get the directory from the first file
	if len(pkg.GoFiles) == 0 {
		return nil, fmt.Errorf("no Go files found in package")
	}

	pkgDir := filepath.Dir(pkg.GoFiles[0])

	// Parse all .go files in the directory
	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		return nil, fmt.Errorf("reading package directory: %w", err)
	}

	// Determine the expected package name from the import path
	expectedPkgName := filepath.Base(pkgPath)

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}

		fullPath := filepath.Join(pkgDir, entry.Name())
		f, err := parser.ParseFile(fset, fullPath, nil, parser.ParseComments)
		if err != nil {
			continue // Skip files that fail to parse
		}

		// Skip files that don't belong to the main package (e.g., example_test.go with package main)
		pkgName := f.Name.Name
		isTestFile := strings.HasSuffix(entry.Name(), "_test.go")

		if isTestFile {
			// Test files can have package name or package name_test
			if pkgName == expectedPkgName || pkgName == expectedPkgName+"_test" {
				testFiles = append(testFiles, f)
			}
		} else {
			// Regular files must match package name
			if pkgName == expectedPkgName {
				files = append(files, f)
				filenames = append(filenames, fullPath)
			}
		}
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no parseable Go files found")
	}

	// Create documentation
	docPkg, err := doc.NewFromFiles(fset, files, pkgPath, doc.AllDecls|doc.AllMethods)
	if err != nil {
		return nil, fmt.Errorf("creating doc: %w", err)
	}

	// Extract examples from test files
	var examples []*doc.Example
	for _, f := range testFiles {
		examples = append(examples, doc.Examples(f)...)
	}

	// Detect license
	license, licenseText := detectLicense(pkgDir)

	// Detect repository
	repository := detectRepository(pkgPath, pkgDir)

	// Detect go.mod info
	hasValidMod, goVersion, modulePath, goModContent := detectGoMod(pkgDir)

	// Detect version
	version := detectVersion(pkgDir, modulePath)

	// Determine if version is tagged and stable
	isTagged, isStable := analyzeVersion(version)

	// Detect published date (most recent file modification)
	publishedAt := detectPublishedDate(filenames)

	// Build result
	result := &PackageDoc{
		ImportPath:      pkgPath,
		Name:            docPkg.Name,
		Doc:             docPkg.Doc,
		Synopsis:        doc.Synopsis(docPkg.Doc),
		Version:         version,
		IsTagged:        isTagged,
		IsStable:        isStable,
		PublishedAt:     publishedAt,
		License:         license,
		LicenseText:     licenseText,
		Redistributable: isRedistributable(license),
		Repository:      repository,
		HasValidMod:     hasValidMod,
		GoVersion:       goVersion,
		ModulePath:      modulePath,
		GoModContent:    goModContent,
		Filenames:       filenames,
	}

	// Extract build constraints from filenames
	goos, goarch := extractBuildConstraints(filenames)
	result.GOOS = goos
	result.GOARCH = goarch

	// Extract imports
	for imp := range pkg.Imports {
		result.Imports = append(result.Imports, imp)
	}

	// Extract constants
	for _, c := range docPkg.Consts {
		result.Constants = append(result.Constants, Constant{
			Names: c.Names,
			Doc:   c.Doc,
			Decl:  formatDecl(fset, c.Decl),
		})
	}

	// Extract variables
	for _, v := range docPkg.Vars {
		result.Variables = append(result.Variables, Variable{
			Names: v.Names,
			Doc:   v.Doc,
			Decl:  formatDecl(fset, v.Decl),
		})
	}

	// Extract functions
	for _, f := range docPkg.Funcs {
		pos := fset.Position(f.Decl.Pos())
		fn := Function{
			Name:       f.Name,
			Doc:        f.Doc,
			Signature:  formatFuncSignature(f.Decl),
			Filename:   filepath.Base(pos.Filename),
			Line:       pos.Line,
			Deprecated: isDeprecated(f.Doc),
		}
		fn.Examples = findExamples(examples, f.Name, fset)
		result.Functions = append(result.Functions, fn)
	}

	// Extract types
	for _, t := range docPkg.Types {
		typePos := fset.Position(t.Decl.Pos())
		typ := Type{
			Name:       t.Name,
			Doc:        t.Doc,
			Decl:       formatDecl(fset, t.Decl),
			Filename:   filepath.Base(typePos.Filename),
			Line:       typePos.Line,
			Deprecated: isDeprecated(t.Doc),
		}

		// Type-associated constants
		for _, c := range t.Consts {
			typ.Constants = append(typ.Constants, Constant{
				Names: c.Names,
				Doc:   c.Doc,
				Decl:  formatDecl(fset, c.Decl),
			})
		}

		// Type-associated variables
		for _, v := range t.Vars {
			typ.Variables = append(typ.Variables, Variable{
				Names: v.Names,
				Doc:   v.Doc,
				Decl:  formatDecl(fset, v.Decl),
			})
		}

		// Constructor functions
		for _, f := range t.Funcs {
			pos := fset.Position(f.Decl.Pos())
			fn := Function{
				Name:       f.Name,
				Doc:        f.Doc,
				Signature:  formatFuncSignature(f.Decl),
				Filename:   filepath.Base(pos.Filename),
				Line:       pos.Line,
				Deprecated: isDeprecated(f.Doc),
			}
			fn.Examples = findExamples(examples, f.Name, fset)
			typ.Functions = append(typ.Functions, fn)
		}

		// Methods
		for _, m := range t.Methods {
			pos := fset.Position(m.Decl.Pos())
			method := Function{
				Name:       m.Name,
				Doc:        m.Doc,
				Signature:  formatFuncSignature(m.Decl),
				Recv:       m.Recv,
				Filename:   filepath.Base(pos.Filename),
				Line:       pos.Line,
				Deprecated: isDeprecated(m.Doc),
			}
			method.Examples = findExamples(examples, t.Name+"_"+m.Name, fset)
			typ.Methods = append(typ.Methods, method)
		}

		// Type examples
		typ.Examples = findExamples(examples, t.Name, fset)

		result.Types = append(result.Types, typ)
	}

	// Package-level examples
	result.Examples = findExamples(examples, "", fset)

	return result, nil
}

// formatDecl formats a declaration node as a string
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

// formatFuncSignature formats a function declaration as a signature string
func formatFuncSignature(decl *ast.FuncDecl) string {
	if decl == nil {
		return ""
	}

	var buf strings.Builder
	buf.WriteString("func ")

	// Receiver
	if decl.Recv != nil && len(decl.Recv.List) > 0 {
		buf.WriteString("(")
		recv := decl.Recv.List[0]
		if len(recv.Names) > 0 {
			buf.WriteString(recv.Names[0].Name)
			buf.WriteString(" ")
		}
		buf.WriteString(formatExpr(recv.Type))
		buf.WriteString(") ")
	}

	buf.WriteString(decl.Name.Name)
	buf.WriteString(formatFuncType(decl.Type))

	return buf.String()
}

// formatFuncType formats a function type (parameters and return values)
func formatFuncType(ft *ast.FuncType) string {
	if ft == nil {
		return "()"
	}

	var buf strings.Builder

	// Parameters
	buf.WriteString("(")
	if ft.Params != nil {
		buf.WriteString(formatFieldList(ft.Params.List))
	}
	buf.WriteString(")")

	// Return values
	if ft.Results != nil && len(ft.Results.List) > 0 {
		buf.WriteString(" ")
		if len(ft.Results.List) == 1 && len(ft.Results.List[0].Names) == 0 {
			buf.WriteString(formatExpr(ft.Results.List[0].Type))
		} else {
			buf.WriteString("(")
			buf.WriteString(formatFieldList(ft.Results.List))
			buf.WriteString(")")
		}
	}

	return buf.String()
}

// formatFieldList formats a list of fields (parameters or results)
func formatFieldList(fields []*ast.Field) string {
	var parts []string
	for _, f := range fields {
		typeStr := formatExpr(f.Type)
		if len(f.Names) == 0 {
			parts = append(parts, typeStr)
		} else {
			var names []string
			for _, n := range f.Names {
				names = append(names, n.Name)
			}
			parts = append(parts, strings.Join(names, ", ")+" "+typeStr)
		}
	}
	return strings.Join(parts, ", ")
}

// formatExpr formats an expression as a string
func formatExpr(expr ast.Expr) string {
	if expr == nil {
		return ""
	}

	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return formatExpr(e.X) + "." + e.Sel.Name
	case *ast.StarExpr:
		return "*" + formatExpr(e.X)
	case *ast.ArrayType:
		if e.Len == nil {
			return "[]" + formatExpr(e.Elt)
		}
		return "[" + formatExpr(e.Len) + "]" + formatExpr(e.Elt)
	case *ast.MapType:
		return "map[" + formatExpr(e.Key) + "]" + formatExpr(e.Value)
	case *ast.ChanType:
		switch e.Dir {
		case ast.SEND:
			return "chan<- " + formatExpr(e.Value)
		case ast.RECV:
			return "<-chan " + formatExpr(e.Value)
		default:
			return "chan " + formatExpr(e.Value)
		}
	case *ast.FuncType:
		return "func" + formatFuncType(e)
	case *ast.InterfaceType:
		if e.Methods == nil || len(e.Methods.List) == 0 {
			return "interface{}"
		}
		return "interface{ ... }"
	case *ast.StructType:
		if e.Fields == nil || len(e.Fields.List) == 0 {
			return "struct{}"
		}
		return "struct{ ... }"
	case *ast.Ellipsis:
		return "..." + formatExpr(e.Elt)
	case *ast.BasicLit:
		return e.Value
	case *ast.ParenExpr:
		return "(" + formatExpr(e.X) + ")"
	case *ast.IndexExpr:
		return formatExpr(e.X) + "[" + formatExpr(e.Index) + "]"
	case *ast.IndexListExpr:
		var indices []string
		for _, idx := range e.Indices {
			indices = append(indices, formatExpr(idx))
		}
		return formatExpr(e.X) + "[" + strings.Join(indices, ", ") + "]"
	default:
		return fmt.Sprintf("%T", expr)
	}
}

// findExamples finds examples matching a given name
func findExamples(examples []*doc.Example, name string, fset *token.FileSet) []Example {
	var result []Example
	for _, ex := range examples {
		exName := ex.Name
		match := false

		if name == "" && exName == "" {
			match = true
		} else if name != "" && (exName == name || strings.HasPrefix(exName, name+"_")) {
			match = true
		}

		if match {
			code := formatDecl(fset, ex.Code)
			if code == "" && ex.Play != nil {
				code = formatDecl(fset, ex.Play)
			}

			result = append(result, Example{
				Name:   exName,
				Doc:    ex.Doc,
				Code:   code,
				Output: ex.Output,
			})
		}
	}
	return result
}

// detectLicense looks for a license file and identifies the license type
func detectLicense(dir string) (licenseType string, licenseText string) {
	// Walk up directories to find LICENSE file (for module root)
	currentDir := dir
	for i := 0; i < 10; i++ { // Limit depth
		licenseType, licenseText = findLicenseInDir(currentDir)
		if licenseType != "" {
			return licenseType, licenseText
		}
		parent := filepath.Dir(currentDir)
		if parent == currentDir {
			break
		}
		currentDir = parent
	}
	return "", ""
}

func findLicenseInDir(dir string) (licenseType string, licenseText string) {
	licenseFiles := []string{
		"LICENSE", "LICENSE.txt", "LICENSE.md",
		"LICENCE", "LICENCE.txt", "LICENCE.md",
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

	// Check for common license patterns
	switch {
	case strings.Contains(content, "apache license") && strings.Contains(content, "version 2.0"):
		return "Apache-2.0"
	case strings.Contains(content, "mit license") || strings.Contains(content, "permission is hereby granted, free of charge"):
		return "MIT"
	case strings.Contains(content, "bsd 3-clause") || (strings.Contains(content, "redistribution and use") && strings.Contains(content, "neither the name")):
		return "BSD-3-Clause"
	case strings.Contains(content, "bsd 2-clause") || (strings.Contains(content, "redistribution and use") && !strings.Contains(content, "neither the name") && !strings.Contains(content, "advertising")):
		return "BSD-2-Clause"
	case strings.Contains(content, "gnu general public license") && strings.Contains(content, "version 3"):
		return "GPL-3.0"
	case strings.Contains(content, "gnu general public license") && strings.Contains(content, "version 2"):
		return "GPL-2.0"
	case strings.Contains(content, "gnu lesser general public license"):
		return "LGPL"
	case strings.Contains(content, "mozilla public license") && strings.Contains(content, "2.0"):
		return "MPL-2.0"
	case strings.Contains(content, "unlicense") || strings.Contains(content, "this is free and unencumbered"):
		return "Unlicense"
	case strings.Contains(content, "isc license"):
		return "ISC"
	case strings.Contains(content, "creative commons"):
		if strings.Contains(content, "cc0") {
			return "CC0-1.0"
		}
		return "CC"
	}

	return "Unknown"
}

// isRedistributable checks if a license allows redistribution
func isRedistributable(license string) bool {
	redistributable := map[string]bool{
		"MIT":          true,
		"Apache-2.0":   true,
		"BSD-2-Clause": true,
		"BSD-3-Clause": true,
		"ISC":          true,
		"MPL-2.0":      true,
		"Unlicense":    true,
		"CC0-1.0":      true,
		"LGPL":         true,
	}
	return redistributable[license]
}

// detectGoMod checks for a valid go.mod and extracts Go version, module path, and content
func detectGoMod(pkgDir string) (hasValidMod bool, goVersion string, modulePath string, goModContent string) {
	currentDir := pkgDir
	for i := 0; i < 10; i++ {
		gomodPath := filepath.Join(currentDir, "go.mod")
		content, err := os.ReadFile(gomodPath)
		if err == nil {
			goModContent = string(content)
			for _, line := range strings.Split(goModContent, "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "module ") {
					hasValidMod = true
					modulePath = strings.TrimSpace(strings.TrimPrefix(line, "module "))
				}
				if strings.HasPrefix(line, "go ") {
					goVersion = strings.TrimSpace(strings.TrimPrefix(line, "go "))
				}
			}
			return hasValidMod, goVersion, modulePath, goModContent
		}
		parent := filepath.Dir(currentDir)
		if parent == currentDir {
			break
		}
		currentDir = parent
	}
	return false, "", "", ""
}

// detectRepository detects the repository URL from the import path or go.mod
func detectRepository(importPath, pkgDir string) string {
	// Try to find go.mod and extract module path
	modulePath := findModulePath(pkgDir)
	if modulePath == "" {
		modulePath = importPath
	}

	// Convert module path to repository URL
	return moduleToRepoURL(modulePath)
}

func findModulePath(dir string) string {
	currentDir := dir
	for i := 0; i < 10; i++ {
		gomodPath := filepath.Join(currentDir, "go.mod")
		content, err := os.ReadFile(gomodPath)
		if err == nil {
			// Parse module line
			for _, line := range strings.Split(string(content), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "module ") {
					return strings.TrimSpace(strings.TrimPrefix(line, "module"))
				}
			}
		}
		parent := filepath.Dir(currentDir)
		if parent == currentDir {
			break
		}
		currentDir = parent
	}
	return ""
}

func moduleToRepoURL(modulePath string) string {
	// Handle common hosting services
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
	case host == "golang.org" && len(parts) >= 2 && parts[1] == "x":
		if len(parts) >= 3 {
			return "https://go.googlesource.com/" + parts[2]
		}
	case strings.Contains(host, "."):
		// Generic: assume https://host/path
		if len(parts) >= 3 {
			return "https://" + parts[0] + "/" + parts[1] + "/" + parts[2]
		}
	}

	return ""
}

// isDeprecated checks if a doc comment indicates deprecation
func isDeprecated(doc string) bool {
	doc = strings.TrimSpace(doc)
	// Check for "Deprecated:" at start of doc or after paragraph break
	if strings.HasPrefix(doc, "Deprecated:") {
		return true
	}
	// Also check for "Deprecated:" on its own line
	return strings.Contains(doc, "\nDeprecated:") || strings.Contains(doc, "\n\nDeprecated:")
}

// extractBuildConstraints extracts GOOS and GOARCH from filenames
func extractBuildConstraints(filenames []string) (goos []string, goarch []string) {
	validGOOS := map[string]bool{
		"aix": true, "android": true, "darwin": true, "dragonfly": true,
		"freebsd": true, "illumos": true, "ios": true, "js": true,
		"linux": true, "netbsd": true, "openbsd": true, "plan9": true,
		"solaris": true, "wasip1": true, "windows": true,
	}
	validGOARCH := map[string]bool{
		"386": true, "amd64": true, "arm": true, "arm64": true,
		"loong64": true, "mips": true, "mips64": true, "mips64le": true,
		"mipsle": true, "ppc64": true, "ppc64le": true, "riscv64": true,
		"s390x": true, "wasm": true,
	}

	goosSet := make(map[string]bool)
	goarchSet := make(map[string]bool)

	for _, filename := range filenames {
		base := filepath.Base(filename)
		base = strings.TrimSuffix(base, ".go")

		parts := strings.Split(base, "_")
		for _, part := range parts {
			if validGOOS[part] {
				goosSet[part] = true
			}
			if validGOARCH[part] {
				goarchSet[part] = true
			}
		}
	}

	for os := range goosSet {
		goos = append(goos, os)
	}
	for arch := range goarchSet {
		goarch = append(goarch, arch)
	}

	return goos, goarch
}

// detectVersion tries to detect the package version from git tags or go.mod
func detectVersion(pkgDir string, modulePath string) string {
	// First, try to get version from git tags
	version := detectGitVersion(pkgDir)
	if version != "" {
		return version
	}

	// Check for version suffix in module path (e.g., /v2, /v3)
	if modulePath != "" {
		versionRegex := regexp.MustCompile(`/v(\d+)$`)
		if matches := versionRegex.FindStringSubmatch(modulePath); len(matches) > 1 {
			return "v" + matches[1] + ".x"
		}
	}

	return ""
}

// detectGitVersion tries to get version from git describe
func detectGitVersion(dir string) string {
	// Try git describe to get the most recent tag
	cmd := exec.Command("git", "describe", "--tags", "--abbrev=0")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err == nil {
		version := strings.TrimSpace(string(output))
		if version != "" {
			return version
		}
	}

	// Try to get the latest tag from git tag -l
	cmd = exec.Command("git", "tag", "-l", "--sort=-v:refname")
	cmd.Dir = dir
	output, err = cmd.Output()
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && (strings.HasPrefix(line, "v") || regexp.MustCompile(`^\d+\.\d+`).MatchString(line)) {
				return line
			}
		}
	}

	return ""
}

// detectPublishedDate returns the most recent modification time of the source files
func detectPublishedDate(filenames []string) string {
	var latestTime time.Time

	for _, filename := range filenames {
		info, err := os.Stat(filename)
		if err != nil {
			continue
		}
		if info.ModTime().After(latestTime) {
			latestTime = info.ModTime()
		}
	}

	if latestTime.IsZero() {
		return ""
	}

	return latestTime.Format("Jan 2, 2006")
}

// analyzeVersion determines if a version string represents a tagged and stable release
func analyzeVersion(version string) (isTagged bool, isStable bool) {
	if version == "" {
		return false, false
	}

	// Semver pattern: v1.2.3 or v1.2.3-pre.1+build
	semverRegex := regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)(-[a-zA-Z0-9.-]+)?(\+[a-zA-Z0-9.-]+)?$`)
	matches := semverRegex.FindStringSubmatch(version)

	if matches == nil {
		// Not a proper semver tag
		return false, false
	}

	isTagged = true

	// Check if stable: major >= 1 and no pre-release suffix
	major := matches[1]
	prerelease := matches[4]

	if major != "0" && prerelease == "" {
		isStable = true
	}

	return isTagged, isStable
}
