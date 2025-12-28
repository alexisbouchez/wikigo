package phpparser

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Symbol represents a PHP symbol
type Symbol struct {
	Name      string
	Kind      string // function, class, interface, trait, constant
	Signature string
	Line      int
	Public    bool
	Doc       string
	FilePath  string
}

// Parser handles PHP file parsing
type Parser struct {
	funcRegex      *regexp.Regexp
	classRegex     *regexp.Regexp
	interfaceRegex *regexp.Regexp
	traitRegex     *regexp.Regexp
	constRegex     *regexp.Regexp
	docBlockRegex  *regexp.Regexp
}

// NewParser creates a new PHP parser
func NewParser() *Parser {
	return &Parser{
		// function name(params): returnType
		funcRegex:      regexp.MustCompile(`^(?:(?:public|protected|private|static|final|abstract)\s+)*function\s+(\w+)\s*\(([^)]*)\)(?:\s*:\s*([^\s{]+))?`),
		// class ClassName extends Parent implements Interface
		classRegex:     regexp.MustCompile(`^(?:(?:abstract|final)\s+)?class\s+(\w+)(?:\s+extends\s+(\w+))?(?:\s+implements\s+([^{]+))?`),
		// interface InterfaceName extends OtherInterface
		interfaceRegex: regexp.MustCompile(`^interface\s+(\w+)(?:\s+extends\s+([^{]+))?`),
		// trait TraitName
		traitRegex:     regexp.MustCompile(`^trait\s+(\w+)`),
		// const CONSTANT_NAME = value or define('CONSTANT', value)
		constRegex:     regexp.MustCompile(`^(?:(?:public|protected|private)\s+)?const\s+(\w+)\s*=`),
		// PHPDoc block /** ... */
		docBlockRegex:  regexp.MustCompile(`/\*\*\s*([\s\S]*?)\s*\*/`),
	}
}

// ParseFile parses a PHP file and extracts symbols
func (p *Parser) ParseFile(filePath string) ([]Symbol, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	return p.extractSymbols(string(content), filePath), nil
}

// extractSymbols performs symbol extraction from PHP source code
func (p *Parser) extractSymbols(content, filePath string) []Symbol {
	var symbols []Symbol
	lines := strings.Split(content, "\n")

	var docBlock string
	inDocBlock := false
	docBlockLines := []string{}

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Handle PHPDoc blocks
		if strings.HasPrefix(trimmed, "/**") {
			inDocBlock = true
			docBlockLines = []string{}
			// Check if single-line doc block
			if strings.Contains(trimmed, "*/") {
				inDocBlock = false
				docBlock = strings.TrimPrefix(trimmed, "/**")
				docBlock = strings.TrimSuffix(docBlock, "*/")
				docBlock = strings.TrimSpace(docBlock)
			}
			continue
		}
		if inDocBlock {
			if strings.Contains(trimmed, "*/") {
				inDocBlock = false
				docBlock = strings.Join(docBlockLines, "\n")
				docBlock = cleanDocBlock(docBlock)
			} else {
				// Remove leading * from doc lines
				docLine := strings.TrimPrefix(trimmed, "*")
				docLine = strings.TrimSpace(docLine)
				docBlockLines = append(docBlockLines, docLine)
			}
			continue
		}

		// Skip empty lines and regular comments
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Function
		if match := p.funcRegex.FindStringSubmatch(trimmed); match != nil {
			name := match[1]
			params := match[2]
			returnType := ""
			if len(match) > 3 {
				returnType = strings.TrimSpace(match[3])
			}

			// Determine visibility
			isPublic := !strings.Contains(trimmed, "private ") && !strings.Contains(trimmed, "protected ")

			sig := "function " + name + "(" + params + ")"
			if returnType != "" {
				sig += ": " + returnType
			}

			symbols = append(symbols, Symbol{
				Name:      name,
				Kind:      "function",
				Signature: sig,
				Line:      i + 1,
				Public:    isPublic,
				Doc:       docBlock,
				FilePath:  filePath,
			})
			docBlock = ""
			continue
		}

		// Class
		if match := p.classRegex.FindStringSubmatch(trimmed); match != nil {
			name := match[1]
			extends := ""
			implements := ""
			if len(match) > 2 {
				extends = strings.TrimSpace(match[2])
			}
			if len(match) > 3 {
				implements = strings.TrimSpace(match[3])
			}

			sig := "class " + name
			if extends != "" {
				sig += " extends " + extends
			}
			if implements != "" {
				sig += " implements " + implements
			}

			symbols = append(symbols, Symbol{
				Name:      name,
				Kind:      "class",
				Signature: sig,
				Line:      i + 1,
				Public:    true,
				Doc:       docBlock,
				FilePath:  filePath,
			})
			docBlock = ""
			continue
		}

		// Interface
		if match := p.interfaceRegex.FindStringSubmatch(trimmed); match != nil {
			name := match[1]
			extends := ""
			if len(match) > 2 {
				extends = strings.TrimSpace(match[2])
			}

			sig := "interface " + name
			if extends != "" {
				sig += " extends " + extends
			}

			symbols = append(symbols, Symbol{
				Name:      name,
				Kind:      "interface",
				Signature: sig,
				Line:      i + 1,
				Public:    true,
				Doc:       docBlock,
				FilePath:  filePath,
			})
			docBlock = ""
			continue
		}

		// Trait
		if match := p.traitRegex.FindStringSubmatch(trimmed); match != nil {
			name := match[1]

			symbols = append(symbols, Symbol{
				Name:      name,
				Kind:      "trait",
				Signature: "trait " + name,
				Line:      i + 1,
				Public:    true,
				Doc:       docBlock,
				FilePath:  filePath,
			})
			docBlock = ""
			continue
		}

		// Constant
		if match := p.constRegex.FindStringSubmatch(trimmed); match != nil {
			name := match[1]
			isPublic := !strings.Contains(trimmed, "private ") && !strings.Contains(trimmed, "protected ")

			symbols = append(symbols, Symbol{
				Name:      name,
				Kind:      "constant",
				Signature: trimmed,
				Line:      i + 1,
				Public:    isPublic,
				Doc:       docBlock,
				FilePath:  filePath,
			})
			docBlock = ""
			continue
		}

		// Reset doc block for non-matching lines
		if !strings.HasPrefix(trimmed, "*") {
			docBlock = ""
		}
	}

	return symbols
}

// cleanDocBlock cleans up a PHPDoc block
func cleanDocBlock(doc string) string {
	lines := strings.Split(doc, "\n")
	var cleaned []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Skip @param, @return, etc. annotations for now
		if strings.HasPrefix(line, "@") {
			continue
		}
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}
	return strings.Join(cleaned, " ")
}

// ParseDirectory recursively parses all PHP files in a directory
func (p *Parser) ParseDirectory(dirPath string) ([]Symbol, error) {
	var allSymbols []Symbol

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories to ignore
		if info.IsDir() {
			name := info.Name()
			switch name {
			case "vendor", ".git", "tests", "test", "Tests", "Test",
				"node_modules", "cache", ".cache", "var":
				return filepath.SkipDir
			}
			return nil
		}

		// Parse .php files
		if filepath.Ext(path) == ".php" {
			symbols, err := p.ParseFile(path)
			if err != nil {
				// Log warning but continue
				return nil
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
