# wikigo

An open-source clone of pkg.go.dev - a documentation server for Go packages.

## Features

### Package Documentation
- Full package documentation with syntax highlighting
- Functions, types, methods, constants, and variables
- Collapsible sections and jump-to navigation
- Source file links with line numbers
- Cross-package type linking

### Search & Discovery
- Full-text search across packages and symbols
- Symbol search with type filtering (functions, types, methods)
- Search result highlighting
- Autocomplete suggestions

### Version Management
- Version history tracking with timestamps
- Tagged/stable/pre-release indicators
- API diff between versions
- Package comparison view

### Module Indexing
- Crawls modules from proxy.golang.org
- Stores documentation in SQLite with FTS4
- Tracks import relationships
- Periodic re-indexing with daemon mode

### UI Features
- Dark mode toggle
- Keyboard shortcuts (/ for search)
- Mobile-responsive design
- Copy buttons for code blocks
- Breadcrumb navigation

## Installation

```bash
# Clone the repository
git clone https://github.com/alexisbouchez/wikigo.git
cd wikigo

# Build the project
go build ./...
```

## Usage

### Serve Local Packages

```bash
# Serve documentation for packages in a directory
go run ./cmd/serve -dir /path/to/your/go/packages

# With database for search and indexing
go run ./cmd/serve -dir /path/to/packages -db wikigo.db

# Custom port
go run ./cmd/serve -dir /path/to/packages -addr :3000
```

### Crawl Public Modules

```bash
# Crawl modules from proxy.golang.org
go run ./cmd/crawl -db wikigo.db

# Limit number of modules (for testing)
go run ./cmd/crawl -db wikigo.db -max 100

# Crawl with more workers
go run ./cmd/crawl -db wikigo.db -workers 8

# Incremental crawl since a specific time
go run ./cmd/crawl -db wikigo.db -since 2024-01-01T00:00:00Z

# Daemon mode with periodic re-indexing
go run ./cmd/crawl -db wikigo.db -daemon -interval 1h
```

### Command Options

#### serve
| Flag | Default | Description |
|------|---------|-------------|
| `-dir` | `.` | Directory containing Go packages |
| `-addr` | `:8080` | Server address |
| `-db` | `` | SQLite database path for indexing |

#### crawl
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

## Routes

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
| `/api/{path}` | JSON API for package metadata |
| `/badge/{path}` | shields.io compatible badge |

## Database Schema

wikigo uses SQLite with the following tables:

- `packages` - Package metadata and documentation
- `symbols` - Searchable symbols (functions, types, etc.)
- `imports` - Import relationships between packages
- `module_versions` - Version history for modules
- `crawl_metadata` - Crawler state (last crawl time)
- `packages_fts` / `symbols_fts` - Full-text search indexes

## Project Structure

```
wikigo/
├── cmd/
│   ├── serve/       # Documentation server
│   └── crawl/       # Module crawler
├── crawler/         # Crawler implementation
├── db/              # Database layer
├── web/
│   ├── server.go    # HTTP handlers
│   ├── templates/   # HTML templates
│   └── static/      # CSS and JavaScript
└── todo.txt         # Feature checklist
```

## License

MIT License
