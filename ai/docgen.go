package ai

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"
)

// UncommentedSymbol represents a symbol that lacks documentation
type UncommentedSymbol struct {
	Name      string
	Kind      string // "func", "type", "method"
	Signature string
	Body      string
	FilePath  string
	Line      int
}

// DocumentationAnalyzer finds uncommented exported symbols
type DocumentationAnalyzer struct {
	fset *token.FileSet
}

// NewDocumentationAnalyzer creates a new analyzer
func NewDocumentationAnalyzer(fset *token.FileSet) *DocumentationAnalyzer {
	return &DocumentationAnalyzer{
		fset: fset,
	}
}

// FindUncommentedFunctions finds all exported functions without documentation
func (da *DocumentationAnalyzer) FindUncommentedFunctions(pkgName string, files []*ast.File) []UncommentedSymbol {
	var uncommented []UncommentedSymbol

	for _, file := range files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch decl := n.(type) {
			case *ast.FuncDecl:
				// Only process exported functions
				if !ast.IsExported(decl.Name.Name) {
					return true
				}

				// Skip if already has documentation
				if decl.Doc != nil && len(decl.Doc.List) > 0 {
					return true
				}

				// Extract function signature and body
				signature := da.extractFunctionSignature(decl)
				body := da.extractFunctionBody(decl)

				position := da.fset.Position(decl.Pos())

				uncommented = append(uncommented, UncommentedSymbol{
					Name:      decl.Name.Name,
					Kind:      "func",
					Signature: signature,
					Body:      body,
					FilePath:  position.Filename,
					Line:      position.Line,
				})
			}
			return true
		})
	}

	return uncommented
}

// FindUncommentedTypes finds all exported types without documentation
func (da *DocumentationAnalyzer) FindUncommentedTypes(files []*ast.File) []UncommentedSymbol {
	var uncommented []UncommentedSymbol

	for _, file := range files {
		ast.Inspect(file, func(n ast.Node) bool {
			if genDecl, ok := n.(*ast.GenDecl); ok && genDecl.Tok == token.TYPE {
				for _, spec := range genDecl.Specs {
					typeSpec, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}

					// Only process exported types
					if !ast.IsExported(typeSpec.Name.Name) {
						continue
					}

					// Skip if already has documentation
					if typeSpec.Doc != nil && len(typeSpec.Doc.List) > 0 {
						continue
					}
					if genDecl.Doc != nil && len(genDecl.Doc.List) > 0 {
						continue
					}

					position := da.fset.Position(typeSpec.Pos())

					uncommented = append(uncommented, UncommentedSymbol{
						Name:      typeSpec.Name.Name,
						Kind:      "type",
						Signature: fmt.Sprintf("type %s", typeSpec.Name.Name),
						Body:      da.extractTypeDefinition(typeSpec),
						FilePath:  position.Filename,
						Line:      position.Line,
					})
				}
			}
			return true
		})
	}

	return uncommented
}

// FindUncommentedMethods finds all exported methods without documentation
func (da *DocumentationAnalyzer) FindUncommentedMethods(files []*ast.File) []UncommentedSymbol {
	var uncommented []UncommentedSymbol

	for _, file := range files {
		ast.Inspect(file, func(n ast.Node) bool {
			if funcDecl, ok := n.(*ast.FuncDecl); ok {
				// Only methods (have receiver)
				if funcDecl.Recv == nil {
					return true
				}

				// Only exported methods
				if !ast.IsExported(funcDecl.Name.Name) {
					return true
				}

				// Skip if already has documentation
				if funcDecl.Doc != nil && len(funcDecl.Doc.List) > 0 {
					return true
				}

				signature := da.extractFunctionSignature(funcDecl)
				body := da.extractFunctionBody(funcDecl)

				position := da.fset.Position(funcDecl.Pos())

				uncommented = append(uncommented, UncommentedSymbol{
					Name:      funcDecl.Name.Name,
					Kind:      "method",
					Signature: signature,
					Body:      body,
					FilePath:  position.Filename,
					Line:      position.Line,
				})
			}
			return true
		})
	}

	return uncommented
}

