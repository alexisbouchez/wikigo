package rsparser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseRustFile(t *testing.T) {
	// Create a temporary Rust file
	tmpDir := t.TempDir()
	rustFile := filepath.Join(tmpDir, "test.rs")

	rustCode := `/// A public function that adds two numbers
pub fn add(a: i32, b: i32) -> i32 {
    a + b
}

/// A public struct representing a point
pub struct Point {
    pub x: f64,
    pub y: f64,
}

/// A public enum for results
pub enum Result<T, E> {
    Ok(T),
    Err(E),
}

/// A public trait for displayable items
pub trait Display {
    fn fmt(&self) -> String;
}

/// A public constant
pub const MAX_SIZE: usize = 1024;

/// A public static variable
pub static VERSION: &str = "1.0.0";

/// A public type alias
pub type IntResult = Result<i32, String>;

/// A public module
pub mod utils;

/// A public macro
macro_rules! debug {
    ($($arg:tt)*) => {
        println!($($arg)*)
    };
}

// Private function (should not be extracted)
fn private_fn() {}
`

	err := os.WriteFile(rustFile, []byte(rustCode), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	parser := NewParser()
	symbols, err := parser.ParseFile(rustFile)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}

	// Count symbols by kind
	kindCount := make(map[string]int)
	publicCount := 0
	for _, sym := range symbols {
		kindCount[sym.Kind]++
		if sym.Public {
			publicCount++
		}
	}

	// Verify we found the expected symbols
	if kindCount["function"] != 1 {
		t.Errorf("Expected 1 function, got %d", kindCount["function"])
	}
	if kindCount["struct"] != 1 {
		t.Errorf("Expected 1 struct, got %d", kindCount["struct"])
	}
	if kindCount["enum"] != 1 {
		t.Errorf("Expected 1 enum, got %d", kindCount["enum"])
	}
	if kindCount["trait"] != 1 {
		t.Errorf("Expected 1 trait, got %d", kindCount["trait"])
	}
	if kindCount["const"] != 1 {
		t.Errorf("Expected 1 const, got %d", kindCount["const"])
	}
	if kindCount["static"] != 1 {
		t.Errorf("Expected 1 static, got %d", kindCount["static"])
	}
	if kindCount["type"] != 1 {
		t.Errorf("Expected 1 type, got %d", kindCount["type"])
	}
	if kindCount["module"] != 1 {
		t.Errorf("Expected 1 module, got %d", kindCount["module"])
	}
	if kindCount["macro"] != 1 {
		t.Errorf("Expected 1 macro, got %d", kindCount["macro"])
	}

	// All symbols should be public (except maybe the macro)
	if publicCount < 8 {
		t.Errorf("Expected at least 8 public symbols, got %d", publicCount)
	}

	// Verify doc comments were captured
	foundDocComment := false
	for _, sym := range symbols {
		if sym.Name == "add" && strings.Contains(sym.Doc, "adds two numbers") {
			foundDocComment = true
			break
		}
	}
	if !foundDocComment {
		t.Error("Doc comment not captured for 'add' function")
	}
}

func TestParseAsyncUnsafeFunction(t *testing.T) {
	tmpDir := t.TempDir()
	rustFile := filepath.Join(tmpDir, "test.rs")

	rustCode := `pub async fn fetch_data() -> Result<String, Error> {
    Ok("data".to_string())
}

pub unsafe fn raw_operation(ptr: *mut u8) {
    // unsafe code
}

pub async unsafe fn dangerous_async() {
    // async unsafe code
}
`

	err := os.WriteFile(rustFile, []byte(rustCode), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	parser := NewParser()
	symbols, err := parser.ParseFile(rustFile)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}

	if len(symbols) != 3 {
		t.Errorf("Expected 3 functions, got %d", len(symbols))
	}

	// Verify function names
	expectedNames := map[string]bool{
		"fetch_data":       true,
		"raw_operation":    true,
		"dangerous_async":  true,
	}

	for _, sym := range symbols {
		if !expectedNames[sym.Name] {
			t.Errorf("Unexpected function name: %s", sym.Name)
		}
	}
}

func TestParseDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a directory structure
	srcDir := filepath.Join(tmpDir, "src")
	err := os.MkdirAll(srcDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create src directory: %v", err)
	}

	// lib.rs
	libRs := filepath.Join(srcDir, "lib.rs")
	err = os.WriteFile(libRs, []byte(`pub fn lib_function() {}`), 0644)
	if err != nil {
		t.Fatalf("Failed to write lib.rs: %v", err)
	}

	// utils.rs
	utilsRs := filepath.Join(srcDir, "utils.rs")
	err = os.WriteFile(utilsRs, []byte(`pub struct Helper {}`), 0644)
	if err != nil {
		t.Fatalf("Failed to write utils.rs: %v", err)
	}

	parser := NewParser()
	symbols, err := parser.ParseDirectory(tmpDir)
	if err != nil {
		t.Fatalf("ParseDirectory() error = %v", err)
	}

	if len(symbols) != 2 {
		t.Errorf("Expected 2 symbols, got %d", len(symbols))
	}
}
