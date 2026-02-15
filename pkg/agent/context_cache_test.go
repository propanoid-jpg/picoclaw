// PicoClaw - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 PicoClaw contributors

package agent

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestContextCache_SetModel_Invalidation(t *testing.T) {
	// Create temp workspace
	tmpDir := t.TempDir()
	cache := NewContextCache(tmpDir)

	// Set initial model
	changed := cache.SetModel("anthropic/claude-opus-4-5")
	if !changed {
		t.Error("Expected cache to be invalidated on first SetModel call")
	}

	// Verify model was set
	if cache.GetCurrentModel() != "anthropic/claude-opus-4-5" {
		t.Errorf("Expected model to be 'anthropic/claude-opus-4-5', got '%s'", cache.GetCurrentModel())
	}

	// Cache some content
	cache.bootstrapFiles["AGENT.md"] = &CachedContent{
		Content: "test content",
		ModTime: time.Now(),
	}
	cache.skillsSummary = &CachedContent{
		Content: "skills summary",
		ModTime: time.Now(),
	}
	cache.toolsSummary = "tools summary"
	cache.toolsVersion = 5

	// Set same model again - should NOT invalidate
	changed = cache.SetModel("anthropic/claude-opus-4-5")
	if changed {
		t.Error("Expected cache NOT to be invalidated when setting same model")
	}

	// Verify caches are still populated
	if len(cache.bootstrapFiles) == 0 {
		t.Error("Bootstrap files cache should not be cleared when model hasn't changed")
	}

	// Change model - should invalidate
	changed = cache.SetModel("openai/gpt-4")
	if !changed {
		t.Error("Expected cache to be invalidated when model changes")
	}

	// Verify model was updated
	if cache.GetCurrentModel() != "openai/gpt-4" {
		t.Errorf("Expected model to be 'openai/gpt-4', got '%s'", cache.GetCurrentModel())
	}

	// Verify all caches were cleared
	if len(cache.bootstrapFiles) != 0 {
		t.Error("Bootstrap files cache should be cleared when model changes")
	}
	if cache.skillsSummary != nil {
		t.Error("Skills summary cache should be cleared when model changes")
	}
	if cache.toolsSummary != "" {
		t.Error("Tools summary cache should be cleared when model changes")
	}
}

func TestContextCache_GetCacheStats_IncludesModel(t *testing.T) {
	tmpDir := t.TempDir()
	cache := NewContextCache(tmpDir)

	// Set model
	cache.SetModel("anthropic/claude-opus-4-5")

	// Get stats
	stats := cache.GetCacheStats()

	// Verify model is included in stats
	model, ok := stats["current_model"].(string)
	if !ok {
		t.Fatal("Expected current_model in cache stats")
	}
	if model != "anthropic/claude-opus-4-5" {
		t.Errorf("Expected current_model to be 'anthropic/claude-opus-4-5', got '%s'", model)
	}
}

func TestContextCache_BootstrapFileCache_NotInvalidatedByOtherChanges(t *testing.T) {
	tmpDir := t.TempDir()
	cache := NewContextCache(tmpDir)

	// Set model
	cache.SetModel("anthropic/claude-opus-4-5")

	// Create a test bootstrap file
	agentFile := filepath.Join(tmpDir, "AGENT.md")
	content := "# Agent Instructions\n\nTest content"
	if err := os.WriteFile(agentFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Load bootstrap file (should cache it)
	loadedContent, err := cache.GetBootstrapFile("AGENT.md")
	if err != nil {
		t.Fatal(err)
	}
	if loadedContent != content {
		t.Errorf("Expected content '%s', got '%s'", content, loadedContent)
	}

	// Verify it was cached
	if len(cache.bootstrapFiles) != 1 {
		t.Error("Bootstrap file should be cached")
	}

	// Invalidate tools only
	cache.InvalidateTools()

	// Bootstrap cache should still be intact
	if len(cache.bootstrapFiles) != 1 {
		t.Error("Bootstrap files should not be cleared by InvalidateTools")
	}

	// But changing model should clear everything
	cache.SetModel("openai/gpt-4")
	if len(cache.bootstrapFiles) != 0 {
		t.Error("Bootstrap files should be cleared when model changes")
	}
}

func TestContextCache_ThreadSafety(t *testing.T) {
	tmpDir := t.TempDir()
	cache := NewContextCache(tmpDir)

	// Run multiple goroutines changing models concurrently
	done := make(chan bool)
	models := []string{
		"anthropic/claude-opus-4-5",
		"openai/gpt-4",
		"anthropic/claude-sonnet-4-5",
		"openai/gpt-4-turbo",
	}

	for i := 0; i < 10; i++ {
		go func(idx int) {
			model := models[idx%len(models)]
			cache.SetModel(model)
			_ = cache.GetCurrentModel()
			_ = cache.GetCacheStats()
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify cache is in a consistent state
	currentModel := cache.GetCurrentModel()
	if currentModel == "" {
		t.Error("Expected a model to be set after concurrent operations")
	}

	// Verify stats are accessible
	stats := cache.GetCacheStats()
	if stats == nil {
		t.Error("Expected cache stats to be available")
	}
}
