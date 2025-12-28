package db

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setupTestDB(t *testing.T) *DB {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	return db
}

func TestOpen(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	// Verify database file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("database file was not created")
	}

	// Verify tables exist
	var count int
	err = db.conn.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table'").Scan(&count)
	if err != nil {
		t.Fatalf("failed to query tables: %v", err)
	}
	if count == 0 {
		t.Error("no tables were created")
	}
}

func TestUpsertPackage_Insert(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pkg := &Package{
		ImportPath:      "github.com/test/pkg",
		Name:            "pkg",
		Synopsis:        "Test package",
		Doc:             "Package pkg is a test package",
		Version:         "v1.0.0",
		Versions:        []string{"v1.0.0", "v0.9.0"},
		IsTagged:        true,
		IsStable:        true,
		License:         "MIT",
		LicenseText:     "MIT License text",
		Redistributable: true,
		Repository:      "https://github.com/test/pkg",
		HasValidMod:     true,
		GoVersion:       "1.21",
		ModulePath:      "github.com/test/pkg",
		GOOS:            []string{"linux", "darwin"},
		GOARCH:          []string{"amd64", "arm64"},
		DocJSON:         `{"name":"pkg"}`,
	}

	id, err := db.UpsertPackage(pkg)
	if err != nil {
		t.Fatalf("UpsertPackage() error = %v", err)
	}
	if id == 0 {
		t.Error("UpsertPackage() returned zero ID")
	}

	// Verify package was inserted
	retrieved, err := db.GetPackage("github.com/test/pkg")
	if err != nil {
		t.Fatalf("GetPackage() error = %v", err)
	}
	if retrieved == nil {
		t.Fatal("GetPackage() returned nil")
	}

	if retrieved.ImportPath != pkg.ImportPath {
		t.Errorf("ImportPath = %v, want %v", retrieved.ImportPath, pkg.ImportPath)
	}
	if retrieved.Name != pkg.Name {
		t.Errorf("Name = %v, want %v", retrieved.Name, pkg.Name)
	}
	if retrieved.Synopsis != pkg.Synopsis {
		t.Errorf("Synopsis = %v, want %v", retrieved.Synopsis, pkg.Synopsis)
	}
	if len(retrieved.Versions) != len(pkg.Versions) {
		t.Errorf("Versions length = %v, want %v", len(retrieved.Versions), len(pkg.Versions))
	}
	if len(retrieved.GOOS) != len(pkg.GOOS) {
		t.Errorf("GOOS length = %v, want %v", len(retrieved.GOOS), len(pkg.GOOS))
	}
}

func TestUpsertPackage_Update(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pkg := &Package{
		ImportPath: "github.com/test/pkg",
		Name:       "pkg",
		Synopsis:   "Original synopsis",
		Version:    "v1.0.0",
	}

	id1, err := db.UpsertPackage(pkg)
	if err != nil {
		t.Fatalf("UpsertPackage() initial insert error = %v", err)
	}

	// Update the package
	pkg.Synopsis = "Updated synopsis"
	pkg.Version = "v1.1.0"
	id2, err := db.UpsertPackage(pkg)
	if err != nil {
		t.Fatalf("UpsertPackage() update error = %v", err)
	}

	// ID should be the same (update, not insert)
	if id1 != id2 {
		t.Errorf("ID changed after update: %v -> %v", id1, id2)
	}

	// Verify update
	retrieved, err := db.GetPackage("github.com/test/pkg")
	if err != nil {
		t.Fatalf("GetPackage() error = %v", err)
	}
	if retrieved.Synopsis != "Updated synopsis" {
		t.Errorf("Synopsis = %v, want %v", retrieved.Synopsis, "Updated synopsis")
	}
	if retrieved.Version != "v1.1.0" {
		t.Errorf("Version = %v, want %v", retrieved.Version, "v1.1.0")
	}
}

func TestGetPackage_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pkg, err := db.GetPackage("nonexistent/package")
	if err != nil {
		t.Fatalf("GetPackage() error = %v", err)
	}
	if pkg != nil {
		t.Error("GetPackage() should return nil for non-existent package")
	}
}

