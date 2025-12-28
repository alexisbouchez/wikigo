package jsparser

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	esbuild "github.com/evanw/esbuild/pkg/api"
)

// Symbol represents a JavaScript/TypeScript symbol
type Symbol struct {
	Name      string
	Kind      string // function, class, method, interface, type, enum, const, var
	Signature string
	Line      int
	Exported  bool
	Doc       string
	FilePath  string
}

// Parser handles JavaScript/TypeScript file parsing
type Parser struct {
	fset *token.FileSet
}

// NewParser creates a new JavaScript/TypeScript parser
func NewParser() *Parser {
	return &Parser{
		fset: token.NewFileSet(),
	}
}

// ParseFile parses a JavaScript or TypeScript file and extracts symbols
func (p *Parser) ParseFile(filePath string) ([]Symbol, error) {
	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	// Use esbuild to transform TypeScript to get metadata
	result := esbuild.Transform(string(content), esbuild.TransformOptions{
		Loader: p.getLoader(filePath),
		Target: esbuild.ES2020,
	})

	if len(result.Errors) > 0 {
		return nil, fmt.Errorf("parsing file: %v", result.Errors[0].Text)
	}

	// For now, use simple regex-based extraction
	// TODO: Implement proper AST-based extraction
	symbols := p.extractSymbols(string(content), filePath)

	return symbols, nil
}

// getLoader determines the appropriate esbuild loader based on file extension
func (p *Parser) getLoader(filePath string) esbuild.Loader {
	ext := filepath.Ext(filePath)
	switch ext {
	case ".ts":
		return esbuild.LoaderTS
	case ".tsx":
		return esbuild.LoaderTSX
	case ".jsx":
		return esbuild.LoaderJSX
	default:
		return esbuild.LoaderJS
	}
}

// extractSymbols performs basic symbol extraction from source code
func (p *Parser) extractSymbols(content, filePath string) []Symbol {
	var symbols []Symbol
	lines := strings.Split(content, "\n")

	for i, line := range lines {
		line = strings.TrimSpace(line)

		// ES6 Export function
		if strings.HasPrefix(line, "export function ") || strings.HasPrefix(line, "export async function ") {
			name := p.extractFunctionName(line)
			if name != "" {
				symbols = append(symbols, Symbol{
					Name:     name,
					Kind:     "function",
					Line:     i + 1,
					Exported: true,
					FilePath: filePath,
				})
			}
		}

		// ES6 Export const/let arrow functions
		if strings.HasPrefix(line, "export const ") || strings.HasPrefix(line, "export let ") {
			if strings.Contains(line, "=>") || strings.Contains(line, "= function") {
				name := p.extractConstName(line)
				if name != "" {
					symbols = append(symbols, Symbol{
						Name:     name,
						Kind:     "function",
						Line:     i + 1,
						Exported: true,
						FilePath: filePath,
					})
				}
			} else {
				name := p.extractConstName(line)
				if name != "" {
					symbols = append(symbols, Symbol{
						Name:     name,
						Kind:     "const",
						Line:     i + 1,
						Exported: true,
						FilePath: filePath,
					})
				}
			}
		}

		// ES6 Export class
		if strings.HasPrefix(line, "export class ") || strings.HasPrefix(line, "export abstract class ") {
			name := p.extractClassName(line)
			if name != "" {
				symbols = append(symbols, Symbol{
					Name:     name,
					Kind:     "class",
					Line:     i + 1,
					Exported: true,
					FilePath: filePath,
				})
			}
		}

		// ES6 Export interface
		if strings.HasPrefix(line, "export interface ") {
			name := p.extractInterfaceName(line)
			if name != "" {
				symbols = append(symbols, Symbol{
					Name:     name,
					Kind:     "interface",
					Line:     i + 1,
					Exported: true,
					FilePath: filePath,
				})
			}
		}

		// ES6 Export type
		if strings.HasPrefix(line, "export type ") {
			name := p.extractTypeName(line)
			if name != "" {
				symbols = append(symbols, Symbol{
					Name:     name,
					Kind:     "type",
					Line:     i + 1,
					Exported: true,
					FilePath: filePath,
				})
			}
		}

		// ES6 Export enum
		if strings.HasPrefix(line, "export enum ") {
			name := p.extractEnumName(line)
			if name != "" {
				symbols = append(symbols, Symbol{
					Name:     name,
					Kind:     "enum",
					Line:     i + 1,
					Exported: true,
					FilePath: filePath,
				})
			}
		}

		// CommonJS: module.exports = function(...)
		if strings.Contains(line, "module.exports") && strings.Contains(line, "function") {
			// Find the function keyword
			idx := strings.Index(line, "function")
			if idx != -1 {
				// Skip past "function" (8 chars)
				afterFunc := line[idx+8:]
				afterFunc = strings.TrimSpace(afterFunc)

				// Check if it's a named or anonymous function
				if strings.HasPrefix(afterFunc, "(") {
					// Anonymous function - use file name
					baseName := filepath.Base(filePath)
					baseName = strings.TrimSuffix(baseName, filepath.Ext(baseName))
					symbols = append(symbols, Symbol{
						Name:     baseName,
						Kind:     "function",
						Line:     i + 1,
						Exported: true,
						FilePath: filePath,
					})
				} else {
					// Named function
					if parenIdx := strings.Index(afterFunc, "("); parenIdx != -1 {
						name := strings.TrimSpace(afterFunc[:parenIdx])
						if name != "" {
							symbols = append(symbols, Symbol{
								Name:     name,
								Kind:     "function",
								Line:     i + 1,
								Exported: true,
								FilePath: filePath,
							})
						}
					}
				}
			}
		}

		// CommonJS: module.exports.name = ... or exports.name = ...
		if (strings.Contains(line, "module.exports.") || strings.Contains(line, "exports.")) && strings.Contains(line, "=") {
			var name string
			if strings.Contains(line, "module.exports.") {
				parts := strings.Split(line, "module.exports.")
				if len(parts) > 1 {
					name = strings.Split(parts[1], "=")[0]
				}
			} else if strings.Contains(line, "exports.") {
				parts := strings.Split(line, "exports.")
				if len(parts) > 1 {
					name = strings.Split(parts[1], "=")[0]
				}
			}

			name = strings.TrimSpace(name)
			if name != "" {
				kind := "const"
				if strings.Contains(line, "function") || strings.Contains(line, "=>") {
					kind = "function"
				} else if strings.Contains(line, "class ") {
					kind = "class"
				}
				symbols = append(symbols, Symbol{
					Name:     name,
					Kind:     kind,
					Line:     i + 1,
					Exported: true,
					FilePath: filePath,
				})
			}
		}
	}

	return symbols
}

