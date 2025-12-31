package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleHome(t *testing.T) {
	s, err := NewServerWithDB(".", "")
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	defer s.Close()

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	s.handleHome(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Wikistral") {
		t.Error("expected home page to contain 'Wikistral'")
	}
}

func TestHandleSearch_Empty(t *testing.T) {
	s, err := NewServerWithDB(".", "")
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	defer s.Close()

	req := httptest.NewRequest("GET", "/search", nil)
	w := httptest.NewRecorder()

	s.handleSearch(w, req)

	// Empty search should redirect to home
	if w.Code != http.StatusFound {
		t.Errorf("expected redirect (302), got %d", w.Code)
	}
}

func TestHandleSearch_WithQuery(t *testing.T) {
	s, err := NewServerWithDB(".", "")
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	defer s.Close()

	req := httptest.NewRequest("GET", "/search?q=test", nil)
	w := httptest.NewRecorder()

	s.handleSearch(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandleAPI_ListPackages(t *testing.T) {
	s, err := NewServerWithDB(".", "")
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	defer s.Close()

	req := httptest.NewRequest("GET", "/api/", nil)
	w := httptest.NewRecorder()

	s.handleAPI(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", contentType)
	}

	var result []map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Errorf("failed to parse JSON response: %v", err)
	}
}

func TestHandleAPI_Search(t *testing.T) {
	s, err := NewServerWithDB(".", "")
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	defer s.Close()

	req := httptest.NewRequest("GET", "/api/search?q=test", nil)
	w := httptest.NewRecorder()

	s.handleAPI(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", contentType)
	}
}

func TestHandleAPI_SearchEmptyQuery(t *testing.T) {
	s, err := NewServerWithDB(".", "")
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	defer s.Close()

	req := httptest.NewRequest("GET", "/api/search?q=", nil)
	w := httptest.NewRecorder()

	s.handleAPI(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Errorf("failed to parse JSON response: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("expected empty results for empty query, got %d", len(result))
	}
}

func TestHandleAPI_PackageNotFound(t *testing.T) {
	s, err := NewServerWithDB(".", "")
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	defer s.Close()

	req := httptest.NewRequest("GET", "/api/nonexistent/package", nil)
	w := httptest.NewRecorder()

	s.handleAPI(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestHandleBadge_MissingPath(t *testing.T) {
	s, err := NewServerWithDB(".", "")
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	defer s.Close()

	req := httptest.NewRequest("GET", "/badge/", nil)
	w := httptest.NewRecorder()

	s.handleBadge(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandleBadge_UnknownPackage(t *testing.T) {
	s, err := NewServerWithDB(".", "")
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	defer s.Close()

	req := httptest.NewRequest("GET", "/badge/unknown/pkg", nil)
	w := httptest.NewRecorder()

	s.handleBadge(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var badge map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &badge); err != nil {
		t.Errorf("failed to parse badge JSON: %v", err)
	}

	if badge["message"] != "unknown" {
		t.Errorf("expected 'unknown' message for unknown package")
	}
}

func TestHandleLicense_NotFound(t *testing.T) {
	s, err := NewServerWithDB(".", "")
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	defer s.Close()

	req := httptest.NewRequest("GET", "/license/", nil)
	w := httptest.NewRecorder()

	s.handleLicense(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestHandleImports_NotFound(t *testing.T) {
	s, err := NewServerWithDB(".", "")
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	defer s.Close()

	req := httptest.NewRequest("GET", "/imports/", nil)
	w := httptest.NewRecorder()

	s.handleImports(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestHandleModule_NotFound(t *testing.T) {
	s, err := NewServerWithDB(".", "")
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	defer s.Close()

	req := httptest.NewRequest("GET", "/mod/", nil)
	w := httptest.NewRecorder()

	s.handleModule(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestHandleVersions_NotFound(t *testing.T) {
	s, err := NewServerWithDB(".", "")
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	defer s.Close()

	req := httptest.NewRequest("GET", "/versions/", nil)
	w := httptest.NewRecorder()

	s.handleVersions(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestHandleSymbolSearch(t *testing.T) {
	s, err := NewServerWithDB(".", "")
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	defer s.Close()

	req := httptest.NewRequest("GET", "/symbols?q=test", nil)
	w := httptest.NewRecorder()

	s.handleSymbolSearch(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandleSymbolSearch_WithKind(t *testing.T) {
	s, err := NewServerWithDB(".", "")
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	defer s.Close()

	req := httptest.NewRequest("GET", "/symbols?q=test&kind=func", nil)
	w := httptest.NewRecorder()

	s.handleSymbolSearch(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandleCompare(t *testing.T) {
	s, err := NewServerWithDB(".", "")
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	defer s.Close()

	req := httptest.NewRequest("GET", "/compare/", nil)
	w := httptest.NewRecorder()

	s.handleCompare(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandleExplain_MethodNotAllowed(t *testing.T) {
	s, err := NewServerWithDB(".", "")
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	defer s.Close()

	req := httptest.NewRequest("GET", "/api/explain", nil)
	w := httptest.NewRecorder()

	s.handleExplain(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestHandleExplain_EmptyCode(t *testing.T) {
	s, err := NewServerWithDB(".", "")
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	defer s.Close()

	body := strings.NewReader(`{"code": ""}`)
	req := httptest.NewRequest("POST", "/api/explain", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.handleExplain(w, req)

	// Empty code should return 400 (if AI is enabled) or 503 (if not)
	if w.Code != http.StatusBadRequest && w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 400 or 503, got %d", w.Code)
	}
}

func TestHandleRustCrate_Redirect(t *testing.T) {
	s, err := NewServerWithDB(".", "")
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	defer s.Close()

	req := httptest.NewRequest("GET", "/crates.io/", nil)
	w := httptest.NewRecorder()

	s.handleRustCrate(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected redirect (302), got %d", w.Code)
	}
}

func TestHandleJSPackage_Redirect(t *testing.T) {
	s, err := NewServerWithDB(".", "")
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	defer s.Close()

	req := httptest.NewRequest("GET", "/npm/", nil)
	w := httptest.NewRecorder()

	s.handleJSPackage(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected redirect (302), got %d", w.Code)
	}
}

func TestHandlePythonPackage_Redirect(t *testing.T) {
	s, err := NewServerWithDB(".", "")
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	defer s.Close()

	req := httptest.NewRequest("GET", "/pypi/", nil)
	w := httptest.NewRecorder()

	s.handlePythonPackage(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected redirect (302), got %d", w.Code)
	}
}

func TestHandlePHPPackage_Redirect(t *testing.T) {
	s, err := NewServerWithDB(".", "")
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	defer s.Close()

	req := httptest.NewRequest("GET", "/packagist/", nil)
	w := httptest.NewRecorder()

	s.handlePHPPackage(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected redirect (302), got %d", w.Code)
	}
}

func TestHandleImportedBy_NotFound(t *testing.T) {
	s, err := NewServerWithDB(".", "")
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	defer s.Close()

	req := httptest.NewRequest("GET", "/importedby/", nil)
	w := httptest.NewRecorder()

	s.handleImportedBy(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestHandleDiff_NotFound(t *testing.T) {
	s, err := NewServerWithDB(".", "")
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	defer s.Close()

	req := httptest.NewRequest("GET", "/diff/", nil)
	w := httptest.NewRecorder()

	s.handleDiff(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

// Test helper functions
func TestShortDoc(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello world. More text here.", "Hello world."},
		{"Single line without period", "Single line without period"},
		{"First line\nSecond line", "First line"},
		{"", ""},
		{"  Trimmed  ", "Trimmed"},
	}

	for _, tt := range tests {
		result := shortDoc(tt.input)
		if result != tt.expected {
			t.Errorf("shortDoc(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestAnchorName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello World", "Hello-World"},
		{"NoSpaces", "NoSpaces"},
		{"Multiple  Spaces", "Multiple--Spaces"},
	}

	for _, tt := range tests {
		result := anchorName(tt.input)
		if result != tt.expected {
			t.Errorf("anchorName(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestFormatDoc(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"  trimmed  ", "trimmed"},
		{"\n\ntext\n\n", "text"},
		{"", ""},
	}

	for _, tt := range tests {
		result := formatDoc(tt.input)
		if result != tt.expected {
			t.Errorf("formatDoc(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}
