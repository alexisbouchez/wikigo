package web

import (
	"testing"
)

func TestCalculateRelevanceScore_ExactMatch(t *testing.T) {
	result := map[string]interface{}{
		"name":        "express",
		"import_path": "npm/express",
		"synopsis":    "Fast web framework",
	}

	score := calculateRelevanceScore("express", result)
	if score < 1000 {
		t.Errorf("exact name match should score >= 1000, got %f", score)
	}
}

func TestCalculateRelevanceScore_PrefixMatch(t *testing.T) {
	result := map[string]interface{}{
		"name":        "express-session",
		"import_path": "npm/express-session",
		"synopsis":    "Session middleware",
	}

	score := calculateRelevanceScore("express", result)
	if score < 500 {
		t.Errorf("prefix match should score >= 500, got %f", score)
	}
}

func TestCalculateRelevanceScore_ContainsMatch(t *testing.T) {
	result := map[string]interface{}{
		"name":        "body-parser-express",
		"import_path": "npm/body-parser-express",
		"synopsis":    "Body parser",
	}

	score := calculateRelevanceScore("express", result)
	if score < 200 {
		t.Errorf("contains match should score >= 200, got %f", score)
	}
}

func TestCalculateRelevanceScore_SynopsisOnly(t *testing.T) {
	result := map[string]interface{}{
		"name":        "fast-framework",
		"import_path": "npm/fast-framework",
		"synopsis":    "An express-like web framework",
	}

	score := calculateRelevanceScore("express", result)
	if score < 50 {
		t.Errorf("synopsis match should score >= 50, got %f", score)
	}
}

func TestCalculateRelevanceScore_NoMatch(t *testing.T) {
	result := map[string]interface{}{
		"name":        "react",
		"import_path": "npm/react",
		"synopsis":    "A JavaScript library",
	}

	score := calculateRelevanceScore("express", result)
	// Should have minimal score (only from short name bonus)
	if score > 25 {
		t.Errorf("no match should score < 25, got %f", score)
	}
}

func TestCalculateRelevanceScore_PopularityBoost(t *testing.T) {
	popularResult := map[string]interface{}{
		"name":        "lodash",
		"import_path": "npm/lodash",
		"synopsis":    "Utility library",
		"downloads":   1000000,
	}

	unpopularResult := map[string]interface{}{
		"name":        "lodash",
		"import_path": "npm/lodash",
		"synopsis":    "Utility library",
		"downloads":   10,
	}

	popularScore := calculateRelevanceScore("lodash", popularResult)
	unpopularScore := calculateRelevanceScore("lodash", unpopularResult)

	if popularScore <= unpopularScore {
		t.Errorf("popular package should score higher: %f <= %f", popularScore, unpopularScore)
	}
}

func TestSortByRelevance(t *testing.T) {
	results := []map[string]interface{}{
		{"name": "express-session", "import_path": "npm/express-session", "synopsis": "Session middleware"},
		{"name": "express", "import_path": "npm/express", "synopsis": "Web framework"},
		{"name": "my-express-app", "import_path": "npm/my-express-app", "synopsis": "App"},
	}

	sorted := sortByRelevance("express", results)

	// Exact match should be first
	if sorted[0]["name"] != "express" {
		t.Errorf("exact match should be first, got %s", sorted[0]["name"])
	}

	// Prefix match should be second
	if sorted[1]["name"] != "express-session" {
		t.Errorf("prefix match should be second, got %s", sorted[1]["name"])
	}
}

func TestSortByRelevance_Empty(t *testing.T) {
	results := []map[string]interface{}{}
	sorted := sortByRelevance("test", results)
	if len(sorted) != 0 {
		t.Error("empty results should return empty")
	}
}

func TestSortByRelevance_Single(t *testing.T) {
	results := []map[string]interface{}{
		{"name": "test", "import_path": "test", "synopsis": ""},
	}
	sorted := sortByRelevance("test", results)
	if len(sorted) != 1 {
		t.Error("single result should return single")
	}
}

func TestPopularityScore(t *testing.T) {
	tests := []struct {
		count    int
		minScore float64
		maxScore float64
	}{
		{0, 0, 0},
		{50, 1, 5},
		{500, 4, 10},
		{5000, 9, 15},
		{50000, 14, 20},
		{500000, 19, 25},
		{5000000, 24, 26},
	}

	for _, tt := range tests {
		score := popularityScore(tt.count)
		if score < tt.minScore || score > tt.maxScore {
			t.Errorf("popularityScore(%d) = %f, want between %f and %f",
				tt.count, score, tt.minScore, tt.maxScore)
		}
	}
}

func TestGetString(t *testing.T) {
	m := map[string]interface{}{
		"name":    "test",
		"number":  42,
		"nil_val": nil,
	}

	if getString(m, "name") != "test" {
		t.Error("should get string value")
	}

	if getString(m, "number") != "" {
		t.Error("non-string should return empty")
	}

	if getString(m, "missing") != "" {
		t.Error("missing key should return empty")
	}

	if getString(m, "nil_val") != "" {
		t.Error("nil value should return empty")
	}
}