func TestListPackages(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Insert multiple packages
	packages := []*Package{
		{ImportPath: "github.com/a/pkg", Name: "pkg", Synopsis: "Package A"},
		{ImportPath: "github.com/b/pkg", Name: "pkg", Synopsis: "Package B"},
		{ImportPath: "github.com/c/pkg", Name: "pkg", Synopsis: "Package C"},
	}

	for _, pkg := range packages {
		if _, err := db.UpsertPackage(pkg); err != nil {
			t.Fatalf("UpsertPackage() error = %v", err)
		}
	}

	// List all packages
	retrieved, err := db.ListPackages()
	if err != nil {
		t.Fatalf("ListPackages() error = %v", err)
	}

	if len(retrieved) != len(packages) {
		t.Errorf("ListPackages() returned %v packages, want %v", len(retrieved), len(packages))
	}

	// Verify order (should be sorted by import path)
	if len(retrieved) >= 2 && retrieved[0].ImportPath > retrieved[1].ImportPath {
		t.Error("ListPackages() results not sorted by import path")
	}
}

func TestSearchPackages(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Insert test packages
	packages := []*Package{
		{ImportPath: "github.com/http/client", Name: "client", Synopsis: "HTTP client library"},
		{ImportPath: "github.com/http/server", Name: "server", Synopsis: "HTTP server framework"},
		{ImportPath: "github.com/grpc/core", Name: "core", Synopsis: "gRPC core library"},
	}

	for _, pkg := range packages {
		if _, err := db.UpsertPackage(pkg); err != nil {
			t.Fatalf("UpsertPackage() error = %v", err)
		}
	}

	tests := []struct {
		name        string
		query       string
		wantMinimum int
	}{
		{"search by term", "http", 2},
		{"search by synopsis", "client", 1},
		{"search by import path", "grpc", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := db.SearchPackages(tt.query, 100)
			if err != nil {
				t.Fatalf("SearchPackages() error = %v", err)
			}
			if len(results) < tt.wantMinimum {
				t.Errorf("SearchPackages() returned %v results, want at least %v", len(results), tt.wantMinimum)
			}
		})
	}
}

func TestAddImport(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Insert packages first
	importer := &Package{ImportPath: "github.com/test/app", Name: "app", ModulePath: "github.com/test/app"}
	imported := &Package{ImportPath: "github.com/test/lib", Name: "lib", ModulePath: "github.com/test/lib"}

	if _, err := db.UpsertPackage(importer); err != nil {
		t.Fatalf("UpsertPackage(importer) error = %v", err)
	}
	if _, err := db.UpsertPackage(imported); err != nil {
		t.Fatalf("UpsertPackage(imported) error = %v", err)
	}

	// Add import
	err := db.AddImport("github.com/test/app", "github.com/test/lib", "github.com/test/app")
	if err != nil {
		t.Fatalf("AddImport() error = %v", err)
	}

	// Add same import again (should not error due to IGNORE)
	err = db.AddImport("github.com/test/app", "github.com/test/lib", "github.com/test/app")
	if err != nil {
		t.Fatalf("AddImport() duplicate error = %v", err)
	}
}

func TestGetImportedBy(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create a library package
	lib := &Package{ImportPath: "github.com/test/lib", Name: "lib", ModulePath: "github.com/test/lib"}
	if _, err := db.UpsertPackage(lib); err != nil {
		t.Fatalf("UpsertPackage(lib) error = %v", err)
	}

	// Create multiple importers
	for i := 0; i < 5; i++ {
		importPath := filepath.Join("github.com/test", "app"+string(rune('a'+i)))
		importer := &Package{ImportPath: importPath, Name: "app", ModulePath: importPath}
		if _, err := db.UpsertPackage(importer); err != nil {
			t.Fatalf("UpsertPackage(importer) error = %v", err)
		}
		if err := db.AddImport(importPath, "github.com/test/lib", importPath); err != nil {
			t.Fatalf("AddImport() error = %v", err)
		}
	}

	// Test GetImportedBy
	packages, total, err := db.GetImportedBy("github.com/test/lib", 10, 0)
	if err != nil {
		t.Fatalf("GetImportedBy() error = %v", err)
	}
	if total != 5 {
		t.Errorf("GetImportedBy() total = %v, want 5", total)
	}
	if len(packages) != 5 {
		t.Errorf("GetImportedBy() returned %v packages, want 5", len(packages))
	}

	// Test pagination
	packages, total, err = db.GetImportedBy("github.com/test/lib", 2, 0)
	if err != nil {
		t.Fatalf("GetImportedBy() pagination error = %v", err)
	}
	if len(packages) != 2 {
		t.Errorf("GetImportedBy() with limit=2 returned %v packages, want 2", len(packages))
	}
	if total != 5 {
		t.Errorf("GetImportedBy() total with pagination = %v, want 5", total)
	}
}

