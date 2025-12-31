package ai

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"time"
)

// Service provides AI-powered features with caching, rate limiting, and feature flags
type Service struct {
	client *Client
	cache  *Cache
	flags  *FeatureFlags
	budget *Budget
}

// Budget tracks and enforces spending limits
type Budget struct {
	MaxDailyUSD   float64
	MaxMonthlyUSD float64
	CurrentDayUSD float64
	CurrentMonthUSD float64
	DayResetTime  time.Time
	MonthResetTime time.Time
}

// NewBudget creates a new budget tracker
func NewBudget(maxDailyUSD, maxMonthlyUSD float64) *Budget {
	now := time.Now()
	return &Budget{
		MaxDailyUSD:     maxDailyUSD,
		MaxMonthlyUSD:   maxMonthlyUSD,
		DayResetTime:    now.AddDate(0, 0, 1),
		MonthResetTime:  now.AddDate(0, 1, 0),
	}
}

// CanSpend checks if a request would exceed budget limits
func (b *Budget) CanSpend(estimatedCostUSD float64) bool {
	// Reset daily budget if needed
	if time.Now().After(b.DayResetTime) {
		b.CurrentDayUSD = 0
		b.DayResetTime = time.Now().AddDate(0, 0, 1)
	}

	// Reset monthly budget if needed
	if time.Now().After(b.MonthResetTime) {
		b.CurrentMonthUSD = 0
		b.MonthResetTime = time.Now().AddDate(0, 1, 0)
	}

	// Check if we would exceed limits
	if b.CurrentDayUSD+estimatedCostUSD > b.MaxDailyUSD {
		return false
	}
	if b.CurrentMonthUSD+estimatedCostUSD > b.MaxMonthlyUSD {
		return false
	}

	return true
}

// RecordSpend records actual spending
func (b *Budget) RecordSpend(actualCostUSD float64) {
	b.CurrentDayUSD += actualCostUSD
	b.CurrentMonthUSD += actualCostUSD
}

// NewService creates a new AI service
func NewService(apiKey string, requestsPerMinute int, cacheTTL time.Duration) *Service {
	// If no API key, return service with disabled features
	if apiKey == "" {
		log.Println("Warning: No Mistral API key provided, AI features disabled")
		return &Service{
			client: nil,
			cache:  NewCache(cacheTTL),
			flags:  NewFeatureFlags(),
			budget: NewBudget(0, 0),
		}
	}

	return &Service{
		client: NewClient(apiKey, requestsPerMinute),
		cache:  NewCache(cacheTTL),
		flags:  NewFeatureFlags(),
		budget: NewBudget(1.0, 30.0), // Default: $1/day, $30/month
	}
}

// NewServiceFromEnv creates a service from environment variables
func NewServiceFromEnv() *Service {
	apiKey := os.Getenv("MISTRAL_API_KEY")
	return NewService(apiKey, 10, 24*time.Hour)
}

// IsEnabled checks if a feature is enabled
func (s *Service) IsEnabled(flag FeatureFlag) bool {
	if s.client == nil {
		return false
	}
	return s.flags.IsEnabled(flag)
}

// Enable enables a feature flag
func (s *Service) Enable(flag FeatureFlag) {
	if s.flags != nil {
		s.flags.Enable(flag)
	}
}

// GenerateWithCache generates text using cache when possible
func (s *Service) GenerateWithCache(flag FeatureFlag, systemPrompt, userPrompt string, maxTokens int) (string, error) {
	// Check if feature is enabled
	if !s.IsEnabled(flag) {
		return "", fmt.Errorf("feature %s is not enabled", flag)
	}

	// Check cache first
	cacheKey := systemPrompt + "|" + userPrompt
	if cached, found := s.cache.Get(cacheKey); found {
		return cached, nil
	}

	// Estimate cost (rough approximation)
	estimatedTokens := len(systemPrompt+userPrompt)/4 + maxTokens
	estimatedCost := float64(estimatedTokens) / 1000.0 * 0.002

	// Check budget
	if !s.budget.CanSpend(estimatedCost) {
		return "", fmt.Errorf("budget limit exceeded (daily: $%.2f/%.2f, monthly: $%.2f/%.2f)",
			s.budget.CurrentDayUSD, s.budget.MaxDailyUSD,
			s.budget.CurrentMonthUSD, s.budget.MaxMonthlyUSD)
	}

	// Generate
	content, err := s.client.GenerateText(systemPrompt, userPrompt, maxTokens)
	if err != nil {
		return "", err
	}

	// Cache the result
	s.cache.Set(cacheKey, content, estimatedCost, estimatedTokens)

	// Record spending
	s.budget.RecordSpend(estimatedCost)

	// Log for debugging
	log.Printf("AI: Generated %d tokens for feature %s (cache miss)", estimatedTokens, flag)

	return content, nil
}

