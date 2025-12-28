package ai

import (
	"sync"
)

// FeatureFlag represents an AI feature that can be enabled/disabled
type FeatureFlag string

const (
	// Documentation generation features
	FlagAutoComments      FeatureFlag = "auto_comments"
	FlagAutoSynopsis      FeatureFlag = "auto_synopsis"
	FlagEnhanceDocs       FeatureFlag = "enhance_docs"

	// Search features
	FlagSemanticSearch    FeatureFlag = "semantic_search"
	FlagQueryUnderstanding FeatureFlag = "query_understanding"

	// Code examples
	FlagAutoExamples      FeatureFlag = "auto_examples"
	FlagPlaygroundLinks   FeatureFlag = "playground_links"

	// User-facing features
	FlagExplainCode       FeatureFlag = "explain_code"
	FlagLicenseSummary    FeatureFlag = "license_summary"
	FlagDocTranslation    FeatureFlag = "doc_translation"
)

// FeatureFlags manages feature flag state
type FeatureFlags struct {
	flags map[FeatureFlag]bool
	mu    sync.RWMutex
}

// NewFeatureFlags creates a new feature flags manager with all features disabled by default
func NewFeatureFlags() *FeatureFlags {
	return &FeatureFlags{
		flags: make(map[FeatureFlag]bool),
	}
}

// Enable enables a feature flag
func (ff *FeatureFlags) Enable(flag FeatureFlag) {
	ff.mu.Lock()
	defer ff.mu.Unlock()
	ff.flags[flag] = true
}

// Disable disables a feature flag
func (ff *FeatureFlags) Disable(flag FeatureFlag) {
	ff.mu.Lock()
	defer ff.mu.Unlock()
	ff.flags[flag] = false
}

// IsEnabled checks if a feature flag is enabled
func (ff *FeatureFlags) IsEnabled(flag FeatureFlag) bool {
	ff.mu.RLock()
	defer ff.mu.RUnlock()
	return ff.flags[flag]
}

// EnableAll enables all feature flags
func (ff *FeatureFlags) EnableAll() {
	ff.mu.Lock()
	defer ff.mu.Unlock()

	allFlags := []FeatureFlag{
		FlagAutoComments,
		FlagAutoSynopsis,
		FlagEnhanceDocs,
		FlagSemanticSearch,
		FlagQueryUnderstanding,
		FlagAutoExamples,
		FlagPlaygroundLinks,
		FlagExplainCode,
		FlagLicenseSummary,
		FlagDocTranslation,
	}

	for _, flag := range allFlags {
		ff.flags[flag] = true
	}
}

// DisableAll disables all feature flags
func (ff *FeatureFlags) DisableAll() {
	ff.mu.Lock()
	defer ff.mu.Unlock()
	ff.flags = make(map[FeatureFlag]bool)
}

// GetEnabled returns a list of all enabled features
func (ff *FeatureFlags) GetEnabled() []FeatureFlag {
	ff.mu.RLock()
	defer ff.mu.RUnlock()

	var enabled []FeatureFlag
	for flag, isEnabled := range ff.flags {
		if isEnabled {
			enabled = append(enabled, flag)
		}
	}
	return enabled
}
