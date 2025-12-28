package jsparser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseTypeScriptFile(t *testing.T) {
	// Create temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.ts")

	content := `
export function greet(name: string): string {
  return "Hello, " + name;
}

export const PI = 3.14;

export class Calculator {
  add(a: number, b: number): number {
    return a + b;
  }
}

export interface User {
  name: string;
  age: number;
}

export type ID = string | number;

export enum Color {
  Red,
  Green,
  Blue
}
`

	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	parser := NewParser()
	symbols, err := parser.ParseFile(testFile)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}

	expectedSymbols := map[string]string{
		"greet":      "function",
		"PI":         "const",
		"Calculator": "class",
		"User":       "interface",
		"ID":         "type",
		"Color":      "enum",
	}

	if len(symbols) < len(expectedSymbols) {
		t.Errorf("Expected at least %d symbols, got %d", len(expectedSymbols), len(symbols))
	}

	foundSymbols := make(map[string]string)
	for _, sym := range symbols {
		foundSymbols[sym.Name] = sym.Kind
		if !sym.Exported {
			t.Errorf("Symbol %s should be exported", sym.Name)
		}
	}

	for name, expectedKind := range expectedSymbols {
		if kind, ok := foundSymbols[name]; !ok {
			t.Errorf("Expected symbol %s not found", name)
		} else if kind != expectedKind {
			t.Errorf("Symbol %s: expected kind %s, got %s", name, expectedKind, kind)
		}
	}
}

func TestParseJavaScriptFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.js")

	content := `
export function hello() {
  console.log("Hello");
}

export const greet = (name) => {
  return "Hello, " + name;
};

export class Person {
  constructor(name) {
    this.name = name;
  }
}
`

	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	parser := NewParser()
	symbols, err := parser.ParseFile(testFile)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}

	if len(symbols) == 0 {
		t.Error("Expected symbols, got none")
	}

	for _, sym := range symbols {
		if sym.Name == "" {
			t.Error("Symbol should have a name")
		}
		if sym.FilePath != testFile {
			t.Errorf("Symbol FilePath = %s, want %s", sym.FilePath, testFile)
		}
	}
}

func TestParseGoFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")

	content := `package main

func Hello() string {
  return "Hello"
}

func private() string {
  return "private"
}
`

	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	parser := NewParser()
	symbols, err := parser.ParseGoFile(testFile)
	if err != nil {
		t.Fatalf("ParseGoFile() error = %v", err)
	}

	// Should only find exported Hello function
	if len(symbols) != 1 {
		t.Errorf("Expected 1 symbol, got %d", len(symbols))
	}

	if len(symbols) > 0 {
		if symbols[0].Name != "Hello" {
			t.Errorf("Expected symbol name Hello, got %s", symbols[0].Name)
		}
		if symbols[0].Kind != "function" {
			t.Errorf("Expected kind function, got %s", symbols[0].Kind)
		}
	}
}