func TestGetImportedByCount(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create library and importers
	lib := &Package{ImportPath: "github.com/test/lib", Name: "lib", ModulePath: "github.com/test/lib"}
	if _, err := db.UpsertPackage(lib); err != nil {
		t.Fatalf("UpsertPackage() error = %v", err)
	}

	for i := 0; i < 3; i++ {
		importPath := filepath.Join("github.com/test", "app"+string(rune('a'+i)))
		importer := &Package{ImportPath: importPath, Name: "app", ModulePath: importPath}
		if _, err := db.UpsertPackage(importer); err != nil {
			t.Fatalf("UpsertPackage() error = %v", err)
		}
		if err := db.AddImport(importPath, "github.com/test/lib", importPath); err != nil {
			t.Fatalf("AddImport() error = %v", err)
		}
	}

	count, err := db.GetImportedByCount("github.com/test/lib")
	if err != nil {
		t.Fatalf("GetImportedByCount() error = %v", err)
	}
	if count != 3 {
		t.Errorf("GetImportedByCount() = %v, want 3", count)
	}
}

func TestUpsertSymbol(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Insert package first
	pkg := &Package{ImportPath: "github.com/test/pkg", Name: "pkg"}
	pkgID, err := db.UpsertPackage(pkg)
	if err != nil {
		t.Fatalf("UpsertPackage() error = %v", err)
	}

	symbol := &Symbol{
		Name:       "TestFunc",
		Kind:       "func",
		PackageID:  pkgID,
		ImportPath: "github.com/test/pkg",
		Synopsis:   "TestFunc does testing",
		Deprecated: false,
	}

	err = db.UpsertSymbol(symbol)
	if err != nil {
		t.Fatalf("UpsertSymbol() error = %v", err)
	}

	// Update the symbol
	symbol.Synopsis = "Updated synopsis"
	err = db.UpsertSymbol(symbol)
	if err != nil {
		t.Fatalf("UpsertSymbol() update error = %v", err)
	}
}

func TestSearchSymbols(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Insert package and symbols
	pkg := &Package{ImportPath: "github.com/test/pkg", Name: "pkg"}
	pkgID, err := db.UpsertPackage(pkg)
	if err != nil {
		t.Fatalf("UpsertPackage() error = %v", err)
	}

	symbols := []*Symbol{
		{Name: "HTTPClient", Kind: "type", PackageID: pkgID, ImportPath: "github.com/test/pkg", Synopsis: "HTTP client type"},
		{Name: "NewClient", Kind: "func", PackageID: pkgID, ImportPath: "github.com/test/pkg", Synopsis: "Creates new HTTP client"},
		{Name: "ServerConfig", Kind: "type", PackageID: pkgID, ImportPath: "github.com/test/pkg", Synopsis: "Server configuration"},
	}

	for _, sym := range symbols {
		if err := db.UpsertSymbol(sym); err != nil {
			t.Fatalf("UpsertSymbol() error = %v", err)
		}
	}

	// Test search without kind filter
	results, err := db.SearchSymbols("http", "", 100)
	if err != nil {
		t.Fatalf("SearchSymbols() error = %v", err)
	}
	if len(results) < 1 {
		t.Errorf("SearchSymbols() returned %v results, want at least 1", len(results))
	}

	// Test search with kind filter
	results, err = db.SearchSymbols("client", "func", 100)
	if err != nil {
		t.Fatalf("SearchSymbols() with kind error = %v", err)
	}
	if len(results) < 1 {
		t.Errorf("SearchSymbols() with kind returned %v results, want at least 1", len(results))
	}
	for _, sym := range results {
		if sym.Kind != "func" {
			t.Errorf("SearchSymbols() with kind='func' returned symbol with kind=%v", sym.Kind)
		}
	}
}

