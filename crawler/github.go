package crawler

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alexisbouchez/wikigo/db"
	"github.com/alexisbouchez/wikigo/jsparser"
)

const (
	GitHubAPIURL = "https://api.github.com"
)

// GitHubRepository represents a GitHub repository
type GitHubRepository struct {
	Name            string    `json:"name"`
	FullName        string    `json:"full_name"`
	Description     string    `json:"description"`
	HTMLURL         string    `json:"html_url"`
	CloneURL        string    `json:"clone_url"`
	Stars           int       `json:"stargazers_count"`
	Forks           int       `json:"forks_count"`
	Watchers        int       `json:"watchers_count"`
	Language        string    `json:"language"`
	Topics          []string  `json:"topics"`
	License         *struct {
		Name string `json:"name"`
		Key  string `json:"key"`
	} `json:"license"`
	DefaultBranch   string    `json:"default_branch"`
	UpdatedAt       time.Time `json:"updated_at"`
	HasPackageJSON  bool      `json:"-"`
}

// GitHubCrawler fetches and indexes GitHub repositories
type GitHubCrawler struct {
	db        *db.DB
	client    *http.Client
	parser    *jsparser.Parser
	tempDir   string
	rateLimit time.Duration
	token     string // GitHub API token (optional, for higher rate limits)
}

// NewGitHubCrawler creates a new GitHub crawler
func NewGitHubCrawler(database *db.DB, token string) (*GitHubCrawler, error) {
	tempDir, err := os.MkdirTemp("", "github-crawler-*")
	if err != nil {
		return nil, fmt.Errorf("creating temp dir: %w", err)
	}

	return &GitHubCrawler{
		db:        database,
		client:    &http.Client{Timeout: 60 * time.Second},
		parser:    jsparser.NewParser(),
		tempDir:   tempDir,
		rateLimit: 1 * time.Second, // GitHub rate limiting (60 req/hour without auth, 5000 with)
		token:     token,
	}, nil
}

// Close cleans up resources
func (c *GitHubCrawler) Close() error {
	return os.RemoveAll(c.tempDir)
}

