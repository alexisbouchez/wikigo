# AI Package

Provides AI-powered features for wikigo using Mistral AI, with built-in caching, rate limiting, and cost controls.

## Features

- **Mistral API Client**: HTTP client with automatic rate limiting
- **Feature Flags**: Enable/disable individual AI features
- **Caching**: Reduce API costs by caching responses (24h TTL by default)
- **Budget Control**: Daily and monthly spending limits
- **Cost Tracking**: Monitor API usage and costs

## Setup

1. Get a Mistral API key from https://console.mistral.ai/
2. Set environment variable:
```bash
export MISTRAL_API_KEY=your_key_here
```

## Usage

### Basic Setup

```go
import "github.com/alexisbouchez/wikigo/ai"

// Create service from environment variables
service := ai.NewServiceFromEnv()

// Or create manually
service := ai.NewService(apiKey, 10, 24*time.Hour)

// Enable features
service.flags.Enable(ai.FlagAutoComments)
service.flags.Enable(ai.FlagExplainCode)

// Set budget (optional, defaults: $1/day, $30/month)
service.SetBudget(5.0, 100.0)
```

### Generate Function Comments

```go
comment, err := service.GenerateFunctionComment(
    "func ProcessPackage(pkg *Package) error",
    "{ /* function body */ }",
)
```

### Explain Code

```go
explanation, err := service.ExplainCode(`
func min(a, b int) int {
    if a < b {
        return a
    }
    return b
}
`)
```

### Generate Package Synopsis

```go
synopsis, err := service.GeneratePackageSynopsis(
    "httputil",
    []string{"Client", "Server", "RoundTripper"},
)
```

### Summarize License

```go
summary, err := service.SummarizeLicense(licenseText)
```

## Available Feature Flags

### Documentation Generation
- `FlagAutoComments` - Auto-generate function/method comments
- `FlagAutoSynopsis` - Auto-generate package synopsis
- `FlagEnhanceDocs` - Enhance existing sparse documentation

### Search Features
- `FlagSemanticSearch` - Semantic search using embeddings
- `FlagQueryUnderstanding` - Natural language query interpretation

### Code Examples
- `FlagAutoExamples` - Auto-generate usage examples
- `FlagPlaygroundLinks` - Generate Go Playground links

### User-Facing
- `FlagExplainCode` - "Explain this code" feature
- `FlagLicenseSummary` - Plain-English license summaries
- `FlagDocTranslation` - Translate documentation

## Statistics

```go
stats := service.GetStats()

fmt.Printf("Total requests: %d\n", stats["total_requests"])
fmt.Printf("Total cost: $%.2f\n", stats["total_cost_usd"])
fmt.Printf("Cache hit rate: %.1f%%\n", stats["cache_hit_rate"])
fmt.Printf("Cache savings: $%.2f\n", stats["cache_savings_usd"])
```

## Rate Limiting

The client uses a token bucket algorithm with configurable requests per minute:

```go
// Allow up to 20 requests per minute
service := ai.NewService(apiKey, 20, 24*time.Hour)
```

Requests automatically wait if the limit is exceeded.

## Caching

Responses are cached by prompt hash. Cache key = SHA256(systemPrompt + userPrompt).

- Default TTL: 24 hours
- Automatic cleanup of expired entries (every 5 minutes)
- Thread-safe

```go
// Custom cache TTL
cache := ai.NewCache(48 * time.Hour)
```

## Budget Control

Budget limits prevent runaway costs:

```go
budget := ai.NewBudget(
    5.0,   // $5 per day
    100.0, // $100 per month
)

// Automatically checked before each request
if !budget.CanSpend(estimatedCost) {
    // Request blocked
}
```

Budget resets automatically at midnight (daily) and month start (monthly).

## Cost Estimation

Approximate costs for Mistral Small (as of 2024):
- $0.002 per 1K tokens (input + output combined)
- Average function comment: ~100-200 tokens = $0.0002-0.0004
- Average explanation: ~200-300 tokens = $0.0004-0.0006

With caching enabled, costs are significantly reduced for repeated queries.

## Error Handling

```go
content, err := service.GenerateFunctionComment(sig, body)
if err != nil {
    // Common errors:
    // - Feature not enabled
    // - Budget exceeded
    // - Rate limit (shouldn't happen with built-in limiter)
    // - API error (network, auth, etc.)
    log.Printf("AI generation failed: %v", err)
}
```

## Thread Safety

All components are thread-safe:
- Client uses mutex for stats
- Cache uses RWMutex for concurrent reads
- Feature flags use RWMutex
- Rate limiter uses mutex

## Testing Without API Key

Service gracefully handles missing API keys:

```go
service := ai.NewService("", 10, 24*time.Hour)
// All features disabled, IsEnabled() returns false
```
