# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Workflow Requirements

**CRITICAL:** After every meaningful change:
1. Run tests to verify changes work
2. Update `todo.txt` if completing roadmap items
3. Commit and push in a single commit with the todo.txt update
4. Use conventional commits with extra short messages, no description body
5. Format: `feat: add x` / `fix: y bug` / `refactor: z` (george hotz style - minimal, working, tested)

## Build & Development Commands

```bash
# Build all commands
go build ./cmd/...

# Build specific command
go build -o serve ./cmd/serve
go build -o crawl ./cmd/crawl
go build -o crawljs ./cmd/crawljs
go build -o crawlrs ./cmd/crawlrs
go build -o crawlpy ./cmd/crawlpy
go build -o crawlphp ./cmd/crawlphp

# Run the server
./serve -db wikigo.db -addr :8080

# Run all tests
go test ./...

# Run tests for specific package
go test ./crawler/...
go test ./jsparser/...
go test ./rsparser/...
go test ./db/...

# Run single test
go test -run TestFunctionName ./package/...
```

## Architecture Overview

Wikigo is a multi-language documentation viewer (pkg.go.dev clone) supporting Go, JS/TS, Rust, Python, and PHP packages.

### Core Components

- **`main.go`**: CLI tool that extracts Go package documentation to JSON using `go/doc` and `go/ast`
- **`web/server.go`**: HTTP server with embedded templates/static files, handles all routes and renders documentation pages
- **`db/db.go`**: SQLite database layer with FTS4 full-text search, multi-language schema (packages, symbols, imports, AI docs)
- **`ai/`**: Mistral AI integration for code explanations and doc generation

### Crawlers (`crawler/`)

Each language has its own crawler:
- `crawler.go` - Go modules from proxy.golang.org
- `npm.go` - NPM packages from registry.npmjs.org
- `github.go` - GitHub repositories
- `crates.go` - Rust crates from crates.io
- `pypi.go` - Python packages from PyPI
- `packagist.go` - PHP packages from Packagist

### Parsers

Language-specific symbol extraction:
- `jsparser/` - JS/TS parsing via esbuild
- `rsparser/` - Rust symbol extraction
- `pyparser/` - Python symbol extraction
- `phpparser/` - PHP symbol extraction

### Database Schema

Multi-language tables with FTS indexes:
- Go: `packages`, `symbols`, `imports`, `packages_fts`, `symbols_fts`
- JS: `js_packages`, `js_symbols`, `js_packages_fts`, `js_symbols_fts`
- Rust: `rust_crates`, `rust_symbols`, `rust_crates_fts`, `rust_symbols_fts`
- Python: `python_packages`, `python_symbols`, `python_packages_fts`
- PHP: `php_packages`, `php_symbols`, `php_packages_fts`
- AI: `ai_docs` (generated documentation)
- Meta: `crawl_metadata`, `module_versions`

### Web Routes

Package docs: `/{import-path}`, `/crates.io/{name}`, `/npm/{name}`, `/pypi/{name}`, `/packagist/{name}`
Search: `/search?q=`, `/symbols?q=`, `/api/search?q=&lang=`
Utilities: `/badge/`, `/license/`, `/imports/`, `/importedby/`, `/versions/`, `/diff/`, `/compare/`
AI: `/api/explain` (POST)

### Shared Utilities

`util/util.go` contains common helpers: `IdentifyLicense`, `ModuleToRepoURL`, `IsDeprecated`, `IsRedistributable`

## Environment Variables

- `MISTRAL_API_KEY` - Enables AI features
- `GITHUB_TOKEN` - Higher rate limits for GitHub API
