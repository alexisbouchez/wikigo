package ai

import (
	"strings"
	"testing"
)

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
