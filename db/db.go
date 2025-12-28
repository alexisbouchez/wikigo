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
	Doc        string `json:"doc"`       // Full documentation
	Signature  string `json:"signature"` // Function signature
	Decl       string `json:"decl"`      // Type/const/var declaration
	Deprecated bool   `json:"deprecated"`
}

// ModuleVersion represents a version of a module
type ModuleVersion struct {
	ID         int64     `json:"id"`
	ModulePath string    `json:"module_path"`
	Version    string    `json:"version"`
	Timestamp  time.Time `json:"timestamp"`  // When this version was published
	IsTagged   bool      `json:"is_tagged"`  // Semver tagged version
	IsStable   bool      `json:"is_stable"`  // v1+ and no pre-release
	Retracted  bool      `json:"retracted"`  // Version was retracted
	CreatedAt  time.Time `json:"created_at"` // When we indexed it
}

// AIDoc represents AI-generated documentation for a symbol
type AIDoc struct {
	ID           int64     `json:"id"`
	SymbolName   string    `json:"symbol_name"`
	SymbolKind   string    `json:"symbol_kind"` // "func", "type", "method"
	ImportPath   string    `json:"import_path"`
	GeneratedDoc string    `json:"generated_doc"`
	Approved     bool      `json:"approved"`
	Flagged      bool      `json:"flagged"`
	FlagReason   string    `json:"flag_reason,omitempty"`
	CostUSD      float64   `json:"cost_usd"`
	Tokens       int       `json:"tokens"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
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
			doc TEXT,
			signature TEXT,
			decl TEXT,
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

		// Module versions table for version history tracking
		`CREATE TABLE IF NOT EXISTS module_versions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			module_path TEXT NOT NULL,
			version TEXT NOT NULL,
			timestamp DATETIME,
			is_tagged INTEGER DEFAULT 0,
			is_stable INTEGER DEFAULT 0,
			retracted INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(module_path, version)
		)`,

		`CREATE INDEX IF NOT EXISTS idx_module_versions_path ON module_versions(module_path)`,
		`CREATE INDEX IF NOT EXISTS idx_module_versions_timestamp ON module_versions(timestamp DESC)`,

		// AI-generated documentation table
		`CREATE TABLE IF NOT EXISTS ai_docs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			symbol_name TEXT NOT NULL,
			symbol_kind TEXT NOT NULL,
			import_path TEXT NOT NULL,
			generated_doc TEXT NOT NULL,
			approved INTEGER DEFAULT 0,
			flagged INTEGER DEFAULT 0,
			flag_reason TEXT,
			cost_usd REAL DEFAULT 0,
			tokens INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(import_path, symbol_name, symbol_kind)
		)`,

		`CREATE INDEX IF NOT EXISTS idx_ai_docs_import_path ON ai_docs(import_path)`,
		`CREATE INDEX IF NOT EXISTS idx_ai_docs_approved ON ai_docs(approved)`,
		`CREATE INDEX IF NOT EXISTS idx_ai_docs_flagged ON ai_docs(flagged)`,

		// JavaScript/TypeScript packages table
		`CREATE TABLE IF NOT EXISTS js_packages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT UNIQUE NOT NULL,
			version TEXT,
			description TEXT,
			author TEXT,
			license TEXT,
			repository_url TEXT,
			homepage TEXT,
			npm_url TEXT,
			github_url TEXT,
			main_file TEXT,
			types_file TEXT,
			has_typescript INTEGER DEFAULT 0,
			stars INTEGER DEFAULT 0,
			forks INTEGER DEFAULT 0,
			keywords_json TEXT,
			dependencies_json TEXT,
			package_json TEXT,
			readme TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			indexed_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// JavaScript/TypeScript symbols table
		`CREATE TABLE IF NOT EXISTS js_symbols (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			kind TEXT NOT NULL,
			signature TEXT,
			package_id INTEGER NOT NULL,
			package_name TEXT NOT NULL,
			file_path TEXT,
			line INTEGER DEFAULT 0,
			exported INTEGER DEFAULT 0,
			doc TEXT,
			deprecated INTEGER DEFAULT 0,
			FOREIGN KEY (package_id) REFERENCES js_packages(id) ON DELETE CASCADE
		)`,

		// Indexes for JS packages and symbols
		`CREATE INDEX IF NOT EXISTS idx_js_packages_name ON js_packages(name)`,
		`CREATE INDEX IF NOT EXISTS idx_js_packages_author ON js_packages(author)`,
		`CREATE INDEX IF NOT EXISTS idx_js_packages_stars ON js_packages(stars DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_js_symbols_name ON js_symbols(name)`,
		`CREATE INDEX IF NOT EXISTS idx_js_symbols_kind ON js_symbols(kind)`,
		`CREATE INDEX IF NOT EXISTS idx_js_symbols_package ON js_symbols(package_id)`,
		`CREATE INDEX IF NOT EXISTS idx_js_symbols_exported ON js_symbols(exported)`,

		// FTS for JavaScript/TypeScript packages
		`CREATE VIRTUAL TABLE IF NOT EXISTS js_packages_fts USING fts4(
			name,
			description,
			author,
			keywords,
			content=js_packages,
			tokenize=porter
		)`,

		// FTS for JavaScript/TypeScript symbols
		`CREATE VIRTUAL TABLE IF NOT EXISTS js_symbols_fts USING fts4(
			name,
			signature,
			doc,
			content=js_symbols,
			tokenize=porter
		)`,

		// Triggers for JS packages FTS
		`CREATE TRIGGER IF NOT EXISTS js_packages_ai AFTER INSERT ON js_packages BEGIN
			INSERT INTO js_packages_fts(docid, name, description, author, keywords)
			VALUES (new.id, new.name, new.description, new.author, new.keywords_json);
		END`,

		`CREATE TRIGGER IF NOT EXISTS js_packages_ad AFTER DELETE ON js_packages BEGIN
			DELETE FROM js_packages_fts WHERE docid = old.id;
		END`,

		`CREATE TRIGGER IF NOT EXISTS js_packages_au AFTER UPDATE ON js_packages BEGIN
			DELETE FROM js_packages_fts WHERE docid = old.id;
			INSERT INTO js_packages_fts(docid, name, description, author, keywords)
			VALUES (new.id, new.name, new.description, new.author, new.keywords_json);
		END`,

		// Triggers for JS symbols FTS
		`CREATE TRIGGER IF NOT EXISTS js_symbols_ai AFTER INSERT ON js_symbols BEGIN
			INSERT INTO js_symbols_fts(docid, name, signature, doc)
			VALUES (new.id, new.name, new.signature, new.doc);
		END`,

		`CREATE TRIGGER IF NOT EXISTS js_symbols_ad AFTER DELETE ON js_symbols BEGIN
			DELETE FROM js_symbols_fts WHERE docid = old.id;
		END`,

		`CREATE TRIGGER IF NOT EXISTS js_symbols_au AFTER UPDATE ON js_symbols BEGIN
			DELETE FROM js_symbols_fts WHERE docid = old.id;
			INSERT INTO js_symbols_fts(docid, name, signature, doc)
			VALUES (new.id, new.name, new.signature, new.doc);
		END`,

		// Rust crates table
		`CREATE TABLE IF NOT EXISTS rust_crates (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT UNIQUE NOT NULL,
			version TEXT,
			description TEXT,
			license TEXT,
			repository TEXT,
			homepage TEXT,
			documentation TEXT,
			downloads INTEGER DEFAULT 0,
			keywords_json TEXT,
			categories_json TEXT,
			dependencies_json TEXT,
			authors_json TEXT,
			readme TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			indexed_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Rust symbols table
		`CREATE TABLE IF NOT EXISTS rust_symbols (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			kind TEXT NOT NULL,
			signature TEXT,
			crate_id INTEGER NOT NULL,
			crate_name TEXT NOT NULL,
			file_path TEXT,
			line INTEGER DEFAULT 0,
			public INTEGER DEFAULT 0,
			doc TEXT,
			FOREIGN KEY (crate_id) REFERENCES rust_crates(id) ON DELETE CASCADE
		)`,

		// Rust crates FTS table
		`CREATE VIRTUAL TABLE IF NOT EXISTS rust_crates_fts USING fts4(
			name,
			description,
			keywords,
			content=rust_crates,
			tokenize=porter
		)`,

		// Rust symbols FTS table
		`CREATE VIRTUAL TABLE IF NOT EXISTS rust_symbols_fts USING fts4(
			name,
			signature,
			doc,
			content=rust_symbols,
			tokenize=porter
		)`,

		// Triggers for Rust crates FTS
		`CREATE TRIGGER IF NOT EXISTS rust_crates_ai AFTER INSERT ON rust_crates BEGIN
			INSERT INTO rust_crates_fts(docid, name, description, keywords)
			VALUES (new.id, new.name, new.description, new.keywords_json);
		END`,

		`CREATE TRIGGER IF NOT EXISTS rust_crates_ad AFTER DELETE ON rust_crates BEGIN
			DELETE FROM rust_crates_fts WHERE docid = old.id;
		END`,

		`CREATE TRIGGER IF NOT EXISTS rust_crates_au AFTER UPDATE ON rust_crates BEGIN
			DELETE FROM rust_crates_fts WHERE docid = old.id;
			INSERT INTO rust_crates_fts(docid, name, description, keywords)
			VALUES (new.id, new.name, new.description, new.keywords_json);
		END`,

		// Triggers for Rust symbols FTS
		`CREATE TRIGGER IF NOT EXISTS rust_symbols_ai AFTER INSERT ON rust_symbols BEGIN
			INSERT INTO rust_symbols_fts(docid, name, signature, doc)
			VALUES (new.id, new.name, new.signature, new.doc);
		END`,

		`CREATE TRIGGER IF NOT EXISTS rust_symbols_ad AFTER DELETE ON rust_symbols BEGIN
			DELETE FROM rust_symbols_fts WHERE docid = old.id;
		END`,

		`CREATE TRIGGER IF NOT EXISTS rust_symbols_au AFTER UPDATE ON rust_symbols BEGIN
			DELETE FROM rust_symbols_fts WHERE docid = old.id;
			INSERT INTO rust_symbols_fts(docid, name, signature, doc)
			VALUES (new.id, new.name, new.signature, new.doc);
		END`,

		// Python packages table
		`CREATE TABLE IF NOT EXISTS python_packages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT UNIQUE NOT NULL,
			version TEXT,
			summary TEXT,
			author TEXT,
			author_email TEXT,
			license TEXT,
			home_page TEXT,
			project_url TEXT,
			pypi_url TEXT,
			repository_url TEXT,
			documentation_url TEXT,
			requires_python TEXT,
			downloads INTEGER DEFAULT 0,
			keywords_json TEXT,
			classifiers_json TEXT,
			dependencies_json TEXT,
			readme TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			indexed_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Python symbols table
		`CREATE TABLE IF NOT EXISTS python_symbols (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			kind TEXT NOT NULL,
			signature TEXT,
			package_id INTEGER NOT NULL,
			package_name TEXT NOT NULL,
			file_path TEXT,
			line INTEGER DEFAULT 0,
			public INTEGER DEFAULT 0,
			doc TEXT,
			FOREIGN KEY (package_id) REFERENCES python_packages(id) ON DELETE CASCADE
		)`,

		// Python packages FTS table
		`CREATE VIRTUAL TABLE IF NOT EXISTS python_packages_fts USING fts4(
			name,
			summary,
			author,
			keywords,
			content=python_packages,
			tokenize=porter
		)`,

		// Python symbols FTS table
		`CREATE VIRTUAL TABLE IF NOT EXISTS python_symbols_fts USING fts4(
			name,
			signature,
			doc,
			content=python_symbols,
			tokenize=porter
		)`,

		// Triggers for Python packages FTS
		`CREATE TRIGGER IF NOT EXISTS python_packages_ai AFTER INSERT ON python_packages BEGIN
			INSERT INTO python_packages_fts(docid, name, summary, author, keywords)
			VALUES (new.id, new.name, new.summary, new.author, new.keywords_json);
		END`,

		`CREATE TRIGGER IF NOT EXISTS python_packages_ad AFTER DELETE ON python_packages BEGIN
			DELETE FROM python_packages_fts WHERE docid = old.id;
		END`,

		`CREATE TRIGGER IF NOT EXISTS python_packages_au AFTER UPDATE ON python_packages BEGIN
			DELETE FROM python_packages_fts WHERE docid = old.id;
			INSERT INTO python_packages_fts(docid, name, summary, author, keywords)
			VALUES (new.id, new.name, new.summary, new.author, new.keywords_json);
		END`,

		// Triggers for Python symbols FTS
		`CREATE TRIGGER IF NOT EXISTS python_symbols_ai AFTER INSERT ON python_symbols BEGIN
			INSERT INTO python_symbols_fts(docid, name, signature, doc)
			VALUES (new.id, new.name, new.signature, new.doc);
		END`,

		`CREATE TRIGGER IF NOT EXISTS python_symbols_ad AFTER DELETE ON python_symbols BEGIN
			DELETE FROM python_symbols_fts WHERE docid = old.id;
		END`,

		`CREATE TRIGGER IF NOT EXISTS python_symbols_au AFTER UPDATE ON python_symbols BEGIN
			DELETE FROM python_symbols_fts WHERE docid = old.id;
			INSERT INTO python_symbols_fts(docid, name, signature, doc)
			VALUES (new.id, new.name, new.signature, new.doc);
		END`,

		// Indexes for Python
		`CREATE INDEX IF NOT EXISTS idx_python_packages_name ON python_packages(name)`,
		`CREATE INDEX IF NOT EXISTS idx_python_symbols_package ON python_symbols(package_id)`,
		`CREATE INDEX IF NOT EXISTS idx_python_symbols_public ON python_symbols(public)`,
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
		if err := json.Unmarshal([]byte(versionsJSON.String), &pkg.Versions); err != nil {
			return nil, fmt.Errorf("unmarshaling versions: %w", err)
		}
	}
	if goosJSON.Valid {
		if err := json.Unmarshal([]byte(goosJSON.String), &pkg.GOOS); err != nil {
			return nil, fmt.Errorf("unmarshaling goos: %w", err)
		}
	}
	if goarchJSON.Valid {
		if err := json.Unmarshal([]byte(goarchJSON.String), &pkg.GOARCH); err != nil {
			return nil, fmt.Errorf("unmarshaling goarch: %w", err)
		}
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
		INSERT INTO symbols (name, kind, package_id, import_path, synopsis, doc, signature, decl, deprecated)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT DO UPDATE SET
			synopsis = excluded.synopsis,
			doc = excluded.doc,
			signature = excluded.signature,
			decl = excluded.decl,
			deprecated = excluded.deprecated
	`, symbol.Name, symbol.Kind, symbol.PackageID, symbol.ImportPath, symbol.Synopsis, symbol.Doc, symbol.Signature, symbol.Decl, symbol.Deprecated)
	return err
}

// DeletePackageSymbols deletes all symbols for a package
func (db *DB) DeletePackageSymbols(packageID int64) error {
	_, err := db.conn.Exec("DELETE FROM symbols WHERE package_id = ?", packageID)
	return err
}

// GetPackageSymbols returns all symbols for a package
func (db *DB) GetPackageSymbols(packageID int64) ([]*Symbol, error) {
	rows, err := db.conn.Query(`
		SELECT id, name, kind, package_id, import_path, synopsis, doc, signature, decl, deprecated
		FROM symbols WHERE package_id = ?
		ORDER BY kind, name
	`, packageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var symbols []*Symbol
	for rows.Next() {
		sym := &Symbol{}
		var doc, signature, decl sql.NullString
		if err := rows.Scan(&sym.ID, &sym.Name, &sym.Kind, &sym.PackageID, &sym.ImportPath, &sym.Synopsis, &doc, &signature, &decl, &sym.Deprecated); err != nil {
			return nil, err
		}
		sym.Doc = doc.String
		sym.Signature = signature.String
		sym.Decl = decl.String
		symbols = append(symbols, sym)
	}
	return symbols, rows.Err()
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

// UpsertModuleVersion inserts or updates a module version
func (db *DB) UpsertModuleVersion(mv *ModuleVersion) error {
	_, err := db.conn.Exec(`
		INSERT INTO module_versions (module_path, version, timestamp, is_tagged, is_stable, retracted)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(module_path, version) DO UPDATE SET
			timestamp = COALESCE(excluded.timestamp, module_versions.timestamp),
			is_tagged = excluded.is_tagged,
			is_stable = excluded.is_stable,
			retracted = excluded.retracted
	`, mv.ModulePath, mv.Version, mv.Timestamp, mv.IsTagged, mv.IsStable, mv.Retracted)
	return err
}

// GetModuleVersions returns all versions for a module, sorted by semver (newest first)
func (db *DB) GetModuleVersions(modulePath string) ([]*ModuleVersion, error) {
	rows, err := db.conn.Query(`
		SELECT id, module_path, version, timestamp, is_tagged, is_stable, retracted, created_at
		FROM module_versions
		WHERE module_path = ?
		ORDER BY
			CASE WHEN version LIKE 'v%' THEN 0 ELSE 1 END,
			CAST(SUBSTR(version, 2, INSTR(SUBSTR(version, 2), '.') - 1) AS INTEGER) DESC,
			timestamp DESC
	`, modulePath)
	if err != nil {
		return nil, fmt.Errorf("querying versions: %w", err)
	}
	defer rows.Close()

	var versions []*ModuleVersion
	for rows.Next() {
		mv := &ModuleVersion{}
		var timestamp sql.NullTime
		err := rows.Scan(&mv.ID, &mv.ModulePath, &mv.Version, &timestamp,
			&mv.IsTagged, &mv.IsStable, &mv.Retracted, &mv.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("scanning version: %w", err)
		}
		if timestamp.Valid {
			mv.Timestamp = timestamp.Time
		}
		versions = append(versions, mv)
	}

	return versions, rows.Err()
}

// GetModuleVersion returns a specific version of a module
func (db *DB) GetModuleVersion(modulePath, version string) (*ModuleVersion, error) {
	row := db.conn.QueryRow(`
		SELECT id, module_path, version, timestamp, is_tagged, is_stable, retracted, created_at
		FROM module_versions
		WHERE module_path = ? AND version = ?
	`, modulePath, version)

	mv := &ModuleVersion{}
	var timestamp sql.NullTime
	err := row.Scan(&mv.ID, &mv.ModulePath, &mv.Version, &timestamp,
		&mv.IsTagged, &mv.IsStable, &mv.Retracted, &mv.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scanning version: %w", err)
	}
	if timestamp.Valid {
		mv.Timestamp = timestamp.Time
	}

	return mv, nil
}

// GetLatestModuleVersion returns the latest version for a module
func (db *DB) GetLatestModuleVersion(modulePath string) (*ModuleVersion, error) {
	row := db.conn.QueryRow(`
		SELECT id, module_path, version, timestamp, is_tagged, is_stable, retracted, created_at
		FROM module_versions
		WHERE module_path = ? AND retracted = 0
		ORDER BY
			CASE WHEN is_stable = 1 THEN 0 ELSE 1 END,
			timestamp DESC
		LIMIT 1
	`, modulePath)

	mv := &ModuleVersion{}
	var timestamp sql.NullTime
	err := row.Scan(&mv.ID, &mv.ModulePath, &mv.Version, &timestamp,
		&mv.IsTagged, &mv.IsStable, &mv.Retracted, &mv.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scanning version: %w", err)
	}
	if timestamp.Valid {
		mv.Timestamp = timestamp.Time
	}

	return mv, nil
}

// CountModuleVersions returns the number of versions for a module
func (db *DB) CountModuleVersions(modulePath string) (int, error) {
	var count int
	err := db.conn.QueryRow(`
		SELECT COUNT(*) FROM module_versions WHERE module_path = ?
	`, modulePath).Scan(&count)
	return count, err
}

// UpsertAIDoc inserts or updates an AI-generated doc
func (db *DB) UpsertAIDoc(doc *AIDoc) error {
	_, err := db.conn.Exec(`
		INSERT INTO ai_docs (symbol_name, symbol_kind, import_path, generated_doc, approved, flagged, flag_reason, cost_usd, tokens)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(import_path, symbol_name, symbol_kind) DO UPDATE SET
			generated_doc = excluded.generated_doc,
			updated_at = CURRENT_TIMESTAMP
	`, doc.SymbolName, doc.SymbolKind, doc.ImportPath, doc.GeneratedDoc, doc.Approved, doc.Flagged, doc.FlagReason, doc.CostUSD, doc.Tokens)
	return err
}

// GetAIDoc retrieves an AI-generated doc for a symbol
func (db *DB) GetAIDoc(importPath, symbolName, symbolKind string) (*AIDoc, error) {
	row := db.conn.QueryRow(`
		SELECT id, symbol_name, symbol_kind, import_path, generated_doc, approved, flagged, flag_reason, cost_usd, tokens, created_at, updated_at
		FROM ai_docs
		WHERE import_path = ? AND symbol_name = ? AND symbol_kind = ?
	`, importPath, symbolName, symbolKind)

	doc := &AIDoc{}
	var flagReason sql.NullString
	err := row.Scan(&doc.ID, &doc.SymbolName, &doc.SymbolKind, &doc.ImportPath, &doc.GeneratedDoc,
		&doc.Approved, &doc.Flagged, &flagReason, &doc.CostUSD, &doc.Tokens, &doc.CreatedAt, &doc.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scanning ai doc: %w", err)
	}

	if flagReason.Valid {
		doc.FlagReason = flagReason.String
	}

	return doc, nil
}

// GetAIDocsForPackage retrieves all AI-generated docs for a package
func (db *DB) GetAIDocsForPackage(importPath string) ([]*AIDoc, error) {
	rows, err := db.conn.Query(`
		SELECT id, symbol_name, symbol_kind, import_path, generated_doc, approved, flagged, flag_reason, cost_usd, tokens, created_at, updated_at
		FROM ai_docs
		WHERE import_path = ?
		ORDER BY symbol_kind, symbol_name
	`, importPath)
	if err != nil {
		return nil, fmt.Errorf("querying ai docs: %w", err)
	}
	defer rows.Close()

	var docs []*AIDoc
	for rows.Next() {
		doc := &AIDoc{}
		var flagReason sql.NullString
		err := rows.Scan(&doc.ID, &doc.SymbolName, &doc.SymbolKind, &doc.ImportPath, &doc.GeneratedDoc,
			&doc.Approved, &doc.Flagged, &flagReason, &doc.CostUSD, &doc.Tokens, &doc.CreatedAt, &doc.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("scanning ai doc: %w", err)
		}
		if flagReason.Valid {
			doc.FlagReason = flagReason.String
		}
		docs = append(docs, doc)
	}

	return docs, rows.Err()
}

// ApproveAIDoc marks an AI-generated doc as approved
func (db *DB) ApproveAIDoc(id int64) error {
	_, err := db.conn.Exec(`UPDATE ai_docs SET approved = 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
	return err
}