func (p *Parser) extractFunctionName(line string) string {
	line = strings.TrimPrefix(line, "export ")
	line = strings.TrimPrefix(line, "async ")
	line = strings.TrimPrefix(line, "function ")
	parts := strings.Split(line, "(")
	if len(parts) > 0 {
		return strings.TrimSpace(parts[0])
	}
	return ""
}

func (p *Parser) extractConstName(line string) string {
	line = strings.TrimPrefix(line, "export ")
	line = strings.TrimPrefix(line, "const ")
	line = strings.TrimPrefix(line, "let ")
	parts := strings.Split(line, "=")
	if len(parts) > 0 {
		name := strings.TrimSpace(parts[0])
		// Remove type annotation if present
		if idx := strings.Index(name, ":"); idx != -1 {
			name = name[:idx]
		}
		return strings.TrimSpace(name)
	}
	return ""
}

func (p *Parser) extractClassName(line string) string {
	line = strings.TrimPrefix(line, "export ")
	line = strings.TrimPrefix(line, "abstract ")
	line = strings.TrimPrefix(line, "class ")
	parts := strings.FieldsFunc(line, func(r rune) bool {
		return r == ' ' || r == '{' || r == '<' || r == '('
	})
	if len(parts) > 0 {
		return strings.TrimSpace(parts[0])
	}
	return ""
}

func (p *Parser) extractInterfaceName(line string) string {
	line = strings.TrimPrefix(line, "export ")
	line = strings.TrimPrefix(line, "interface ")
	parts := strings.FieldsFunc(line, func(r rune) bool {
		return r == ' ' || r == '{' || r == '<'
	})
	if len(parts) > 0 {
		return strings.TrimSpace(parts[0])
	}
	return ""
}

func (p *Parser) extractTypeName(line string) string {
	line = strings.TrimPrefix(line, "export ")
	line = strings.TrimPrefix(line, "type ")
	parts := strings.FieldsFunc(line, func(r rune) bool {
		return r == ' ' || r == '=' || r == '<'
	})
	if len(parts) > 0 {
		return strings.TrimSpace(parts[0])
	}
	return ""
}

func (p *Parser) extractEnumName(line string) string {
	line = strings.TrimPrefix(line, "export ")
	line = strings.TrimPrefix(line, "enum ")
	parts := strings.FieldsFunc(line, func(r rune) bool {
		return r == ' ' || r == '{'
	})
	if len(parts) > 0 {
		return strings.TrimSpace(parts[0])
	}
	return ""
}

// ParseGoFile is included for compatibility but delegates to go/parser
func (p *Parser) ParseGoFile(filePath string) ([]Symbol, error) {
	f, err := parser.ParseFile(p.fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parsing go file: %w", err)
	}

	var symbols []Symbol

	ast.Inspect(f, func(n ast.Node) bool {
		switch decl := n.(type) {
		case *ast.FuncDecl:
			if ast.IsExported(decl.Name.Name) {
				pos := p.fset.Position(decl.Pos())
				symbols = append(symbols, Symbol{
					Name:     decl.Name.Name,
					Kind:     "function",
					Line:     pos.Line,
					Exported: true,
					FilePath: filePath,
				})
			}
		}
		return true
	})

	return symbols, nil
}
