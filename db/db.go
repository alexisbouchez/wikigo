package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// DB wraps the SQLite database connection
type DB struct {
	conn *sql.DB
}

// Package represents a Go package in the database
type Package struct {
	ID              int64     `json:"id"`
	ImportPath      string    `json:"import_path"`
	Name            string    `json:"name"`
	Synopsis        string    `json:"synopsis"`
	Doc             string    `json:"doc"`
	Version         string    `json:"version"`
	Versions        []string  `json:"versions"`
	IsTagged        bool      `json:"is_tagged"`
	IsStable        bool      `json:"is_stable"`
	License         string    `json:"license"`
	LicenseText     string    `json:"license_text"`
	Redistributable bool      `json:"redistributable"`
	Repository      string    `json:"repository"`
	HasValidMod     bool      `json:"has_valid_mod"`
	GoVersion       string    `json:"go_version"`
	ModulePath      string    `json:"module_path"`
	GoModContent    string    `json:"gomod_content"`
	GOOS            []string  `json:"goos"`
	GOARCH          []string  `json:"goarch"`
	DocJSON         string    `json:"doc_json"` // Full package documentation as JSON
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	IndexedAt       time.Time `json:"indexed_at"`
}

// Import represents an import relationship between packages
type Import struct {
	ID             int64  `json:"id"`
	ImporterPath   string `json:"importer_path"`   // Package that imports
	ImportedPath   string `json:"imported_path"`   // Package being imported
	ImporterModule string `json:"importer_module"` // Module of the importer
}

// Symbol represents a searchable symbol (function, type, method, etc.)
type Symbol struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Kind       string `json:"kind"` // func, type, method, const, var
	PackageID  int64  `json:"package_id"`
	ImportPath string `json:"import_path"`
	Synopsis   string `json:"synopsis"`
	Deprecated bool   `json:"deprecated"`
}

// Open opens or creates a SQLite database
func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Enable foreign keys
	if _, err := conn.Exec("PRAGMA foreign_keys = ON"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("enabling foreign keys: %w", err)
	}

	db := &DB{conn: conn}

	// Run migrations
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return db, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.conn.Close()
}