func TestDeletePackageSymbols(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Insert package and symbols
	pkg := &Package{ImportPath: "github.com/test/pkg", Name: "pkg"}
	pkgID, err := db.UpsertPackage(pkg)
	if err != nil {
		t.Fatalf("UpsertPackage() error = %v", err)
	}

	symbol := &Symbol{
		Name:       "TestFunc",
		Kind:       "func",
		PackageID:  pkgID,
		ImportPath: "github.com/test/pkg",
	}
	if err := db.UpsertSymbol(symbol); err != nil {
		t.Fatalf("UpsertSymbol() error = %v", err)
	}

	// Delete symbols
	err = db.DeletePackageSymbols(pkgID)
	if err != nil {
		t.Fatalf("DeletePackageSymbols() error = %v", err)
	}

	// Verify deletion
	var count int
	err = db.conn.QueryRow("SELECT COUNT(*) FROM symbols WHERE package_id = ?", pkgID).Scan(&count)
	if err != nil {
		t.Fatalf("failed to count symbols: %v", err)
	}
	if count != 0 {
		t.Errorf("DeletePackageSymbols() left %v symbols, want 0", count)
	}
}

func TestDeletePackage(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Insert package with symbols and imports
	pkg := &Package{ImportPath: "github.com/test/pkg", Name: "pkg", ModulePath: "github.com/test/pkg"}
	pkgID, err := db.UpsertPackage(pkg)
	if err != nil {
		t.Fatalf("UpsertPackage() error = %v", err)
	}

	symbol := &Symbol{Name: "TestFunc", Kind: "func", PackageID: pkgID, ImportPath: "github.com/test/pkg"}
	if err := db.UpsertSymbol(symbol); err != nil {
		t.Fatalf("UpsertSymbol() error = %v", err)
	}

	if err := db.AddImport("github.com/test/pkg", "fmt", "github.com/test/pkg"); err != nil {
		t.Fatalf("AddImport() error = %v", err)
	}

	// Delete package
	err = db.DeletePackage("github.com/test/pkg")
	if err != nil {
		t.Fatalf("DeletePackage() error = %v", err)
	}

	// Verify package is deleted
	retrieved, err := db.GetPackage("github.com/test/pkg")
	if err != nil {
		t.Fatalf("GetPackage() error = %v", err)
	}
	if retrieved != nil {
		t.Error("DeletePackage() did not delete package")
	}

	// Verify symbols are deleted (CASCADE)
	var symbolCount int
	db.conn.QueryRow("SELECT COUNT(*) FROM symbols WHERE package_id = ?", pkgID).Scan(&symbolCount)
	if symbolCount != 0 {
		t.Errorf("DeletePackage() left %v symbols, want 0", symbolCount)
	}

	// Verify imports are deleted
	var importCount int
	db.conn.QueryRow("SELECT COUNT(*) FROM imports WHERE importer_path = ?", "github.com/test/pkg").Scan(&importCount)
	if importCount != 0 {
		t.Errorf("DeletePackage() left %v imports, want 0", importCount)
	}
}

func TestGetStats(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Insert test data
	pkg := &Package{ImportPath: "github.com/test/pkg", Name: "pkg", ModulePath: "github.com/test/pkg"}
	pkgID, err := db.UpsertPackage(pkg)
	if err != nil {
		t.Fatalf("UpsertPackage() error = %v", err)
	}

	symbol := &Symbol{Name: "TestFunc", Kind: "func", PackageID: pkgID, ImportPath: "github.com/test/pkg"}
	if err := db.UpsertSymbol(symbol); err != nil {
		t.Fatalf("UpsertSymbol() error = %v", err)
	}

	if err := db.AddImport("github.com/test/pkg", "fmt", "github.com/test/pkg"); err != nil {
		t.Fatalf("AddImport() error = %v", err)
	}

	// Get stats
	packageCount, symbolCount, importCount, err := db.GetStats()
	if err != nil {
		t.Fatalf("GetStats() error = %v", err)
	}

	if packageCount != 1 {
		t.Errorf("GetStats() packageCount = %v, want 1", packageCount)
	}
	if symbolCount != 1 {
		t.Errorf("GetStats() symbolCount = %v, want 1", symbolCount)
	}
	if importCount != 1 {
		t.Errorf("GetStats() importCount = %v, want 1", importCount)
	}
}

