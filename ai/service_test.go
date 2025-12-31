package ai

import (
	"strings"
	"testing"
)

func TestValidateGeneratedContent(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		context    ValidationContext
		wantValid  bool
		minConfidence float64
	}{
		{
			name:       "valid content",
			content:    "ReadFile reads the named file and returns the contents.",
			context:    ValidationContext{},
			wantValid:  true,
			minConfidence: 0.9,
		},
		{
			name:       "empty content",
			content:    "",
			context:    ValidationContext{},
			wantValid:  false,
			minConfidence: 0,
		},
		{
			name:       "uncertainty phrase",
			content:    "I'm not sure what this function does.",
			context:    ValidationContext{},
			wantValid:  true,
			minConfidence: 0.5,
		},
		{
			name:       "AI self-reference",
			content:    "As an AI, I cannot determine this.",
			context:    ValidationContext{},
			wantValid:  false,
			minConfidence: 0,
		},
		{
			name:       "missing expected symbol",
			content:    "This function does something.",
			context:    ValidationContext{ExpectedSymbols: []string{"ReadFile"}},
			wantValid:  true,
			minConfidence: 0.8,
		},
		{
			name:       "valid Go code",
			content:    `fmt.Println("hello")`,
			context:    ValidationContext{IsGoCode: true, ValidImports: []string{"fmt"}},
			wantValid:  true,
			minConfidence: 0.9,
		},
		{
			name:       "unbalanced braces",
			content:    `func test() { fmt.Println("hello")`,
			context:    ValidationContext{IsGoCode: true},
			wantValid:  true,
			minConfidence: 0.8,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := ValidateGeneratedContent(tc.content, tc.context)
			if result.IsValid != tc.wantValid {
				t.Errorf("IsValid = %v, want %v", result.IsValid, tc.wantValid)
			}
			if result.Confidence < tc.minConfidence {
				t.Errorf("Confidence = %v, want >= %v", result.Confidence, tc.minConfidence)
			}
		})
	}
}

func TestCheckGoSyntax(t *testing.T) {
	tests := []struct {
		name      string
		code      string
		wantIssues int
	}{
		{
			name:      "valid code",
			code:      `func main() { fmt.Println("hello") }`,
			wantIssues: 0,
		},
		{
			name:      "unbalanced braces",
			code:      `func main() { fmt.Println("hello")`,
			wantIssues: 1,
		},
		{
			name:      "unbalanced parens",
			code:      `fmt.Println("hello"`,
			wantIssues: 1,
		},
		{
			name:      "double semicolon",
			code:      `x := 1;; y := 2`,
			wantIssues: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			issues := checkGoSyntax(tc.code)
			if len(issues) != tc.wantIssues {
				t.Errorf("got %d issues, want %d: %v", len(issues), tc.wantIssues, issues)
			}
		})
	}
}

func TestBuildPlaygroundCode(t *testing.T) {
	tests := []struct {
		name     string
		example  *GeneratedExample
		contains []string
	}{
		{
			name: "simple example",
			example: &GeneratedExample{
				FunctionName: "ReadFile",
				ImportPath:   "os",
				Imports:      `"os"`,
				Code:         `data, _ := os.ReadFile("test.txt")`,
			},
			contains: []string{
				"package main",
				`"os"`,
				"func main() {",
				"os.ReadFile",
			},
		},
		{
			name: "multiple imports",
			example: &GeneratedExample{
				FunctionName: "Println",
				ImportPath:   "fmt",
				Imports:      "\"fmt\"\n\"os\"",
				Code:         `fmt.Println("hello")`,
			},
			contains: []string{
				"package main",
				`"fmt"`,
				`"os"`,
				"func main() {",
				`fmt.Println("hello")`,
			},
		},
		{
			name: "no imports",
			example: &GeneratedExample{
				FunctionName: "Example",
				Code:         `x := 1 + 1`,
			},
			contains: []string{
				"package main",
				"func main() {",
				"x := 1 + 1",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.example.BuildPlaygroundCode()
			for _, s := range tc.contains {
				if !strings.Contains(result, s) {
					t.Errorf("expected result to contain %q, got:\n%s", s, result)
				}
			}
		})
	}
}