// GenerateFunctionComment generates a comment for an uncommented function
func (s *Service) GenerateFunctionComment(functionSignature, functionBody string) (string, error) {
	systemPrompt := "You are a Go documentation expert. Generate concise, accurate doc comments for Go functions. Follow Go conventions: start with the function name, be brief, explain what it does (not how)."

	userPrompt := fmt.Sprintf(`Generate a doc comment for this Go function:

%s
%s

Return ONLY the comment text, without code fences or additional explanation.`, functionSignature, functionBody)

	return s.GenerateWithCache(FlagAutoComments, systemPrompt, userPrompt, 150)
}

// GeneratePackageSynopsis generates a synopsis for a package
func (s *Service) GeneratePackageSynopsis(packageName string, exportedSymbols []string) (string, error) {
	systemPrompt := "You are a Go documentation expert. Generate concise package synopses (one line, under 80 characters) that describe what the package does."

	userPrompt := fmt.Sprintf(`Generate a one-line synopsis for Go package "%s" with these exported symbols: %v

Return ONLY the synopsis text, under 80 characters.`, packageName, exportedSymbols)

	return s.GenerateWithCache(FlagAutoSynopsis, systemPrompt, userPrompt, 50)
}

// ExplainCode explains what a piece of code does
func (s *Service) ExplainCode(code string) (string, error) {
	systemPrompt := "You are a Go expert. Explain code clearly and concisely for developers who want to understand what it does."

	userPrompt := fmt.Sprintf(`Explain what this Go code does:

%s

Provide a clear, concise explanation in 2-3 sentences.`, code)

	return s.GenerateWithCache(FlagExplainCode, systemPrompt, userPrompt, 200)
}

// SummarizeLicense generates a plain-English summary of a license
func (s *Service) SummarizeLicense(licenseText string) (string, error) {
	systemPrompt := "You are a legal expert. Summarize software licenses in plain English, highlighting key permissions and restrictions."

	userPrompt := fmt.Sprintf(`Summarize this software license in plain English (3-4 bullet points):

%s

Focus on: what you CAN do, what you MUST do, and what you CANNOT do.`, licenseText)

	return s.GenerateWithCache(FlagLicenseSummary, systemPrompt, userPrompt, 250)
}

// GetStats returns combined statistics
func (s *Service) GetStats() map[string]interface{} {
	stats := make(map[string]interface{})

	if s.client != nil {
		clientStats := s.client.GetStats()
		stats["total_requests"] = clientStats.TotalRequests
		stats["failed_requests"] = clientStats.FailedRequests
		stats["total_tokens"] = clientStats.TotalTokens
		stats["total_cost_usd"] = clientStats.TotalCostUSD
	}

	cacheStats := s.cache.GetStats()
	stats["cache_hits"] = cacheStats.Hits
	stats["cache_misses"] = cacheStats.Misses
	stats["cache_hit_rate"] = s.cache.HitRate()
	stats["cache_savings_usd"] = cacheStats.TotalSavings
	stats["cache_size"] = cacheStats.CurrentSize

	stats["enabled_features"] = s.flags.GetEnabled()

	stats["budget_daily_used"] = s.budget.CurrentDayUSD
	stats["budget_daily_max"] = s.budget.MaxDailyUSD
	stats["budget_monthly_used"] = s.budget.CurrentMonthUSD
	stats["budget_monthly_max"] = s.budget.MaxMonthlyUSD

	return stats
}

// SetBudget updates budget limits
func (s *Service) SetBudget(maxDailyUSD, maxMonthlyUSD float64) {
	s.budget.MaxDailyUSD = maxDailyUSD
	s.budget.MaxMonthlyUSD = maxMonthlyUSD
}

// GenerateMethodComment generates a comment for an uncommented method
func (s *Service) GenerateMethodComment(receiverType, methodName, signature, body string) (string, error) {
	systemPrompt := "You are a Go documentation expert. Generate concise, accurate doc comments for Go methods. Follow Go conventions: start with the method name, be brief, explain what it does (not how)."

	userPrompt := fmt.Sprintf(`Generate a doc comment for this Go method:

Receiver: %s
%s
%s

Return ONLY the comment text, without code fences or additional explanation.`, receiverType, signature, body)

	return s.GenerateWithCache(FlagAutoComments, systemPrompt, userPrompt, 150)
}

// GenerateTypeComment generates a comment for an uncommented type
func (s *Service) GenerateTypeComment(typeName, typeDefinition string) (string, error) {
	systemPrompt := "You are a Go documentation expert. Generate concise, accurate doc comments for Go types. Follow Go conventions: start with the type name, be brief, explain what it represents."

	userPrompt := fmt.Sprintf(`Generate a doc comment for this Go type:

type %s %s

Return ONLY the comment text, without code fences or additional explanation.`, typeName, typeDefinition)

	return s.GenerateWithCache(FlagAutoComments, systemPrompt, userPrompt, 100)
}