// migrate runs database migrations
func (db *DB) migrate() error {
	migrations := []string{
		// Packages table
		`CREATE TABLE IF NOT EXISTS packages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			import_path TEXT UNIQUE NOT NULL,
			name TEXT NOT NULL,
			synopsis TEXT,
			doc TEXT,
			version TEXT,
			versions_json TEXT,
			is_tagged INTEGER DEFAULT 0,
			is_stable INTEGER DEFAULT 0,
			license TEXT,
			license_text TEXT,
			redistributable INTEGER DEFAULT 0,
			repository TEXT,
			has_valid_mod INTEGER DEFAULT 0,
			go_version TEXT,
			module_path TEXT,
			gomod_content TEXT,
			goos_json TEXT,
			goarch_json TEXT,
			doc_json TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			indexed_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Imports table for tracking import relationships
		`CREATE TABLE IF NOT EXISTS imports (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			importer_path TEXT NOT NULL,
			imported_path TEXT NOT NULL,
			importer_module TEXT,
			UNIQUE(importer_path, imported_path)
		)`,

		// Symbols table for symbol search
		`CREATE TABLE IF NOT EXISTS symbols (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			kind TEXT NOT NULL,
			package_id INTEGER NOT NULL,
			import_path TEXT NOT NULL,
			synopsis TEXT,
			deprecated INTEGER DEFAULT 0,
			FOREIGN KEY (package_id) REFERENCES packages(id) ON DELETE CASCADE
		)`,

		// Indexes for fast lookups
		`CREATE INDEX IF NOT EXISTS idx_packages_import_path ON packages(import_path)`,
		`CREATE INDEX IF NOT EXISTS idx_packages_module_path ON packages(module_path)`,
		`CREATE INDEX IF NOT EXISTS idx_packages_name ON packages(name)`,
		`CREATE INDEX IF NOT EXISTS idx_imports_importer ON imports(importer_path)`,
		`CREATE INDEX IF NOT EXISTS idx_imports_imported ON imports(imported_path)`,
		`CREATE INDEX IF NOT EXISTS idx_symbols_name ON symbols(name)`,
		`CREATE INDEX IF NOT EXISTS idx_symbols_kind ON symbols(kind)`,
		`CREATE INDEX IF NOT EXISTS idx_symbols_package ON symbols(package_id)`,

		// Full-text search for packages using FTS4 (more widely supported)
		`CREATE VIRTUAL TABLE IF NOT EXISTS packages_fts USING fts4(
			import_path,
			name,
			synopsis,
			doc,
			content="packages",
			tokenize=porter
		)`,

		// Full-text search for symbols using FTS4
		`CREATE VIRTUAL TABLE IF NOT EXISTS symbols_fts USING fts4(
			name,
			synopsis,
			content="symbols",
			tokenize=porter
		)`,

		// Triggers to keep FTS in sync with packages
		`CREATE TRIGGER IF NOT EXISTS packages_ai AFTER INSERT ON packages BEGIN
			INSERT INTO packages_fts(docid, import_path, name, synopsis, doc)
			VALUES (new.id, new.import_path, new.name, new.synopsis, new.doc);
		END`,

		`CREATE TRIGGER IF NOT EXISTS packages_ad AFTER DELETE ON packages BEGIN
			DELETE FROM packages_fts WHERE docid = old.id;
		END`,

		`CREATE TRIGGER IF NOT EXISTS packages_au AFTER UPDATE ON packages BEGIN
			DELETE FROM packages_fts WHERE docid = old.id;
			INSERT INTO packages_fts(docid, import_path, name, synopsis, doc)
			VALUES (new.id, new.import_path, new.name, new.synopsis, new.doc);
		END`,

		// Triggers to keep FTS in sync with symbols
		`CREATE TRIGGER IF NOT EXISTS symbols_ai AFTER INSERT ON symbols BEGIN
			INSERT INTO symbols_fts(docid, name, synopsis)
			VALUES (new.id, new.name, new.synopsis);
		END`,

		`CREATE TRIGGER IF NOT EXISTS symbols_ad AFTER DELETE ON symbols BEGIN
			DELETE FROM symbols_fts WHERE docid = old.id;
		END`,

		`CREATE TRIGGER IF NOT EXISTS symbols_au AFTER UPDATE ON symbols BEGIN
			DELETE FROM symbols_fts WHERE docid = old.id;
			INSERT INTO symbols_fts(docid, name, synopsis)
			VALUES (new.id, new.name, new.synopsis);
		END`,

		// Metadata table for crawl state tracking
		`CREATE TABLE IF NOT EXISTS crawl_metadata (
			key TEXT PRIMARY KEY,
			value TEXT,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	for _, migration := range migrations {
		if _, err := db.conn.Exec(migration); err != nil {
			return fmt.Errorf("executing migration: %w", err)
		}
	}

	return nil
}

// UpsertPackage inserts or updates a package
func (db *DB) UpsertPackage(pkg *Package) (int64, error) {
	versionsJSON, _ := json.Marshal(pkg.Versions)
	goosJSON, _ := json.Marshal(pkg.GOOS)
	goarchJSON, _ := json.Marshal(pkg.GOARCH)

	result, err := db.conn.Exec(`
		INSERT INTO packages (
			import_path, name, synopsis, doc, version, versions_json,
			is_tagged, is_stable, license, license_text, redistributable,
			repository, has_valid_mod, go_version, module_path, gomod_content,
			goos_json, goarch_json, doc_json, updated_at, indexed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(import_path) DO UPDATE SET
			name = excluded.name,
			synopsis = excluded.synopsis,
			doc = excluded.doc,
			version = excluded.version,
			versions_json = excluded.versions_json,
			is_tagged = excluded.is_tagged,
			is_stable = excluded.is_stable,
			license = excluded.license,
			license_text = excluded.license_text,
			redistributable = excluded.redistributable,
			repository = excluded.repository,
			has_valid_mod = excluded.has_valid_mod,
			go_version = excluded.go_version,
			module_path = excluded.module_path,
			gomod_content = excluded.gomod_content,
			goos_json = excluded.goos_json,
			goarch_json = excluded.goarch_json,
			doc_json = excluded.doc_json,
			updated_at = CURRENT_TIMESTAMP,
			indexed_at = CURRENT_TIMESTAMP
	`, pkg.ImportPath, pkg.Name, pkg.Synopsis, pkg.Doc, pkg.Version, string(versionsJSON),
		pkg.IsTagged, pkg.IsStable, pkg.License, pkg.LicenseText, pkg.Redistributable,
		pkg.Repository, pkg.HasValidMod, pkg.GoVersion, pkg.ModulePath, pkg.GoModContent,
		string(goosJSON), string(goarchJSON), pkg.DocJSON)

	if err != nil {
		return 0, fmt.Errorf("upserting package: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		// If upsert did an update, get the existing ID
		row := db.conn.QueryRow("SELECT id FROM packages WHERE import_path = ?", pkg.ImportPath)
		if err := row.Scan(&id); err != nil {
			return 0, fmt.Errorf("getting package id: %w", err)
		}
	}

	return id, nil
}

// GetPackage retrieves a package by import path
func (db *DB) GetPackage(importPath string) (*Package, error) {
	row := db.conn.QueryRow(`
		SELECT id, import_path, name, synopsis, doc, version, versions_json,
			is_tagged, is_stable, license, license_text, redistributable,
			repository, has_valid_mod, go_version, module_path, gomod_content,
			goos_json, goarch_json, doc_json, created_at, updated_at, indexed_at
		FROM packages WHERE import_path = ?
	`, importPath)

	pkg := &Package{}
	var versionsJSON, goosJSON, goarchJSON sql.NullString
	var docJSON sql.NullString

	err := row.Scan(
		&pkg.ID, &pkg.ImportPath, &pkg.Name, &pkg.Synopsis, &pkg.Doc,
		&pkg.Version, &versionsJSON, &pkg.IsTagged, &pkg.IsStable,
		&pkg.License, &pkg.LicenseText, &pkg.Redistributable,
		&pkg.Repository, &pkg.HasValidMod, &pkg.GoVersion, &pkg.ModulePath,
		&pkg.GoModContent, &goosJSON, &goarchJSON, &docJSON,
		&pkg.CreatedAt, &pkg.UpdatedAt, &pkg.IndexedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scanning package: %w", err)
	}

	// Parse JSON fields
	if versionsJSON.Valid {
		json.Unmarshal([]byte(versionsJSON.String), &pkg.Versions)
	}
	if goosJSON.Valid {
		json.Unmarshal([]byte(goosJSON.String), &pkg.GOOS)
	}
	if goarchJSON.Valid {
		json.Unmarshal([]byte(goarchJSON.String), &pkg.GOARCH)
	}
	if docJSON.Valid {
		pkg.DocJSON = docJSON.String
	}

	return pkg, nil
}

// ListPackages returns all packages
func (db *DB) ListPackages() ([]*Package, error) {
	rows, err := db.conn.Query(`
		SELECT id, import_path, name, synopsis, version, is_tagged, is_stable,
			license, redistributable, repository, module_path
		FROM packages ORDER BY import_path
	`)
	if err != nil {
		return nil, fmt.Errorf("querying packages: %w", err)
	}
	defer rows.Close()

	var packages []*Package
	for rows.Next() {
		pkg := &Package{}
		err := rows.Scan(
			&pkg.ID, &pkg.ImportPath, &pkg.Name, &pkg.Synopsis,
			&pkg.Version, &pkg.IsTagged, &pkg.IsStable,
			&pkg.License, &pkg.Redistributable, &pkg.Repository, &pkg.ModulePath,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning package row: %w", err)
		}
		packages = append(packages, pkg)
	}

	return packages, rows.Err()
}

// SearchPackages searches packages using full-text search
func (db *DB) SearchPackages(query string, limit int) ([]*Package, error) {
	if limit <= 0 {
		limit = 50
	}

	rows, err := db.conn.Query(`
		SELECT p.id, p.import_path, p.name, p.synopsis, p.version,
			p.is_tagged, p.is_stable, p.license, p.redistributable,
			p.repository, p.module_path
		FROM packages p
		JOIN packages_fts fts ON p.id = fts.docid
		WHERE packages_fts MATCH ?
		LIMIT ?
	`, query, limit)
	if err != nil {
		return nil, fmt.Errorf("searching packages: %w", err)
	}
	defer rows.Close()

	var packages []*Package
	for rows.Next() {
		pkg := &Package{}
		err := rows.Scan(
			&pkg.ID, &pkg.ImportPath, &pkg.Name, &pkg.Synopsis,
			&pkg.Version, &pkg.IsTagged, &pkg.IsStable,
			&pkg.License, &pkg.Redistributable, &pkg.Repository, &pkg.ModulePath,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning search result: %w", err)
		}
		packages = append(packages, pkg)
	}

	return packages, rows.Err()
}

// AddImport records an import relationship
func (db *DB) AddImport(importerPath, importedPath, importerModule string) error {
	_, err := db.conn.Exec(`
		INSERT OR IGNORE INTO imports (importer_path, imported_path, importer_module)
		VALUES (?, ?, ?)
	`, importerPath, importedPath, importerModule)
	return err
}

// GetImportedBy returns packages that import the given package
func (db *DB) GetImportedBy(importPath string, limit, offset int) ([]*Package, int, error) {
	if limit <= 0 {
		limit = 50
	}

	// Get total count
	var total int
	err := db.conn.QueryRow(`
		SELECT COUNT(DISTINCT importer_path) FROM imports WHERE imported_path = ?
	`, importPath).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("counting importers: %w", err)
	}

	// Get packages
	rows, err := db.conn.Query(`
		SELECT DISTINCT p.id, p.import_path, p.name, p.synopsis, p.version,
			p.is_tagged, p.is_stable, p.license, p.redistributable,
			p.repository, p.module_path
		FROM imports i
		JOIN packages p ON i.importer_path = p.import_path
		WHERE i.imported_path = ?
		ORDER BY p.import_path
		LIMIT ? OFFSET ?
	`, importPath, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("querying importers: %w", err)
	}
	defer rows.Close()

	var packages []*Package
	for rows.Next() {
		pkg := &Package{}
		err := rows.Scan(
			&pkg.ID, &pkg.ImportPath, &pkg.Name, &pkg.Synopsis,
			&pkg.Version, &pkg.IsTagged, &pkg.IsStable,
			&pkg.License, &pkg.Redistributable, &pkg.Repository, &pkg.ModulePath,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("scanning importer: %w", err)
		}
		packages = append(packages, pkg)
	}

	return packages, total, rows.Err()
}

// GetImportedByCount returns the count of packages that import the given package
func (db *DB) GetImportedByCount(importPath string) (int, error) {
	var count int
	err := db.conn.QueryRow(`
		SELECT COUNT(DISTINCT importer_path) FROM imports WHERE imported_path = ?
	`, importPath).Scan(&count)
	return count, err
}

// UpsertSymbol inserts or updates a symbol
func (db *DB) UpsertSymbol(symbol *Symbol) error {
	_, err := db.conn.Exec(`
		INSERT INTO symbols (name, kind, package_id, import_path, synopsis, deprecated)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT DO UPDATE SET
			synopsis = excluded.synopsis,
			deprecated = excluded.deprecated
	`, symbol.Name, symbol.Kind, symbol.PackageID, symbol.ImportPath, symbol.Synopsis, symbol.Deprecated)
	return err
}

// DeletePackageSymbols deletes all symbols for a package
func (db *DB) DeletePackageSymbols(packageID int64) error {
	_, err := db.conn.Exec("DELETE FROM symbols WHERE package_id = ?", packageID)
	return err
}

// SearchSymbols searches symbols using full-text search
func (db *DB) SearchSymbols(query, kind string, limit int) ([]*Symbol, error) {
	if limit <= 0 {
		limit = 100
	}

	var rows *sql.Rows
	var err error

	if kind != "" {
		rows, err = db.conn.Query(`
			SELECT s.id, s.name, s.kind, s.package_id, s.import_path, s.synopsis, s.deprecated
			FROM symbols s
			JOIN symbols_fts fts ON s.id = fts.docid
			WHERE symbols_fts MATCH ? AND s.kind = ?
			LIMIT ?
		`, query, kind, limit)
	} else {
		rows, err = db.conn.Query(`
			SELECT s.id, s.name, s.kind, s.package_id, s.import_path, s.synopsis, s.deprecated
			FROM symbols s
			JOIN symbols_fts fts ON s.id = fts.docid
			WHERE symbols_fts MATCH ?
			LIMIT ?
		`, query, limit)
	}

	if err != nil {
		return nil, fmt.Errorf("searching symbols: %w", err)
	}
	defer rows.Close()

	var symbols []*Symbol
	for rows.Next() {
		sym := &Symbol{}
		err := rows.Scan(&sym.ID, &sym.Name, &sym.Kind, &sym.PackageID,
			&sym.ImportPath, &sym.Synopsis, &sym.Deprecated)
		if err != nil {
			return nil, fmt.Errorf("scanning symbol: %w", err)
		}
		symbols = append(symbols, sym)
	}

	return symbols, rows.Err()
}

// GetStats returns database statistics
func (db *DB) GetStats() (packageCount, symbolCount, importCount int, err error) {
	err = db.conn.QueryRow("SELECT COUNT(*) FROM packages").Scan(&packageCount)
	if err != nil {
		return
	}
	err = db.conn.QueryRow("SELECT COUNT(*) FROM symbols").Scan(&symbolCount)
	if err != nil {
		return
	}
	err = db.conn.QueryRow("SELECT COUNT(*) FROM imports").Scan(&importCount)
	return
}

// DeletePackage deletes a package and its related data
func (db *DB) DeletePackage(importPath string) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Get package ID first
	var packageID int64
	err = tx.QueryRow("SELECT id FROM packages WHERE import_path = ?", importPath).Scan(&packageID)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}

	// Delete symbols
	if _, err := tx.Exec("DELETE FROM symbols WHERE package_id = ?", packageID); err != nil {
		return err
	}

	// Delete imports
	if _, err := tx.Exec("DELETE FROM imports WHERE importer_path = ?", importPath); err != nil {
		return err
	}

	// Delete package
	if _, err := tx.Exec("DELETE FROM packages WHERE id = ?", packageID); err != nil {
		return err
	}

	return tx.Commit()
}

