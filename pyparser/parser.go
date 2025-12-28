package pyparser

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Symbol represents a Python symbol
type Symbol struct {
	Name      string
	Kind      string // function, class, constant, variable
	Signature string
	Line      int
	Public    bool
	Doc       string
	FilePath  string
}

// Parser handles Python file parsing
type Parser struct {
	funcRegex      *regexp.Regexp
	asyncFuncRegex *regexp.Regexp
	classRegex     *regexp.Regexp
	constantRegex  *regexp.Regexp
	decoratorRegex *regexp.Regexp
}

// NewParser creates a new Python parser
func NewParser() *Parser {
	return &Parser{
		// def function_name(args) or def function_name(args) -> return_type:
		funcRegex:      regexp.MustCompile(`^def\s+(\w+)\s*\(([^)]*)\)(?:\s*->\s*([^:]+))?:`),
		asyncFuncRegex: regexp.MustCompile(`^async\s+def\s+(\w+)\s*\(([^)]*)\)(?:\s*->\s*([^:]+))?:`),
		// class ClassName or class ClassName(bases):
		classRegex:     regexp.MustCompile(`^class\s+(\w+)(?:\s*\(([^)]*)\))?:`),
		// CONSTANT_NAME = value (must be ALL_CAPS with underscores)
		constantRegex:  regexp.MustCompile(`^([A-Z][A-Z0-9_]*)\s*=`),
		// @decorator
		decoratorRegex: regexp.MustCompile(`^@(\w+)`),
	}
}

// ParseFile parses a Python file and extracts public symbols
func (p *Parser) ParseFile(filePath string) ([]Symbol, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	return p.extractSymbols(string(content), filePath), nil
}

// extractSymbols performs symbol extraction from Python source code
func (p *Parser) extractSymbols(content, filePath string) []Symbol {
	var symbols []Symbol
	lines := strings.Split(content, "\n")

	var docstring string
	var decorators []string
	inDocstring := false
	docstringDelim := ""

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Handle multi-line docstrings
		if inDocstring {
			if strings.Contains(line, docstringDelim) {
				inDocstring = false
				docstringDelim = ""
			}
			continue
		}

		// Check for docstring start (standalone, not after def/class)
		if strings.HasPrefix(trimmed, `"""`) || strings.HasPrefix(trimmed, `'''`) {
			delim := trimmed[:3]
			// Check if docstring ends on same line
			rest := trimmed[3:]
			if strings.Contains(rest, delim) {
				// Single-line docstring - extract content
				endIdx := strings.Index(rest, delim)
				docstring = strings.TrimSpace(rest[:endIdx])
			} else {
				inDocstring = true
				docstringDelim = delim
				docstring = strings.TrimSpace(rest)
			}
			continue
		}

		// Skip empty lines and comments
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			// Collect comments as potential doc
			comment := strings.TrimPrefix(trimmed, "#")
			comment = strings.TrimSpace(comment)
			if docstring != "" {
				docstring += "\n" + comment
			} else {
				docstring = comment
			}
			continue
		}

		// Collect decorators
		if match := p.decoratorRegex.FindStringSubmatch(trimmed); match != nil {
			decorators = append(decorators, "@"+match[1])
			continue
		}

		// Async function
		if match := p.asyncFuncRegex.FindStringSubmatch(trimmed); match != nil {
			name := match[1]
			args := match[2]
			returnType := ""
			if len(match) > 3 {
				returnType = strings.TrimSpace(match[3])
			}

			sig := "async def " + name + "(" + args + ")"
			if returnType != "" {
				sig += " -> " + returnType
			}

			symbols = append(symbols, Symbol{
				Name:      name,
				Kind:      "function",
				Signature: sig,
				Line:      i + 1,
				Public:    !strings.HasPrefix(name, "_"),
				Doc:       docstring,
				FilePath:  filePath,
			})
			docstring = ""
			decorators = nil
			continue
		}

		// Function
		if match := p.funcRegex.FindStringSubmatch(trimmed); match != nil {
			name := match[1]
			args := match[2]
			returnType := ""
			if len(match) > 3 {
				returnType = strings.TrimSpace(match[3])
			}

			sig := "def " + name + "(" + args + ")"
			if returnType != "" {
				sig += " -> " + returnType
			}

			symbols = append(symbols, Symbol{
				Name:      name,
				Kind:      "function",
				Signature: sig,
				Line:      i + 1,
				Public:    !strings.HasPrefix(name, "_"),
				Doc:       docstring,
				FilePath:  filePath,
			})
			docstring = ""
			decorators = nil
			continue
		}

		// Class
		if match := p.classRegex.FindStringSubmatch(trimmed); match != nil {
			name := match[1]
			bases := ""
			if len(match) > 2 {
				bases = strings.TrimSpace(match[2])
			}

			sig := "class " + name
			if bases != "" {
				sig += "(" + bases + ")"
			}

			symbols = append(symbols, Symbol{
				Name:      name,
				Kind:      "class",
				Signature: sig,
				Line:      i + 1,
				Public:    !strings.HasPrefix(name, "_"),
				Doc:       docstring,
				FilePath:  filePath,
			})
			docstring = ""
			decorators = nil
			continue
		}

		// Constant (module-level ALL_CAPS)
		if match := p.constantRegex.FindStringSubmatch(trimmed); match != nil {
			name := match[1]
			symbols = append(symbols, Symbol{
				Name:      name,
				Kind:      "constant",
				Signature: trimmed,
				Line:      i + 1,
				Public:    !strings.HasPrefix(name, "_"),
				Doc:       docstring,
				FilePath:  filePath,
			})
			docstring = ""
			decorators = nil
			continue
		}

		// Reset state for non-matching lines
		if !strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, "@") {
			docstring = ""
			decorators = nil
		}
	}

	return symbols
}

// ParseDirectory recursively parses all Python files in a directory
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
			case "__pycache__", ".git", "venv", "env", ".venv", ".env",
				"tests", "test", ".tox", "build", "dist", ".eggs",
				"node_modules", ".pytest_cache", ".mypy_cache":
				return filepath.SkipDir
			}
			// Skip egg-info directories
			if strings.HasSuffix(name, ".egg-info") {
				return filepath.SkipDir
			}
			return nil
		}

		// Parse .py files
		if filepath.Ext(path) == ".py" {
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
