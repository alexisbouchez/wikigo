package rsparser

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Symbol represents a Rust symbol
type Symbol struct {
	Name      string
	Kind      string // function, struct, enum, trait, const, static, type, module, macro
	Signature string
	Line      int
	Public    bool
	Doc       string
	FilePath  string
}

// Parser handles Rust file parsing
type Parser struct {
	// Regex patterns for Rust symbols
	pubFnRegex     *regexp.Regexp
	pubStructRegex *regexp.Regexp
	pubEnumRegex   *regexp.Regexp
	pubTraitRegex  *regexp.Regexp
	pubConstRegex  *regexp.Regexp
	pubStaticRegex *regexp.Regexp
	pubTypeRegex   *regexp.Regexp
	pubModRegex    *regexp.Regexp
	macroRegex     *regexp.Regexp
	docCommentRegex *regexp.Regexp
}

// NewParser creates a new Rust parser
func NewParser() *Parser {
	return &Parser{
		pubFnRegex:     regexp.MustCompile(`pub\s+(?:async\s+)?(?:unsafe\s+)?(?:extern\s+"[^"]*"\s+)?fn\s+(\w+)`),
		pubStructRegex: regexp.MustCompile(`pub\s+struct\s+(\w+)`),
		pubEnumRegex:   regexp.MustCompile(`pub\s+enum\s+(\w+)`),
		pubTraitRegex:  regexp.MustCompile(`pub\s+trait\s+(\w+)`),
		pubConstRegex:  regexp.MustCompile(`pub\s+const\s+(\w+)`),
		pubStaticRegex: regexp.MustCompile(`pub\s+static\s+(\w+)`),
		pubTypeRegex:   regexp.MustCompile(`pub\s+type\s+(\w+)`),
		pubModRegex:    regexp.MustCompile(`pub\s+mod\s+(\w+)`),
		macroRegex:     regexp.MustCompile(`(?:pub\s+)?macro_rules!\s+(\w+)`),
		docCommentRegex: regexp.MustCompile(`^\s*///(.*)$`),
	}
}

// ParseFile parses a Rust file and extracts public symbols
func (p *Parser) ParseFile(filePath string) ([]Symbol, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	return p.extractSymbols(string(content), filePath), nil
}

// extractSymbols performs symbol extraction from Rust source code
func (p *Parser) extractSymbols(content, filePath string) []Symbol {
	var symbols []Symbol
	lines := strings.Split(content, "\n")

	var docComment string

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Collect doc comments
		if match := p.docCommentRegex.FindStringSubmatch(line); match != nil {
			if docComment != "" {
				docComment += "\n"
			}
			docComment += strings.TrimSpace(match[1])
			continue
		}

		// Skip if it's a comment or empty line
		if strings.HasPrefix(trimmed, "//") || trimmed == "" {
			if !strings.HasPrefix(trimmed, "///") {
				docComment = "" // Reset doc comment if non-doc comment found
			}
			continue
		}

		// Public function
		if match := p.pubFnRegex.FindStringSubmatch(trimmed); match != nil {
			symbols = append(symbols, Symbol{
				Name:     match[1],
				Kind:     "function",
				Line:     i + 1,
				Public:   true,
				Doc:      docComment,
				FilePath: filePath,
			})
			docComment = ""
			continue
		}

		// Public struct
		if match := p.pubStructRegex.FindStringSubmatch(trimmed); match != nil {
			symbols = append(symbols, Symbol{
				Name:     match[1],
				Kind:     "struct",
				Line:     i + 1,
				Public:   true,
				Doc:      docComment,
				FilePath: filePath,
			})
			docComment = ""
			continue
		}

		// Public enum
		if match := p.pubEnumRegex.FindStringSubmatch(trimmed); match != nil {
			symbols = append(symbols, Symbol{
				Name:     match[1],
				Kind:     "enum",
				Line:     i + 1,
				Public:   true,
				Doc:      docComment,
				FilePath: filePath,
			})
			docComment = ""
			continue
		}

		// Public trait
		if match := p.pubTraitRegex.FindStringSubmatch(trimmed); match != nil {
			symbols = append(symbols, Symbol{
				Name:     match[1],
				Kind:     "trait",
				Line:     i + 1,
				Public:   true,
				Doc:      docComment,
				FilePath: filePath,
			})
			docComment = ""
			continue
		}

		// Public const
		if match := p.pubConstRegex.FindStringSubmatch(trimmed); match != nil {
			symbols = append(symbols, Symbol{
				Name:     match[1],
				Kind:     "const",
				Line:     i + 1,
				Public:   true,
				Doc:      docComment,
				FilePath: filePath,
			})
			docComment = ""
			continue
		}

		// Public static
		if match := p.pubStaticRegex.FindStringSubmatch(trimmed); match != nil {
			symbols = append(symbols, Symbol{
				Name:     match[1],
				Kind:     "static",
				Line:     i + 1,
				Public:   true,
				Doc:      docComment,
				FilePath: filePath,
			})
			docComment = ""
			continue
		}

		// Public type alias
		if match := p.pubTypeRegex.FindStringSubmatch(trimmed); match != nil {
			symbols = append(symbols, Symbol{
				Name:     match[1],
				Kind:     "type",
				Line:     i + 1,
				Public:   true,
				Doc:      docComment,
				FilePath: filePath,
			})
			docComment = ""
			continue
		}

		// Public module
		if match := p.pubModRegex.FindStringSubmatch(trimmed); match != nil {
			symbols = append(symbols, Symbol{
				Name:     match[1],
				Kind:     "module",
				Line:     i + 1,
				Public:   true,
				Doc:      docComment,
				FilePath: filePath,
			})
			docComment = ""
			continue
		}

		// Macro
		if match := p.macroRegex.FindStringSubmatch(trimmed); match != nil {
			isPublic := strings.Contains(trimmed, "pub ")
			symbols = append(symbols, Symbol{
				Name:     match[1],
				Kind:     "macro",
				Line:     i + 1,
				Public:   isPublic,
				Doc:      docComment,
				FilePath: filePath,
			})
			docComment = ""
			continue
		}

		// Reset doc comment if we encounter non-comment, non-empty line without match
		if !strings.HasPrefix(trimmed, "//") && trimmed != "" {
			docComment = ""
		}
	}

	return symbols
}

// ParseDirectory recursively parses all Rust files in a directory
func (p *Parser) ParseDirectory(dirPath string) ([]Symbol, error) {
	var allSymbols []Symbol

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories to ignore
		if info.IsDir() {
			name := info.Name()
			if name == "target" || name == ".git" || name == "tests" || name == "benches" || name == "examples" {
				return filepath.SkipDir
			}
			return nil
		}

		// Parse .rs files
		if filepath.Ext(path) == ".rs" {
			symbols, err := p.ParseFile(path)
			if err != nil {
				return fmt.Errorf("parsing %s: %w", path, err)
			}
			allSymbols = append(allSymbols, symbols...)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walking directory: %w", err)
	}

	return allSymbols, nil
}
