package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	MistralAPIURL      = "https://api.mistral.ai/v1/chat/completions"
	MistralEmbedAPIURL = "https://api.mistral.ai/v1/embeddings"
	DefaultModel       = "mistral-small-latest"
	EmbeddingModel     = "mistral-embed"
)

// Client represents a Mistral AI API client with rate limiting
type Client struct {
	apiKey      string
	model       string
	httpClient  *http.Client
	rateLimiter *RateLimiter
	stats       *Stats
	statsMu     sync.Mutex
}

// Stats tracks API usage statistics
type Stats struct {
	TotalRequests   int64
	FailedRequests  int64
	TotalTokens     int64
	PromptTokens    int64
	CompletionTokens int64
	TotalCostUSD    float64
	LastRequestTime time.Time
}

// RateLimiter implements token bucket rate limiting
type RateLimiter struct {
	tokens         int
	maxTokens      int
	refillRate     time.Duration
	lastRefillTime time.Time
	mu             sync.Mutex
}

// NewRateLimiter creates a rate limiter with given capacity and refill rate
func NewRateLimiter(requestsPerMinute int) *RateLimiter {
	return &RateLimiter{
		tokens:         requestsPerMinute,
		maxTokens:      requestsPerMinute,
		refillRate:     time.Minute / time.Duration(requestsPerMinute),
		lastRefillTime: time.Now(),
	}
}

// Allow checks if a request is allowed and consumes a token
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Refill tokens based on time elapsed
	now := time.Now()
	elapsed := now.Sub(rl.lastRefillTime)
	tokensToAdd := int(elapsed / rl.refillRate)

	if tokensToAdd > 0 {
		rl.tokens = min(rl.maxTokens, rl.tokens+tokensToAdd)
		rl.lastRefillTime = now
	}

	// Check if we have tokens available
	if rl.tokens > 0 {
		rl.tokens--
		return true
	}

	return false
}

// Wait blocks until a token is available
func (rl *RateLimiter) Wait() {
	for !rl.Allow() {
		time.Sleep(rl.refillRate)
	}
}

// NewClient creates a new Mistral AI client
func NewClient(apiKey string, requestsPerMinute int) *Client {
	if requestsPerMinute <= 0 {
		requestsPerMinute = 10 // Default: 10 requests per minute
	}

	return &Client{
		apiKey: apiKey,
		model:  DefaultModel,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		rateLimiter: NewRateLimiter(requestsPerMinute),
		stats:       &Stats{},
	}
}

// SetModel sets the model to use for completions
func (c *Client) SetModel(model string) {
	c.model = model
}

// ChatMessage represents a message in the chat
type ChatMessage struct {
	Role    string `json:"role"`    // "system", "user", or "assistant"
	Content string `json:"content"`
}

// ChatRequest represents a chat completion request
type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
}

// ChatResponse represents a chat completion response
type ChatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int         `json:"index"`
		Message ChatMessage `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// Complete sends a chat completion request to Mistral AI
func (c *Client) Complete(messages []ChatMessage, temperature float64, maxTokens int) (*ChatResponse, error) {
	// Wait for rate limiter
	c.rateLimiter.Wait()

	// Prepare request
	req := ChatRequest{
		Model:       c.model,
		Messages:    messages,
		Temperature: temperature,
		MaxTokens:   maxTokens,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequest("POST", MistralAPIURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	// Send request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		c.recordFailedRequest()
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.recordFailedRequest()
		return nil, fmt.Errorf("reading response: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		c.recordFailedRequest()
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var chatResp ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		c.recordFailedRequest()
		return nil, fmt.Errorf("unmarshaling response: %w", err)
	}

	// Update stats
	c.recordSuccessfulRequest(&chatResp)

	return &chatResp, nil
}

// GenerateText is a convenience method for simple text generation
func (c *Client) GenerateText(systemPrompt, userPrompt string, maxTokens int) (string, error) {
	messages := []ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	resp, err := c.Complete(messages, 0.7, maxTokens)
	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no completion choices returned")
	}

	return resp.Choices[0].Message.Content, nil
}

func (c *Client) recordSuccessfulRequest(resp *ChatResponse) {
	c.statsMu.Lock()
	defer c.statsMu.Unlock()

	c.stats.TotalRequests++
	c.stats.TotalTokens += int64(resp.Usage.TotalTokens)
	c.stats.PromptTokens += int64(resp.Usage.PromptTokens)
	c.stats.CompletionTokens += int64(resp.Usage.CompletionTokens)
	c.stats.LastRequestTime = time.Now()

	// Approximate cost based on Mistral pricing
	// mistral-small: ~$0.002 per 1K tokens (input + output)
	costPer1KTokens := 0.002
	c.stats.TotalCostUSD += float64(resp.Usage.TotalTokens) / 1000.0 * costPer1KTokens
}

func (c *Client) recordFailedRequest() {
	c.statsMu.Lock()
	defer c.statsMu.Unlock()

	c.stats.TotalRequests++
	c.stats.FailedRequests++
	c.stats.LastRequestTime = time.Now()
}

// GetStats returns a copy of the current statistics
func (c *Client) GetStats() Stats {
	c.statsMu.Lock()
	defer c.statsMu.Unlock()
	return *c.stats
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// EmbeddingRequest represents an embedding API request
type EmbeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// EmbeddingResponse represents an embedding API response
type EmbeddingResponse struct {
	ID     string `json:"id"`
	Object string `json:"object"`
	Data   []struct {
		Object    string    `json:"object"`
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

// GenerateEmbedding generates an embedding for a single text
func (c *Client) GenerateEmbedding(text string) ([]float32, error) {
	embeddings, err := c.GenerateEmbeddings([]string{text})
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}
	return embeddings[0], nil
}

// GenerateEmbeddings generates embeddings for multiple texts
func (c *Client) GenerateEmbeddings(texts []string) ([][]float32, error) {
	// Wait for rate limiter
	c.rateLimiter.Wait()

	// Prepare request
	req := EmbeddingRequest{
		Model: EmbeddingModel,
		Input: texts,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequest("POST", MistralEmbedAPIURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	// Send request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		c.recordFailedRequest()
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.recordFailedRequest()
		return nil, fmt.Errorf("reading response: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		c.recordFailedRequest()
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var embedResp EmbeddingResponse
	if err := json.Unmarshal(body, &embedResp); err != nil {
		c.recordFailedRequest()
		return nil, fmt.Errorf("unmarshaling response: %w", err)
	}

	// Update stats
	c.statsMu.Lock()
	c.stats.TotalRequests++
	c.stats.TotalTokens += int64(embedResp.Usage.TotalTokens)
	c.stats.PromptTokens += int64(embedResp.Usage.PromptTokens)
	c.stats.LastRequestTime = time.Now()
	// Embedding cost is lower than chat (~$0.0001 per 1K tokens)
	c.stats.TotalCostUSD += float64(embedResp.Usage.TotalTokens) / 1000.0 * 0.0001
	c.statsMu.Unlock()

	// Extract embeddings
	embeddings := make([][]float32, len(embedResp.Data))
	for _, d := range embedResp.Data {
		embeddings[d.Index] = d.Embedding
	}

	return embeddings, nil
}
