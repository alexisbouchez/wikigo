package ai

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

// CacheEntry represents a cached AI-generated response
type CacheEntry struct {
	Content   string
	CreatedAt time.Time
	ExpiresAt time.Time
	CostUSD   float64
	Tokens    int
}

// Cache provides in-memory caching for AI-generated content
type Cache struct {
	entries map[string]*CacheEntry
	mu      sync.RWMutex
	ttl     time.Duration
	stats   CacheStats
}

// CacheStats tracks cache performance
type CacheStats struct {
	Hits          int64
	Misses        int64
	Evictions     int64
	TotalSavings  float64 // Estimated cost savings from cache hits
	CurrentSize   int
}

// NewCache creates a new cache with the given TTL
func NewCache(ttl time.Duration) *Cache {
	cache := &Cache{
		entries: make(map[string]*CacheEntry),
		ttl:     ttl,
	}

	// Start background cleanup goroutine
	go cache.cleanupExpired()

	return cache
}

// generateKey creates a cache key from the prompt
func (c *Cache) generateKey(prompt string) string {
	hash := sha256.Sum256([]byte(prompt))
	return hex.EncodeToString(hash[:])
}

// Get retrieves a cached entry if it exists and is not expired
func (c *Cache) Get(prompt string) (string, bool) {
	key := c.generateKey(prompt)

	c.mu.RLock()
	entry, exists := c.entries[key]
	c.mu.RUnlock()

	if !exists {
		c.recordMiss()
		return "", false
	}

	// Check if expired
	if time.Now().After(entry.ExpiresAt) {
		c.mu.Lock()
		delete(c.entries, key)
		c.stats.Evictions++
		c.stats.CurrentSize = len(c.entries)
		c.mu.Unlock()

		c.recordMiss()
		return "", false
	}

	c.recordHit(entry.CostUSD)
	return entry.Content, true
}

// Set stores a new entry in the cache
func (c *Cache) Set(prompt, content string, costUSD float64, tokens int) {
	key := c.generateKey(prompt)

	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	c.entries[key] = &CacheEntry{
		Content:   content,
		CreatedAt: now,
		ExpiresAt: now.Add(c.ttl),
		CostUSD:   costUSD,
		Tokens:    tokens,
	}

	c.stats.CurrentSize = len(c.entries)
}

// Clear removes all entries from the cache
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*CacheEntry)
	c.stats.CurrentSize = 0
}

// GetStats returns a copy of the cache statistics
func (c *Cache) GetStats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.stats
}

// cleanupExpired runs periodically to remove expired entries
func (c *Cache) cleanupExpired() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()

		now := time.Now()
		for key, entry := range c.entries {
			if now.After(entry.ExpiresAt) {
				delete(c.entries, key)
				c.stats.Evictions++
			}
		}

		c.stats.CurrentSize = len(c.entries)
		c.mu.Unlock()
	}
}

func (c *Cache) recordHit(costSaved float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stats.Hits++
	c.stats.TotalSavings += costSaved
}

func (c *Cache) recordMiss() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stats.Misses++
}

// HitRate returns the cache hit rate as a percentage
func (c *Cache) HitRate() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := c.stats.Hits + c.stats.Misses
	if total == 0 {
		return 0
	}

	return float64(c.stats.Hits) / float64(total) * 100
}