// GetLastCrawlTime returns the last successful crawl time
func (db *DB) GetLastCrawlTime() (time.Time, error) {
	var value sql.NullString
	err := db.conn.QueryRow(`
		SELECT value FROM crawl_metadata WHERE key = 'last_crawl_time'
	`).Scan(&value)

	if err == sql.ErrNoRows || !value.Valid {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, err
	}

	return time.Parse(time.RFC3339, value.String)
}

// SetLastCrawlTime sets the last successful crawl time
func (db *DB) SetLastCrawlTime(t time.Time) error {
	_, err := db.conn.Exec(`
		INSERT INTO crawl_metadata (key, value, updated_at)
		VALUES ('last_crawl_time', ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET
			value = excluded.value,
			updated_at = CURRENT_TIMESTAMP
	`, t.Format(time.RFC3339))
	return err
}

// GetMetadata retrieves a metadata value by key
func (db *DB) GetMetadata(key string) (string, error) {
	var value sql.NullString
	err := db.conn.QueryRow(`
		SELECT value FROM crawl_metadata WHERE key = ?
	`, key).Scan(&value)

	if err == sql.ErrNoRows || !value.Valid {
		return "", nil
	}
	if err != nil {
		return "", err
	}

	return value.String, nil
}

// SetMetadata sets a metadata value
func (db *DB) SetMetadata(key, value string) error {
	_, err := db.conn.Exec(`
		INSERT INTO crawl_metadata (key, value, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET
			value = excluded.value,
			updated_at = CURRENT_TIMESTAMP
	`, key, value)
	return err
}