// FindUncommentedPackage checks if package lacks a doc comment
func (da *DocumentationAnalyzer) FindUncommentedPackage(files []*ast.File) bool {
	for _, file := range files {
		// Check for package-level doc comment
		if file.Doc != nil && len(file.Doc.List) > 0 {
			// Has package documentation
			return false
		}
	}
	// No package documentation found
	return true
}

// extractFunctionSignature extracts the function signature as a string
func (da *DocumentationAnalyzer) extractFunctionSignature(funcDecl *ast.FuncDecl) string {
	var buf strings.Builder

	buf.WriteString("func ")

	// Add receiver if it's a method
	if funcDecl.Recv != nil && len(funcDecl.Recv.List) > 0 {
		buf.WriteString("(")
		recv := funcDecl.Recv.List[0]
		if len(recv.Names) > 0 {
			buf.WriteString(recv.Names[0].Name)
			buf.WriteString(" ")
		}
		buf.WriteString(da.typeToString(recv.Type))
		buf.WriteString(") ")
	}

	buf.WriteString(funcDecl.Name.Name)
	buf.WriteString("(")

	// Parameters
	if funcDecl.Type.Params != nil {
		for i, param := range funcDecl.Type.Params.List {
			if i > 0 {
				buf.WriteString(", ")
			}
			if len(param.Names) > 0 {
				buf.WriteString(param.Names[0].Name)
				buf.WriteString(" ")
			}
			buf.WriteString(da.typeToString(param.Type))
		}
	}

	buf.WriteString(")")

	// Results
	if funcDecl.Type.Results != nil && len(funcDecl.Type.Results.List) > 0 {
		buf.WriteString(" ")
		if len(funcDecl.Type.Results.List) > 1 {
			buf.WriteString("(")
		}
		for i, result := range funcDecl.Type.Results.List {
			if i > 0 {
				buf.WriteString(", ")
			}
			buf.WriteString(da.typeToString(result.Type))
		}
		if len(funcDecl.Type.Results.List) > 1 {
			buf.WriteString(")")
		}
	}

	return buf.String()
}

// extractFunctionBody extracts a simplified view of the function body
func (da *DocumentationAnalyzer) extractFunctionBody(funcDecl *ast.FuncDecl) string {
	if funcDecl.Body == nil {
		return "{}"
	}

	// Limit body size to avoid huge prompts
	var buf strings.Builder
	buf.WriteString("{\n")

	count := 0
	maxStatements := 10 // Only show first 10 statements

	for _, stmt := range funcDecl.Body.List {
		if count >= maxStatements {
			buf.WriteString("  // ... more statements ...\n")
			break
		}

		// Very simplified statement representation
		buf.WriteString("  ")
		buf.WriteString(da.stmtToString(stmt))
		buf.WriteString("\n")
		count++
	}

	buf.WriteString("}")
	return buf.String()
}

// extractTypeDefinition extracts the type definition
func (da *DocumentationAnalyzer) extractTypeDefinition(typeSpec *ast.TypeSpec) string {
	return da.typeToString(typeSpec.Type)
}

// typeToString converts an ast.Expr type to a string
func (da *DocumentationAnalyzer) typeToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + da.typeToString(t.X)
	case *ast.ArrayType:
		return "[]" + da.typeToString(t.Elt)
	case *ast.MapType:
		return "map[" + da.typeToString(t.Key) + "]" + da.typeToString(t.Value)
	case *ast.SelectorExpr:
		return da.typeToString(t.X) + "." + t.Sel.Name
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.StructType:
		return "struct{...}"
	case *ast.FuncType:
		return "func(...)"
	case *ast.ChanType:
		return "chan " + da.typeToString(t.Value)
	default:
		return "unknown"
	}
}

// stmtToString converts a statement to a simplified string
func (da *DocumentationAnalyzer) stmtToString(stmt ast.Stmt) string {
	switch stmt.(type) {
	case *ast.ReturnStmt:
		return "return ..."
	case *ast.IfStmt:
		return "if ..."
	case *ast.ForStmt:
		return "for ..."
	case *ast.RangeStmt:
		return "for range ..."
	case *ast.SwitchStmt:
		return "switch ..."
	case *ast.AssignStmt:
		return "assignment"
	case *ast.ExprStmt:
		return "expression"
	case *ast.DeferStmt:
		return "defer ..."
	case *ast.GoStmt:
		return "go ..."
	default:
		return "statement"
	}
}
