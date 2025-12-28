# wikigo

Multi-language documentation viewer for Go, JavaScript/TypeScript, and Rust packages with AI-powered features.

## Overview

wikigo is an open-source documentation platform that indexes and serves package documentation for multiple programming languages:
- **Go packages** from proxy.golang.org
- **JavaScript/TypeScript packages** from npm and GitHub
- **Rust crates** from crates.io

## Features

### Multi-Language Support
- **Go**: Full pkg.go.dev clone with module indexing
- **JavaScript/TypeScript**: NPM packages and GitHub repositories
- **Rust**: Crates from crates.io with symbol extraction

### Package Documentation
- Full package/crate documentation with syntax highlighting
- Functions, types, methods, constants, and variables
- Collapsible sections and jump-to navigation
- Source file links with line numbers
- Cross-package type linking
- Doc comment parsing (GoDoc, JSDoc, Rust doc comments)

### Search & Discovery
- Full-text search across packages, crates, and symbols
- Symbol search with type filtering (functions, types, methods)
- Search result highlighting
- Autocomplete suggestions
- Language filtering (Go, JS/TS, Rust)

### AI-Powered Features
- **Code Explanation**: AI-powered "Explain this code" for functions and methods
- **Auto-documentation**: Generate missing doc comments
- **Package Synopsis**: Auto-generate package descriptions
- Feature flags and cost tracking for AI usage
- Powered by Mistral AI

### Version Management
- Version history tracking with timestamps
- Tagged/stable/pre-release indicators
- API diff between versions
- Package comparison view

### Module Indexing
- **Go**: Crawls modules from proxy.golang.org
- **JavaScript/TypeScript**: Crawls npm registry and GitHub
- **Rust**: Crawls crates.io
- Stores documentation in SQLite with FTS4
- Tracks import/dependency relationships
- Periodic re-indexing with daemon mode

### UI Features
- Dark mode toggle
- Keyboard shortcuts (/ for search)
- Mobile-responsive design
- Copy buttons for code blocks and import paths
- Breadcrumb navigation
- Sticky sidebar navigation

## Installation

### Prerequisites
- Go 1.25.0 or later
- SQLite 3

### Quick Start

```bash
# Clone the repository
git clone https://github.com/alexisbouchez/wikigo.git
cd wikigo

# Run the interactive setup script
go run ./cmd/setup
```

### Manual Installation

```bash
# Build all binaries
go build ./cmd/serve
go build ./cmd/crawl
go build ./cmd/crawljs
go build ./cmd/crawlrs

# Or build everything at once
go build ./...
```

## Usage

### Serve Local Documentation

```bash
# Serve documentation for Go packages in a directory
./serve -dir /path/to/your/go/packages

# With database for search and indexing
./serve -dir /path/to/packages -db wikigo.db

# Custom port
./serve -addr :3000
```

### Crawl Go Modules

```bash
# Crawl modules from proxy.golang.org
./crawl -db wikigo.db

# Limit number of modules (for testing)
./crawl -db wikigo.db -max 100

# Crawl with more workers
./crawl -db wikigo.db -workers 8

# Incremental crawl since a specific time
./crawl -db wikigo.db -since 2024-01-01T00:00:00Z

# Daemon mode with periodic re-indexing
./crawl -db wikigo.db -daemon -interval 1h
```

### Crawl JavaScript/TypeScript Packages

```bash
# Index an NPM package
./crawljs -npm express -db wikigo.db

# Index a GitHub repository
./crawljs -github facebook/react -db wikigo.db -token YOUR_GITHUB_TOKEN

# Query indexed JS/TS packages
./queryjs -db wikigo.db
./queryjs -db wikigo.db -pkg express
```

### Crawl Rust Crates

```bash
# Index a Rust crate
./crawlrs -crate serde -db wikigo.db

# Query indexed Rust crates
./queryrs -db wikigo.db
./queryrs -db wikigo.db -crate serde
```

### AI Features Configuration

Set environment variables for AI features:

```bash
# Enable AI features with Mistral API
export MISTRAL_API_KEY="your-api-key"

# Run server
./serve -db wikigo.db
```

## Command Reference

### serve

| Flag | Default | Description |
|------|---------|-------------|
| `-dir` | `.` | Directory containing Go packages |
| `-addr` | `:8080` | Server address |
| `-db` | `` | SQLite database path for indexing |

### crawl (Go modules)

| Flag | Default | Description |
|------|---------|-------------|
| `-db` | `wikigo.db` | SQLite database path |
| `-workers` | `4` | Number of concurrent workers |
| `-rate` | `100ms` | Rate limit between requests |
| `-since` | `` | Only fetch modules updated since (RFC3339) |
| `-max` | `0` | Maximum modules to process (0 = unlimited) |
| `-temp` | `` | Temporary directory for downloads |
| `-daemon` | `false` | Run with periodic re-indexing |
| `-interval` | `1h` | Re-indexing interval in daemon mode |

### crawljs (JavaScript/TypeScript)

| Flag | Default | Description |
|------|---------|-------------|
| `-npm` | `` | NPM package name to index |
| `-github` | `` | GitHub repository (owner/repo) to index |
| `-token` | `$GITHUB_TOKEN` | GitHub API token |
| `-db` | `wikigo.db` | SQLite database path |

### crawlrs (Rust crates)