// EnhanceDocumentation improves existing sparse documentation
func (s *Service) EnhanceDocumentation(symbolName, symbolType, existingDoc, signature string) (string, error) {
	systemPrompt := "You are a Go documentation expert. Improve sparse or unclear documentation while keeping it concise. Follow Go conventions."

	userPrompt := fmt.Sprintf(`Enhance this Go %s documentation:

Name: %s
Signature: %s
Current doc: %s

Provide improved documentation that:
1. Starts with the symbol name
2. Explains what it does clearly
3. Mentions important parameters/returns if applicable
4. Stays under 3 sentences

Return ONLY the improved doc text.`, symbolType, symbolName, signature, existingDoc)

	return s.GenerateWithCache(FlagEnhanceDocs, systemPrompt, userPrompt, 200)
}

// IsDocSparse checks if documentation is sparse and could benefit from enhancement
func IsDocSparse(doc string) bool {
	if doc == "" {
		return true
	}
	// Consider doc sparse if it's very short or lacks detail
	words := len(doc) / 5 // rough word count
	return words < 5
}

// GenerateEmbedding generates an embedding for text (used for semantic search)
func (s *Service) GenerateEmbedding(text string) ([]float32, error) {
	if !s.IsEnabled(FlagSemanticSearch) {
		return nil, fmt.Errorf("semantic search is not enabled")
	}
	if s.client == nil {
		return nil, fmt.Errorf("AI client not initialized")
	}
	return s.client.GenerateEmbedding(text)
}

// QueryUnderstanding represents the AI's interpretation of a search query
type QueryUnderstanding struct {
	OriginalQuery   string   `json:"original_query"`
	Intent          string   `json:"intent"`           // What the user is looking for
	Keywords        []string `json:"keywords"`         // Key technical terms
	SuggestedQueries []string `json:"suggested_queries"` // Refined search queries
	RelatedTopics   []string `json:"related_topics"`   // Related areas to explore
}

// UnderstandQuery interprets a vague or natural language query
func (s *Service) UnderstandQuery(query string) (*QueryUnderstanding, error) {
	systemPrompt := `You are a programming expert helping users find the right packages and libraries.
Analyze the user's search query and provide:
1. A clear interpretation of what they're looking for
2. Key technical keywords to search for
3. 2-3 refined search queries that would find relevant packages
4. Related topics they might also be interested in

Respond in JSON format only:
{
  "intent": "brief description of what user wants",
  "keywords": ["keyword1", "keyword2"],
  "suggested_queries": ["query1", "query2"],
  "related_topics": ["topic1", "topic2"]
}`

	userPrompt := fmt.Sprintf("User search query: %s", query)

	response, err := s.GenerateWithCache(FlagQueryUnderstanding, systemPrompt, userPrompt, 300)
	if err != nil {
		return nil, err
	}

	// Parse JSON response
	result := &QueryUnderstanding{
		OriginalQuery: query,
	}

	// Try to extract JSON from response (handle markdown code blocks)
	jsonStr := response
	if idx := findJSONStart(response); idx >= 0 {
		jsonStr = response[idx:]
		if end := findJSONEnd(jsonStr); end > 0 {
			jsonStr = jsonStr[:end+1]
		}
	}

	// Parse the JSON
	var parsed struct {
		Intent          string   `json:"intent"`
		Keywords        []string `json:"keywords"`
		SuggestedQueries []string `json:"suggested_queries"`
		RelatedTopics   []string `json:"related_topics"`
	}

	if err := parseJSON(jsonStr, &parsed); err != nil {
		// If parsing fails, use the raw response as intent
		result.Intent = response
		result.Keywords = []string{query}
		result.SuggestedQueries = []string{query}
		return result, nil
	}

	result.Intent = parsed.Intent
	result.Keywords = parsed.Keywords
	result.SuggestedQueries = parsed.SuggestedQueries
	result.RelatedTopics = parsed.RelatedTopics

	return result, nil
}

// findJSONStart finds the start of a JSON object in a string
func findJSONStart(s string) int {
	for i, c := range s {
		if c == '{' {
			return i
		}
	}
	return -1
}

// findJSONEnd finds the matching closing brace
func findJSONEnd(s string) int {
	depth := 0
	for i, c := range s {
		if c == '{' {
			depth++
		} else if c == '}' {
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// parseJSON attempts to parse JSON into a struct
func parseJSON(s string, v interface{}) error {
	// Simple JSON unmarshal
	return json.Unmarshal([]byte(s), v)
}

// CosineSimilarity computes cosine similarity between two vectors
func CosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dotProduct, normA, normB float32
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
}
