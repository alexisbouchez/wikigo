package web

import (
	"sort"
	"strings"
)

// SearchResult represents a search result with scoring
type SearchResult struct {
	Data  map[string]interface{}
	Score float64
}

// calculateRelevanceScore calculates a relevance score for a search result
func calculateRelevanceScore(query string, result map[string]interface{}) float64 {
	query = strings.ToLower(query)
	var score float64

	// Get fields
	name := strings.ToLower(getString(result, "name"))
	importPath := strings.ToLower(getString(result, "import_path"))
	synopsis := strings.ToLower(getString(result, "synopsis"))

	// Exact name match (highest priority)
	if name == query {
		score += 1000
	} else if name == strings.TrimPrefix(query, "@") { // Handle scoped packages
		score += 1000
	}

	// Name starts with query
	if strings.HasPrefix(name, query) {
		score += 500
	}

	// Name contains query
	if strings.Contains(name, query) {
		score += 200
	}

	// Import path exact match
	if importPath == query {
		score += 800
	}

	// Import path contains query
	if strings.Contains(importPath, query) {
		score += 100
	}

	// Synopsis contains query
	if strings.Contains(synopsis, query) {
		score += 50
	}

	// Shorter names are often more relevant
	if len(name) > 0 && len(name) < 20 {
		score += float64(20 - len(name))
	}

	// Popularity boost
	if downloads, ok := result["downloads"].(int); ok && downloads > 0 {
		score += popularityScore(downloads)
	}
	if stars, ok := result["stars"].(int); ok && stars > 0 {
		score += popularityScore(stars * 10) // Stars weighted higher
	}

	return score
}

// popularityScore converts raw popularity to a bounded score
func popularityScore(count int) float64 {
	if count <= 0 {
		return 0
	}
	// Log scale to prevent very popular packages from dominating
	// Max contribution is ~25 points for 1M+ downloads
	switch {
	case count >= 1000000:
		return 25
	case count >= 100000:
		return 20
	case count >= 10000:
		return 15
	case count >= 1000:
		return 10
	case count >= 100:
		return 5
	default:
		return 2
	}
}

// getString safely extracts a string from a map
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// sortByRelevance sorts search results by relevance score
func sortByRelevance(query string, results []map[string]interface{}) []map[string]interface{} {
	if len(results) <= 1 {
		return results
	}

	// Calculate scores
	scored := make([]SearchResult, len(results))
	for i, r := range results {
		scored[i] = SearchResult{
			Data:  r,
			Score: calculateRelevanceScore(query, r),
		}
	}

	// Sort by score (descending)
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})

	// Extract sorted results
	sorted := make([]map[string]interface{}, len(results))
	for i, s := range scored {
		sorted[i] = s.Data
	}

	return sorted
}