func TestCrawlMetadata(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Test SetLastCrawlTime and GetLastCrawlTime
	now := time.Now().Truncate(time.Second)
	err := db.SetLastCrawlTime(now)
	if err != nil {
		t.Fatalf("SetLastCrawlTime() error = %v", err)
	}

	retrieved, err := db.GetLastCrawlTime()
	if err != nil {
		t.Fatalf("GetLastCrawlTime() error = %v", err)
	}

	if !retrieved.Equal(now) {
		t.Errorf("GetLastCrawlTime() = %v, want %v", retrieved, now)
	}

	// Test generic metadata
	err = db.SetMetadata("test_key", "test_value")
	if err != nil {
		t.Fatalf("SetMetadata() error = %v", err)
	}

	value, err := db.GetMetadata("test_key")
	if err != nil {
		t.Fatalf("GetMetadata() error = %v", err)
	}
	if value != "test_value" {
		t.Errorf("GetMetadata() = %v, want %v", value, "test_value")
	}

	// Test non-existent metadata
	value, err = db.GetMetadata("nonexistent")
	if err != nil {
		t.Fatalf("GetMetadata(nonexistent) error = %v", err)
	}
	if value != "" {
		t.Errorf("GetMetadata(nonexistent) = %v, want empty string", value)
	}
}

func TestModuleVersions(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	modulePath := "github.com/test/module"

	// Insert versions
	versions := []*ModuleVersion{
		{ModulePath: modulePath, Version: "v1.0.0", IsTagged: true, IsStable: true},
		{ModulePath: modulePath, Version: "v1.1.0", IsTagged: true, IsStable: true},
		{ModulePath: modulePath, Version: "v0.9.0", IsTagged: true, IsStable: false},
	}

	for _, mv := range versions {
		if err := db.UpsertModuleVersion(mv); err != nil {
			t.Fatalf("UpsertModuleVersion() error = %v", err)
		}
	}

	// Test GetModuleVersions
	retrieved, err := db.GetModuleVersions(modulePath)
	if err != nil {
		t.Fatalf("GetModuleVersions() error = %v", err)
	}
	if len(retrieved) != 3 {
		t.Errorf("GetModuleVersions() returned %v versions, want 3", len(retrieved))
	}

	// Test GetModuleVersion
	mv, err := db.GetModuleVersion(modulePath, "v1.0.0")
	if err != nil {
		t.Fatalf("GetModuleVersion() error = %v", err)
	}
	if mv == nil {
		t.Fatal("GetModuleVersion() returned nil")
	}
	if mv.Version != "v1.0.0" {
		t.Errorf("GetModuleVersion() version = %v, want v1.0.0", mv.Version)
	}

	// Test GetLatestModuleVersion
	latest, err := db.GetLatestModuleVersion(modulePath)
	if err != nil {
		t.Fatalf("GetLatestModuleVersion() error = %v", err)
	}
	if latest == nil {
		t.Fatal("GetLatestModuleVersion() returned nil")
	}
	// Should prefer stable versions
	if !latest.IsStable {
		t.Errorf("GetLatestModuleVersion() returned unstable version %v", latest.Version)
	}

	// Test CountModuleVersions
	count, err := db.CountModuleVersions(modulePath)
	if err != nil {
		t.Fatalf("CountModuleVersions() error = %v", err)
	}
	if count != 3 {
		t.Errorf("CountModuleVersions() = %v, want 3", count)
	}
}

func TestUpsertModuleVersion_Update(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	mv := &ModuleVersion{
		ModulePath: "github.com/test/module",
		Version:    "v1.0.0",
		IsTagged:   true,
		IsStable:   false,
	}

	if err := db.UpsertModuleVersion(mv); err != nil {
		t.Fatalf("UpsertModuleVersion() insert error = %v", err)
	}

	// Update
	mv.IsStable = true
	if err := db.UpsertModuleVersion(mv); err != nil {
		t.Fatalf("UpsertModuleVersion() update error = %v", err)
	}

	// Verify
	retrieved, err := db.GetModuleVersion(mv.ModulePath, mv.Version)
	if err != nil {
		t.Fatalf("GetModuleVersion() error = %v", err)
	}
	if !retrieved.IsStable {
		t.Error("UpsertModuleVersion() did not update IsStable")
	}
}