| Flag | Default | Description |
|------|---------|-------------|
| `-crate` | `` | Crate name to index |
| `-db` | `wikigo.db` | SQLite database path |

## API Routes

### Package Documentation

| Route | Description |
|-------|-------------|
| `/` | Home page / package list |
| `/{import-path}` | Package documentation |
| `/search?q=` | Search packages and symbols |
| `/symbols?q=` | Symbol search |
| `/versions/{path}` | Version history |
| `/diff/{path}?v1=&v2=` | API diff between versions |
| `/compare/?pkg1=&pkg2=` | Compare two packages |
| `/imports/{path}` | Package imports list |
| `/importedby/{path}` | Packages that import this one |
| `/license/{path}` | License full text |
| `/mod/{path}` | Module information (go.mod) |

### JSON API

| Route | Description |
|-------|-------------|
| `/api/{path}` | Package metadata as JSON |
| `/api/explain` | AI code explanation endpoint |

### Utilities

| Route | Description |
|-------|-------------|
| `/badge/{path}` | shields.io compatible badge |

## Database Schema

wikigo uses SQLite with the following tables:

### Go Packages
- `packages` - Package metadata and documentation
- `symbols` - Searchable symbols (functions, types, etc.)
- `imports` - Import relationships between packages
- `module_versions` - Version history for modules
- `crawl_metadata` - Crawler state (last crawl time)
- `packages_fts` / `symbols_fts` - Full-text search indexes

### JavaScript/TypeScript
- `js_packages` - NPM package and GitHub repo metadata
- `js_symbols` - Exported symbols (functions, classes, types)
- `js_packages_fts` / `js_symbols_fts` - Full-text search indexes

### Rust
- `rust_crates` - Crate metadata from crates.io
- `rust_symbols` - Public symbols (functions, structs, traits, etc.)
- `rust_crates_fts` / `rust_symbols_fts` - Full-text search indexes

### AI Features
- `ai_docs` - AI-generated documentation
- `ai_cache` - Cached AI responses
- `ai_usage` - Cost tracking and usage statistics

## Project Structure

```
wikigo/
├── cmd/
│   ├── serve/          # Documentation server
│   ├── crawl/          # Go module crawler
│   ├── crawljs/        # JavaScript/TypeScript crawler
│   ├── crawlrs/        # Rust crate crawler
│   ├── queryjs/        # Query JS/TS packages
│   ├── queryrs/        # Query Rust crates
│   ├── setup/          # Interactive setup script
│   └── gendocs/        # AI doc generation tool
├── crawler/
│   ├── crawler.go      # Go module crawler
│   ├── npm.go          # NPM package crawler
│   ├── github.go       # GitHub repository crawler
│   └── crates.go       # crates.io crawler
├── db/                 # Database layer with multi-language support
├── jsparser/           # JavaScript/TypeScript parser
├── rsparser/           # Rust parser
├── ai/                 # AI service integration
│   ├── service.go      # Mistral AI client
│   ├── docgen.go       # Doc generation
│   └── flags.go        # Feature flags
├── web/
│   ├── server.go       # HTTP handlers
│   ├── templates/      # HTML templates
│   └── static/         # CSS and JavaScript
├── util/               # Shared utilities
├── deployment/
│   ├── Caddyfile       # Caddy reverse proxy config
│   └── wikigo.service  # systemd service file
└── todo.txt            # Feature roadmap
```

## Deployment

### Using systemd

```bash
# Copy binary to system location
sudo cp serve /usr/local/bin/wikigo-serve

# Copy systemd service file
sudo cp deployment/wikigo.service /etc/systemd/system/

# Edit service file with your paths and user
sudo nano /etc/systemd/system/wikigo.service

# Enable and start service
sudo systemctl daemon-reload
sudo systemctl enable wikigo
sudo systemctl start wikigo
```

### Using Caddy

```bash
# Copy Caddyfile
sudo cp deployment/Caddyfile /etc/caddy/Caddyfile

# Edit with your domain
sudo nano /etc/caddy/Caddyfile

# Reload Caddy
sudo systemctl reload caddy
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `MISTRAL_API_KEY` | Mistral AI API key for AI features | `` |
| `GITHUB_TOKEN` | GitHub API token for higher rate limits | `` |
| `WIKIGO_DB_PATH` | Default database path | `wikigo.db` |
| `WIKIGO_ADDR` | Default server address | `:8080` |

## Development

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests for a specific package
go test ./crawler/...
go test ./jsparser/...
go test ./rsparser/...

# Run with verbose output
go test -v ./...
```

### Building from Source

```bash
# Build all commands
go build ./cmd/...

# Build specific command
go build -o wikigo-serve ./cmd/serve

# Build with optimizations
go build -ldflags="-s -w" ./cmd/serve
```

## Contributing

Contributions are welcome! Please see the [todo.txt](todo.txt) for planned features and improvements.

### Feature Roadmap

See [todo.txt](todo.txt) for a comprehensive list of:
- Completed features ✓
- In-progress work
- Planned enhancements
- AI-powered features
- Multi-language support improvements

## License

MIT License

## Acknowledgments

- Inspired by [pkg.go.dev](https://pkg.go.dev)
- Uses [esbuild](https://esbuild.github.io/) for JavaScript/TypeScript parsing
- Powered by [Mistral AI](https://mistral.ai/) for AI features
- Built with Go's excellent standard library