// FlagAIDoc marks an AI-generated doc as flagged with a reason
func (db *DB) FlagAIDoc(id int64, reason string) error {
	_, err := db.conn.Exec(`UPDATE ai_docs SET flagged = 1, flag_reason = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, reason, id)
	return err
}

// GetAIDocStats returns statistics about AI-generated documentation
func (db *DB) GetAIDocStats() (totalDocs, approvedDocs, flaggedDocs int, totalCost float64, err error) {
	err = db.conn.QueryRow(`
		SELECT
			COUNT(*),
			SUM(CASE WHEN approved = 1 THEN 1 ELSE 0 END),
			SUM(CASE WHEN flagged = 1 THEN 1 ELSE 0 END),
			SUM(cost_usd)
		FROM ai_docs
	`).Scan(&totalDocs, &approvedDocs, &flaggedDocs, &totalCost)
	return
}

// JSPackage represents a JavaScript/TypeScript package
type JSPackage struct {
	ID             int64
	Name           string
	Version        string
	Description    string
	Author         string
	License        string
	RepositoryURL  string
	Homepage       string
	NPMURL         string
	GitHubURL      string
	MainFile       string
	TypesFile      string
	HasTypeScript  bool
	Stars          int
	Forks          int
	Keywords       []string
	Dependencies   map[string]string
	PackageJSON    string
	README         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	IndexedAt      time.Time
}

// JSSymbol represents a JavaScript/TypeScript symbol
type JSSymbol struct {
	ID          int64
	Name        string
	Kind        string // function, class, method, interface, type, enum, const
	Signature   string
	PackageID   int64
	PackageName string
	FilePath    string
	Line        int
	Exported    bool
	Doc         string
	Deprecated  bool
}

// UpsertJSPackage inserts or updates a JavaScript/TypeScript package
func (db *DB) UpsertJSPackage(pkg *JSPackage) (int64, error) {
	keywordsJSON, _ := json.Marshal(pkg.Keywords)
	dependenciesJSON, _ := json.Marshal(pkg.Dependencies)

	result, err := db.conn.Exec(`
		INSERT INTO js_packages (
			name, version, description, author, license, repository_url,
			homepage, npm_url, github_url, main_file, types_file,
			has_typescript, stars, forks, keywords_json, dependencies_json,
			package_json, readme, indexed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'))
		ON CONFLICT(name) DO UPDATE SET
			version=excluded.version,
			description=excluded.description,
			author=excluded.author,
			license=excluded.license,
			repository_url=excluded.repository_url,
			homepage=excluded.homepage,
			npm_url=excluded.npm_url,
			github_url=excluded.github_url,
			main_file=excluded.main_file,
			types_file=excluded.types_file,
			has_typescript=excluded.has_typescript,
			stars=excluded.stars,
			forks=excluded.forks,
			keywords_json=excluded.keywords_json,
			dependencies_json=excluded.dependencies_json,
			package_json=excluded.package_json,
			readme=excluded.readme,
			updated_at=datetime('now'),
			indexed_at=datetime('now')
	`, pkg.Name, pkg.Version, pkg.Description, pkg.Author, pkg.License,
		pkg.RepositoryURL, pkg.Homepage, pkg.NPMURL, pkg.GitHubURL,
		pkg.MainFile, pkg.TypesFile, pkg.HasTypeScript, pkg.Stars, pkg.Forks,
		keywordsJSON, dependenciesJSON, pkg.PackageJSON, pkg.README)

	if err != nil {
		return 0, err
	}

	return result.LastInsertId()
}

// UpsertJSSymbol inserts or updates a JavaScript/TypeScript symbol
func (db *DB) UpsertJSSymbol(sym *JSSymbol) error {
	_, err := db.conn.Exec(`
		INSERT INTO js_symbols (
			name, kind, signature, package_id, package_name,
			file_path, line, exported, doc, deprecated
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			signature=excluded.signature,
			file_path=excluded.file_path,
			line=excluded.line,
			exported=excluded.exported,
			doc=excluded.doc,
			deprecated=excluded.deprecated
	`, sym.Name, sym.Kind, sym.Signature, sym.PackageID, sym.PackageName,
		sym.FilePath, sym.Line, sym.Exported, sym.Doc, sym.Deprecated)

	return err
}

// DeleteJSPackageSymbols deletes all symbols for a package
func (db *DB) DeleteJSPackageSymbols(packageID int64) error {
	_, err := db.conn.Exec("DELETE FROM js_symbols WHERE package_id = ?", packageID)
	return err
}

// SearchJSPackages searches for JavaScript/TypeScript packages
func (db *DB) SearchJSPackages(query string, limit int) ([]*JSPackage, error) {
	rows, err := db.conn.Query(`
		SELECT id, name, version, description, author, license, stars, forks
		FROM js_packages
		WHERE id IN (
			SELECT docid FROM js_packages_fts
			WHERE js_packages_fts MATCH ?
			LIMIT ?
		)
		ORDER BY stars DESC
	`, query, limit)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var packages []*JSPackage
	for rows.Next() {
		var pkg JSPackage
		if err := rows.Scan(&pkg.ID, &pkg.Name, &pkg.Version, &pkg.Description,
			&pkg.Author, &pkg.License, &pkg.Stars, &pkg.Forks); err != nil {
			return nil, err
		}
		packages = append(packages, &pkg)
	}

	return packages, nil
}

// SearchJSSymbols searches for JavaScript/TypeScript symbols
func (db *DB) SearchJSSymbols(query string, limit int) ([]*JSSymbol, error) {
	rows, err := db.conn.Query(`
		SELECT id, name, kind, signature, package_name, file_path, line
		FROM js_symbols
		WHERE id IN (
			SELECT docid FROM js_symbols_fts
			WHERE js_symbols_fts MATCH ?
			LIMIT ?
		)
	`, query, limit)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var symbols []*JSSymbol
	for rows.Next() {
		var sym JSSymbol
		if err := rows.Scan(&sym.ID, &sym.Name, &sym.Kind, &sym.Signature,
			&sym.PackageName, &sym.FilePath, &sym.Line); err != nil {
			return nil, err
		}
		symbols = append(symbols, &sym)
	}

	return symbols, nil
}

// GetJSPackage retrieves a JavaScript/TypeScript package by name
func (db *DB) GetJSPackage(name string) (*JSPackage, error) {
	var pkg JSPackage
	var keywordsJSON, dependenciesJSON sql.NullString

	err := db.conn.QueryRow(`
		SELECT id, name, version, description, author, license,
			repository_url, homepage, npm_url, github_url,
			main_file, types_file, has_typescript, stars, forks,
			keywords_json, dependencies_json, package_json, readme,
			created_at, updated_at, indexed_at
		FROM js_packages WHERE name = ?
	`, name).Scan(&pkg.ID, &pkg.Name, &pkg.Version, &pkg.Description,
		&pkg.Author, &pkg.License, &pkg.RepositoryURL, &pkg.Homepage,
		&pkg.NPMURL, &pkg.GitHubURL, &pkg.MainFile, &pkg.TypesFile,
		&pkg.HasTypeScript, &pkg.Stars, &pkg.Forks,
		&keywordsJSON, &dependenciesJSON, &pkg.PackageJSON, &pkg.README,
		&pkg.CreatedAt, &pkg.UpdatedAt, &pkg.IndexedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if keywordsJSON.Valid {
		json.Unmarshal([]byte(keywordsJSON.String), &pkg.Keywords)
	}
	if dependenciesJSON.Valid {
		json.Unmarshal([]byte(dependenciesJSON.String), &pkg.Dependencies)
	}

	return &pkg, nil
}

// RustCrate represents a Rust crate
type RustCrate struct {
	ID             int64
	Name           string
	Version        string
	Description    string
	License        string
	Repository     string
	Homepage       string
	Documentation  string
	Downloads      int
	Keywords       []string
	Categories     []string
	Dependencies   map[string]string
	Authors        []string
	README         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	IndexedAt      time.Time
}

// RustSymbol represents a Rust symbol
type RustSymbol struct {
	ID        int64
	Name      string
	Kind      string
	Signature string
	CrateID   int64
	CrateName string
	FilePath  string
	Line      int
	Public    bool
	Doc       string
}

// UpsertRustCrate inserts or updates a Rust crate
func (db *DB) UpsertRustCrate(crate *RustCrate) (int64, error) {
	keywordsJSON, _ := json.Marshal(crate.Keywords)
	categoriesJSON, _ := json.Marshal(crate.Categories)
	dependenciesJSON, _ := json.Marshal(crate.Dependencies)
	authorsJSON, _ := json.Marshal(crate.Authors)

	result, err := db.conn.Exec(`
		INSERT INTO rust_crates (name, version, description, license, repository,
			homepage, documentation, downloads, keywords_json, categories_json,
			dependencies_json, authors_json, readme, updated_at, indexed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(name) DO UPDATE SET
			version = excluded.version,
			description = excluded.description,
			license = excluded.license,
			repository = excluded.repository,
			homepage = excluded.homepage,
			documentation = excluded.documentation,
			downloads = excluded.downloads,
			keywords_json = excluded.keywords_json,
			categories_json = excluded.categories_json,
			dependencies_json = excluded.dependencies_json,
			authors_json = excluded.authors_json,
			readme = excluded.readme,
			updated_at = CURRENT_TIMESTAMP,
			indexed_at = CURRENT_TIMESTAMP
	`, crate.Name, crate.Version, crate.Description, crate.License, crate.Repository,
		crate.Homepage, crate.Documentation, crate.Downloads, string(keywordsJSON),
		string(categoriesJSON), string(dependenciesJSON), string(authorsJSON), crate.README)

	if err != nil {
		return 0, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		// If LastInsertId fails (e.g., on UPDATE), query for the ID
		var crateID int64
		err = db.conn.QueryRow("SELECT id FROM rust_crates WHERE name = ?", crate.Name).Scan(&crateID)
		if err != nil {
			return 0, err
		}
		return crateID, nil
	}

	return id, nil
}

// UpsertRustSymbol inserts or updates a Rust symbol
func (db *DB) UpsertRustSymbol(sym *RustSymbol) error {
	_, err := db.conn.Exec(`
		INSERT OR REPLACE INTO rust_symbols
		(name, kind, signature, crate_id, crate_name, file_path, line, public, doc)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, sym.Name, sym.Kind, sym.Signature, sym.CrateID, sym.CrateName,
		sym.FilePath, sym.Line, sym.Public, sym.Doc)

	return err
}

// DeleteRustCrateSymbols deletes all symbols for a crate
func (db *DB) DeleteRustCrateSymbols(crateID int64) error {
	_, err := db.conn.Exec("DELETE FROM rust_symbols WHERE crate_id = ?", crateID)
	return err
}

// SearchRustCrates searches for Rust crates using FTS
func (db *DB) SearchRustCrates(query string, limit int) ([]*RustCrate, error) {
	if query == "" {
		query = "*"
	}

	rows, err := db.conn.Query(`
		SELECT id, name, version, description, license, repository, homepage,
			documentation, downloads, keywords_json, categories_json,
			dependencies_json, authors_json, readme, created_at, updated_at, indexed_at
		FROM rust_crates
		WHERE id IN (
			SELECT docid FROM rust_crates_fts
			WHERE rust_crates_fts MATCH ?
			LIMIT ?
		)
	`, query, limit)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var crates []*RustCrate
	for rows.Next() {
		var crate RustCrate
		var keywordsJSON, categoriesJSON, dependenciesJSON, authorsJSON sql.NullString

		if err := rows.Scan(&crate.ID, &crate.Name, &crate.Version, &crate.Description,
			&crate.License, &crate.Repository, &crate.Homepage, &crate.Documentation,
			&crate.Downloads, &keywordsJSON, &categoriesJSON, &dependenciesJSON,
			&authorsJSON, &crate.README, &crate.CreatedAt, &crate.UpdatedAt,
			&crate.IndexedAt); err != nil {
			return nil, err
		}

		if keywordsJSON.Valid {
			json.Unmarshal([]byte(keywordsJSON.String), &crate.Keywords)
		}
		if categoriesJSON.Valid {
			json.Unmarshal([]byte(categoriesJSON.String), &crate.Categories)
		}
		if dependenciesJSON.Valid {
			json.Unmarshal([]byte(dependenciesJSON.String), &crate.Dependencies)
		}
		if authorsJSON.Valid {
			json.Unmarshal([]byte(authorsJSON.String), &crate.Authors)
		}

		crates = append(crates, &crate)
	}

	return crates, nil
}

// SearchRustSymbols searches for Rust symbols using FTS
func (db *DB) SearchRustSymbols(query string, limit int) ([]*RustSymbol, error) {
	if query == "" {
		query = "*"
	}

	rows, err := db.conn.Query(`
		SELECT id, name, kind, signature, crate_name, file_path, line
		FROM rust_symbols
		WHERE id IN (
			SELECT docid FROM rust_symbols_fts
			WHERE rust_symbols_fts MATCH ?
			LIMIT ?
		)
	`, query, limit)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var symbols []*RustSymbol
	for rows.Next() {
		var sym RustSymbol
		if err := rows.Scan(&sym.ID, &sym.Name, &sym.Kind, &sym.Signature,
			&sym.CrateName, &sym.FilePath, &sym.Line); err != nil {
			return nil, err
		}
		symbols = append(symbols, &sym)
	}

	return symbols, nil
}

// GetRustCrate retrieves a Rust crate by name
func (db *DB) GetRustCrate(name string) (*RustCrate, error) {
	var crate RustCrate
	var keywordsJSON, categoriesJSON, dependenciesJSON, authorsJSON sql.NullString

	err := db.conn.QueryRow(`
		SELECT id, name, version, description, license, repository, homepage,
			documentation, downloads, keywords_json, categories_json,
			dependencies_json, authors_json, readme, created_at, updated_at, indexed_at
		FROM rust_crates WHERE name = ?
	`, name).Scan(&crate.ID, &crate.Name, &crate.Version, &crate.Description,
		&crate.License, &crate.Repository, &crate.Homepage, &crate.Documentation,
		&crate.Downloads, &keywordsJSON, &categoriesJSON, &dependenciesJSON,
		&authorsJSON, &crate.README, &crate.CreatedAt, &crate.UpdatedAt,
		&crate.IndexedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if keywordsJSON.Valid {
		json.Unmarshal([]byte(keywordsJSON.String), &crate.Keywords)
	}
	if categoriesJSON.Valid {
		json.Unmarshal([]byte(categoriesJSON.String), &crate.Categories)
	}
	if dependenciesJSON.Valid {
		json.Unmarshal([]byte(dependenciesJSON.String), &crate.Dependencies)
	}
	if authorsJSON.Valid {
		json.Unmarshal([]byte(authorsJSON.String), &crate.Authors)
	}

	return &crate, nil
}

// GetJSPackageSymbols returns all symbols for a JS package
func (db *DB) GetJSPackageSymbols(packageID int64) ([]*JSSymbol, error) {
	rows, err := db.conn.Query(`
		SELECT id, name, kind, signature, package_id, package_name, file_path, line, exported, doc, deprecated
		FROM js_symbols WHERE package_id = ? AND exported = 1
		ORDER BY kind, name
	`, packageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var symbols []*JSSymbol
	for rows.Next() {
		sym := &JSSymbol{}
		var doc sql.NullString
		if err := rows.Scan(&sym.ID, &sym.Name, &sym.Kind, &sym.Signature, &sym.PackageID, &sym.PackageName, &sym.FilePath, &sym.Line, &sym.Exported, &doc, &sym.Deprecated); err != nil {
			return nil, err
		}
		sym.Doc = doc.String
		symbols = append(symbols, sym)
	}
	return symbols, rows.Err()
}

// GetRustCrateSymbols returns all symbols for a Rust crate
func (db *DB) GetRustCrateSymbols(crateID int64) ([]*RustSymbol, error) {
	rows, err := db.conn.Query(`
		SELECT id, name, kind, signature, crate_id, crate_name, file_path, line, public, doc
		FROM rust_symbols WHERE crate_id = ? AND public = 1
		ORDER BY kind, name
	`, crateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var symbols []*RustSymbol
	for rows.Next() {
		sym := &RustSymbol{}
		var doc sql.NullString
		if err := rows.Scan(&sym.ID, &sym.Name, &sym.Kind, &sym.Signature, &sym.CrateID, &sym.CrateName, &sym.FilePath, &sym.Line, &sym.Public, &doc); err != nil {
			return nil, err
		}
		sym.Doc = doc.String
		symbols = append(symbols, sym)
	}
	return symbols, rows.Err()
}

// PythonPackage represents a Python package from PyPI
type PythonPackage struct {
	ID               int64
	Name             string
	Version          string
	Summary          string
	Author           string
	AuthorEmail      string
	License          string
	HomePage         string
	ProjectURL       string
	PyPIURL          string
	RepositoryURL    string
	DocumentationURL string
	RequiresPython   string
	Downloads        int
	Keywords         []string
	Classifiers      []string
	Dependencies     []string
	README           string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	IndexedAt        time.Time
}

// PythonSymbol represents a Python symbol
type PythonSymbol struct {
	ID          int64
	Name        string
	Kind        string
	Signature   string
	PackageID   int64
	PackageName string
	FilePath    string
	Line        int
	Public      bool
	Doc         string
}

// UpsertPythonPackage inserts or updates a Python package
func (db *DB) UpsertPythonPackage(pkg *PythonPackage) (int64, error) {
	keywordsJSON, _ := json.Marshal(pkg.Keywords)
	classifiersJSON, _ := json.Marshal(pkg.Classifiers)
	dependenciesJSON, _ := json.Marshal(pkg.Dependencies)

	result, err := db.conn.Exec(`
		INSERT INTO python_packages (name, version, summary, author, author_email,
			license, home_page, project_url, pypi_url, repository_url,
			documentation_url, requires_python, downloads, keywords_json,
			classifiers_json, dependencies_json, readme, updated_at, indexed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(name) DO UPDATE SET
			version = excluded.version,
			summary = excluded.summary,
			author = excluded.author,
			author_email = excluded.author_email,
			license = excluded.license,
			home_page = excluded.home_page,
			project_url = excluded.project_url,
			pypi_url = excluded.pypi_url,
			repository_url = excluded.repository_url,
			documentation_url = excluded.documentation_url,
			requires_python = excluded.requires_python,
			downloads = excluded.downloads,
			keywords_json = excluded.keywords_json,
			classifiers_json = excluded.classifiers_json,
			dependencies_json = excluded.dependencies_json,
			readme = excluded.readme,
			updated_at = CURRENT_TIMESTAMP,
			indexed_at = CURRENT_TIMESTAMP
	`, pkg.Name, pkg.Version, pkg.Summary, pkg.Author, pkg.AuthorEmail,
		pkg.License, pkg.HomePage, pkg.ProjectURL, pkg.PyPIURL, pkg.RepositoryURL,
		pkg.DocumentationURL, pkg.RequiresPython, pkg.Downloads, string(keywordsJSON),
		string(classifiersJSON), string(dependenciesJSON), pkg.README)

	if err != nil {
		return 0, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		var pkgID int64
		err = db.conn.QueryRow("SELECT id FROM python_packages WHERE name = ?", pkg.Name).Scan(&pkgID)
		if err != nil {
			return 0, err
		}
		return pkgID, nil
	}

	return id, nil
}

// GetPythonPackage retrieves a Python package by name
func (db *DB) GetPythonPackage(name string) (*PythonPackage, error) {
	var pkg PythonPackage
	var keywordsJSON, classifiersJSON, dependenciesJSON sql.NullString

	err := db.conn.QueryRow(`
		SELECT id, name, version, summary, author, author_email, license,
			home_page, project_url, pypi_url, repository_url, documentation_url,
			requires_python, downloads, keywords_json, classifiers_json,
			dependencies_json, readme, created_at, updated_at, indexed_at
		FROM python_packages WHERE name = ?
	`, name).Scan(&pkg.ID, &pkg.Name, &pkg.Version, &pkg.Summary, &pkg.Author,
		&pkg.AuthorEmail, &pkg.License, &pkg.HomePage, &pkg.ProjectURL,
		&pkg.PyPIURL, &pkg.RepositoryURL, &pkg.DocumentationURL,
		&pkg.RequiresPython, &pkg.Downloads, &keywordsJSON, &classifiersJSON,
		&dependenciesJSON, &pkg.README, &pkg.CreatedAt, &pkg.UpdatedAt, &pkg.IndexedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if keywordsJSON.Valid {
		json.Unmarshal([]byte(keywordsJSON.String), &pkg.Keywords)
	}
	if classifiersJSON.Valid {
		json.Unmarshal([]byte(classifiersJSON.String), &pkg.Classifiers)
	}
	if dependenciesJSON.Valid {
		json.Unmarshal([]byte(dependenciesJSON.String), &pkg.Dependencies)
	}

	return &pkg, nil
}

// UpsertPythonSymbol inserts or updates a Python symbol
func (db *DB) UpsertPythonSymbol(sym *PythonSymbol) error {
	_, err := db.conn.Exec(`
		INSERT OR REPLACE INTO python_symbols
		(name, kind, signature, package_id, package_name, file_path, line, public, doc)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, sym.Name, sym.Kind, sym.Signature, sym.PackageID, sym.PackageName,
		sym.FilePath, sym.Line, sym.Public, sym.Doc)

	return err
}

// DeletePythonPackageSymbols deletes all symbols for a Python package
func (db *DB) DeletePythonPackageSymbols(packageID int64) error {
	_, err := db.conn.Exec("DELETE FROM python_symbols WHERE package_id = ?", packageID)
	return err
}

// GetPythonPackageSymbols returns all public symbols for a Python package
func (db *DB) GetPythonPackageSymbols(packageID int64) ([]*PythonSymbol, error) {
	rows, err := db.conn.Query(`
		SELECT id, name, kind, signature, package_id, package_name, file_path, line, public, doc
		FROM python_symbols WHERE package_id = ? AND public = 1
		ORDER BY kind, name
	`, packageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var symbols []*PythonSymbol
	for rows.Next() {
		sym := &PythonSymbol{}
		var doc sql.NullString
		if err := rows.Scan(&sym.ID, &sym.Name, &sym.Kind, &sym.Signature, &sym.PackageID,
			&sym.PackageName, &sym.FilePath, &sym.Line, &sym.Public, &doc); err != nil {
			return nil, err
		}
		sym.Doc = doc.String
		symbols = append(symbols, sym)
	}
	return symbols, rows.Err()
}

// SearchPythonPackages searches for Python packages using FTS
func (db *DB) SearchPythonPackages(query string, limit int) ([]*PythonPackage, error) {
	if query == "" {
		query = "*"
	}

	rows, err := db.conn.Query(`
		SELECT id, name, version, summary, author, license, downloads
		FROM python_packages
		WHERE id IN (
			SELECT docid FROM python_packages_fts
			WHERE python_packages_fts MATCH ?
			LIMIT ?
		)
	`, query, limit)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var packages []*PythonPackage
	for rows.Next() {
		pkg := &PythonPackage{}
		if err := rows.Scan(&pkg.ID, &pkg.Name, &pkg.Version, &pkg.Summary,
			&pkg.Author, &pkg.License, &pkg.Downloads); err != nil {
			return nil, err
		}
		packages = append(packages, pkg)
	}

	return packages, nil
}

// SearchPythonSymbols searches for Python symbols using FTS
func (db *DB) SearchPythonSymbols(query string, limit int) ([]*PythonSymbol, error) {
	if query == "" {
		query = "*"
	}

	rows, err := db.conn.Query(`
		SELECT id, name, kind, signature, package_name, file_path, line
		FROM python_symbols
		WHERE id IN (
			SELECT docid FROM python_symbols_fts
			WHERE python_symbols_fts MATCH ?
			LIMIT ?
		)
	`, query, limit)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var symbols []*PythonSymbol
	for rows.Next() {
		sym := &PythonSymbol{}
		if err := rows.Scan(&sym.ID, &sym.Name, &sym.Kind, &sym.Signature,
			&sym.PackageName, &sym.FilePath, &sym.Line); err != nil {
			return nil, err
		}
		symbols = append(symbols, sym)
	}

	return symbols, nil
}