// makeRequest makes an authenticated GitHub API request
func (c *GitHubCrawler) makeRequest(url string) (*http.Response, error) {
	time.Sleep(c.rateLimit)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	if c.token != "" {
		req.Header.Set("Authorization", "token "+c.token)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	return c.client.Do(req)
}

// SearchRepositories searches GitHub for JavaScript/TypeScript repositories
func (c *GitHubCrawler) SearchRepositories(query string, limit int) ([]*GitHubRepository, error) {
	// Default query for JS/TS repositories
	searchQuery := fmt.Sprintf("%s language:javascript OR language:typescript", query)
	url := fmt.Sprintf("%s/search/repositories?q=%s&sort=stars&order=desc&per_page=%d",
		GitHubAPIURL, searchQuery, limit)

	resp, err := c.makeRequest(url)
	if err != nil {
		return nil, fmt.Errorf("searching repositories: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search failed: status %d", resp.StatusCode)
	}

	var result struct {
		Items []*GitHubRepository `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding results: %w", err)
	}

	return result.Items, nil
}

// FetchRepository fetches detailed repository information
func (c *GitHubCrawler) FetchRepository(owner, repo string) (*GitHubRepository, error) {
	url := fmt.Sprintf("%s/repos/%s/%s", GitHubAPIURL, owner, repo)

	resp, err := c.makeRequest(url)
	if err != nil {
		return nil, fmt.Errorf("fetching repository: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("repository not found: %s/%s", owner, repo)
	}

	var repository GitHubRepository
	if err := json.NewDecoder(resp.Body).Decode(&repository); err != nil {
		return nil, fmt.Errorf("decoding repository: %w", err)
	}

	// Check for package.json
	hasPackageJSON, err := c.hasFile(owner, repo, "package.json")
	if err != nil {
		log.Printf("Warning: failed to check for package.json: %v", err)
	}
	repository.HasPackageJSON = hasPackageJSON

	return &repository, nil
}

// hasFile checks if a file exists in the repository
func (c *GitHubCrawler) hasFile(owner, repo, path string) (bool, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/contents/%s", GitHubAPIURL, owner, repo, path)

	resp, err := c.makeRequest(url)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}

// DownloadRepository downloads repository as a ZIP archive
func (c *GitHubCrawler) DownloadRepository(repo *GitHubRepository) (string, error) {
	// Download ZIP archive
	parts := strings.Split(repo.FullName, "/")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid repository name: %s", repo.FullName)
	}
	owner, name := parts[0], parts[1]

	url := fmt.Sprintf("%s/repos/%s/%s/zipball/%s",
		GitHubAPIURL, owner, name, repo.DefaultBranch)

	resp, err := c.makeRequest(url)
	if err != nil {
		return "", fmt.Errorf("downloading repository: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed: status %d", resp.StatusCode)
	}

	// Read ZIP into memory
	zipData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading zip data: %w", err)
	}

	// Extract ZIP
	extractDir := filepath.Join(c.tempDir, repo.FullName)
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		return "", fmt.Errorf("creating extract dir: %w", err)
	}

	zipReader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return "", fmt.Errorf("reading zip: %w", err)
	}

	for _, file := range zipReader.File {
		if file.FileInfo().IsDir() {
			continue
		}

		// Skip hidden files and directories
		if strings.Contains(file.Name, "/.") {
			continue
		}

		// Extract file
		targetPath := filepath.Join(extractDir, file.Name)
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return "", fmt.Errorf("creating directories: %w", err)
		}

		outFile, err := os.Create(targetPath)
		if err != nil {
			return "", fmt.Errorf("creating file: %w", err)
		}

		rc, err := file.Open()
		if err != nil {
			outFile.Close()
			return "", fmt.Errorf("opening zip file: %w", err)
		}

		_, err = io.Copy(outFile, rc)
		rc.Close()
		outFile.Close()

		if err != nil {
			return "", fmt.Errorf("extracting file: %w", err)
		}
	}

	return extractDir, nil
}

// ParseRepositorySymbols parses JavaScript/TypeScript files in the repository
func (c *GitHubCrawler) ParseRepositorySymbols(repoDir string) ([]jsparser.Symbol, error) {
	var allSymbols []jsparser.Symbol

	err := filepath.Walk(repoDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories to ignore
		if info.IsDir() {
			name := info.Name()
			if name == "node_modules" || name == ".git" || name == "dist" ||
				name == "build" || name == "test" || name == "__tests__" {
				return filepath.SkipDir
			}
			return nil
		}

		// Parse JS/TS files
		ext := filepath.Ext(path)
		if ext == ".js" || ext == ".ts" || ext == ".jsx" || ext == ".tsx" {
			symbols, err := c.parser.ParseFile(path)
			if err != nil {
				log.Printf("Warning: failed to parse %s: %v", path, err)
				return nil
			}
			allSymbols = append(allSymbols, symbols...)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walking repository: %w", err)
	}

	return allSymbols, nil
}

// IndexRepository indexes a GitHub repository
func (c *GitHubCrawler) IndexRepository(owner, repo string) error {
	log.Printf("Indexing GitHub repository: %s/%s", owner, repo)

	// Fetch repository metadata
	repository, err := c.FetchRepository(owner, repo)
	if err != nil {
		return fmt.Errorf("fetching repository: %w", err)
	}

	// Download repository
	repoDir, err := c.DownloadRepository(repository)
	if err != nil {
		return fmt.Errorf("downloading repository: %w", err)
	}
	defer os.RemoveAll(repoDir)

	// Parse symbols
	symbols, err := c.ParseRepositorySymbols(repoDir)
	if err != nil {
		return fmt.Errorf("parsing symbols: %w", err)
	}

	log.Printf("Found %d symbols in %s/%s", len(symbols), owner, repo)

	// Store in database
	if c.db != nil {
		// Build package name from repo
		pkgName := repository.FullName

		// Determine license
		license := ""
		if repository.License != nil {
			license = repository.License.Name
		}

		dbPkg := &db.JSPackage{
			Name:          pkgName,
			Description:   repository.Description,
			License:       license,
			RepositoryURL: repository.CloneURL,
			Homepage:      repository.HTMLURL,
			GitHubURL:     repository.HTMLURL,
			Stars:         repository.Stars,
			Forks:         repository.Forks,
			Keywords:      repository.Topics,
		}

		pkgID, err := c.db.UpsertJSPackage(dbPkg)
		if err != nil {
			return fmt.Errorf("storing package: %w", err)
		}

		// Delete old symbols
		if err := c.db.DeleteJSPackageSymbols(pkgID); err != nil {
			return fmt.Errorf("deleting old symbols: %w", err)
		}

		// Store symbols
		exportedCount := 0
		for _, sym := range symbols {
			dbSym := &db.JSSymbol{
				Name:        sym.Name,
				Kind:        sym.Kind,
				Signature:   sym.Signature,
				PackageID:   pkgID,
				PackageName: pkgName,
				FilePath:    sym.FilePath,
				Line:        sym.Line,
				Exported:    sym.Exported,
				Doc:         sym.Doc,
			}

			if err := c.db.UpsertJSSymbol(dbSym); err != nil {
				log.Printf("Warning: failed to store symbol %s: %v", sym.Name, err)
			}

			if sym.Exported {
				exportedCount++
			}
		}

		log.Printf("Stored %d symbols (%d exported) in database", len(symbols), exportedCount)
	}

	return nil
}
