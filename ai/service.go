package ai

import (
	"fmt"
	"log"
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
