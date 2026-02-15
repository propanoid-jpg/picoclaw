// PicoClaw - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 PicoClaw contributors

package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// CachedContent represents a cached piece of content with its modification time.
type CachedContent struct {
	Content string
	ModTime time.Time
}

// ContextCache caches static content that rarely changes to avoid
// redundant disk I/O and enable prompt caching features.
type ContextCache struct {
	mu sync.RWMutex

	// Bootstrap files cache
	bootstrapFiles map[string]*CachedContent // filename -> content

	// Skills summary cache
	skillsSummary  *CachedContent
	skillsModTimes map[string]time.Time // skill file -> mod time for invalidation

	// Tools summary cache
	toolsSummary  string
	toolsVersion  int         // Increment when tools change
	toolsRegistry interface{} // Store reference to detect changes

	// Model tracking for cache invalidation
	currentModel string

	workspace string
}

// NewContextCache creates a new context cache.
func NewContextCache(workspace string) *ContextCache {
	return &ContextCache{
		bootstrapFiles: make(map[string]*CachedContent),
		skillsModTimes: make(map[string]time.Time),
		workspace:      workspace,
	}
}

// GetBootstrapFile returns cached bootstrap file content, loading from disk if needed.
// Bootstrap files are: AGENT.md, SOUL.md, USER.md, IDENTITY.md
func (cc *ContextCache) GetBootstrapFile(filename string) (string, error) {
	cc.mu.RLock()
	cached, exists := cc.bootstrapFiles[filename]
	cc.mu.RUnlock()

	filePath := filepath.Join(cc.workspace, filename)

	// Check if file exists and get mod time
	stat, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, return empty content
			return "", nil
		}
		return "", err
	}

	// If cached and not modified, return cached content
	if exists && cached.ModTime.Equal(stat.ModTime()) {
		return cached.Content, nil
	}

	// Load from disk
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	// Update cache
	cc.mu.Lock()
	cc.bootstrapFiles[filename] = &CachedContent{
		Content: string(content),
		ModTime: stat.ModTime(),
	}
	cc.mu.Unlock()

	return string(content), nil
}

// GetAllBootstrapFiles returns all bootstrap files as a map.
func (cc *ContextCache) GetAllBootstrapFiles() (map[string]string, error) {
	bootstrapFileNames := []string{"AGENT.md", "SOUL.md", "USER.md", "IDENTITY.md"}
	result := make(map[string]string)

	for _, filename := range bootstrapFileNames {
		content, err := cc.GetBootstrapFile(filename)
		if err != nil {
			return nil, fmt.Errorf("loading %s: %w", filename, err)
		}
		if content != "" {
			result[filename] = content
		}
	}

	return result, nil
}

// GetSkillsSummary returns cached skills summary, regenerating if needed.
// Checks modification times of all skill files to detect changes.
func (cc *ContextCache) GetSkillsSummary(skillsDir string, generator func() (string, error)) (string, error) {
	// Scan skills directory for modifications
	needsUpdate := false

	err := filepath.Walk(skillsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		cc.mu.RLock()
		cachedModTime, exists := cc.skillsModTimes[path]
		cc.mu.RUnlock()

		if !exists || !cachedModTime.Equal(info.ModTime()) {
			needsUpdate = true
			cc.mu.Lock()
			cc.skillsModTimes[path] = info.ModTime()
			cc.mu.Unlock()
		}

		return nil
	})

	if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	// Check if we have cached content and don't need update
	cc.mu.RLock()
	cached := cc.skillsSummary
	cc.mu.RUnlock()

	if cached != nil && !needsUpdate {
		return cached.Content, nil
	}

	// Regenerate skills summary
	summary, err := generator()
	if err != nil {
		return "", err
	}

	// Cache the result
	cc.mu.Lock()
	cc.skillsSummary = &CachedContent{
		Content: summary,
		ModTime: time.Now(),
	}
	cc.mu.Unlock()

	return summary, nil
}

// GetToolsSummary returns cached tools summary, regenerating if version changed.
func (cc *ContextCache) GetToolsSummary(version int, generator func() string) string {
	cc.mu.RLock()
	if cc.toolsVersion == version && cc.toolsSummary != "" {
		summary := cc.toolsSummary
		cc.mu.RUnlock()
		return summary
	}
	cc.mu.RUnlock()

	// Regenerate tools summary
	summary := generator()

	// Cache the result
	cc.mu.Lock()
	cc.toolsSummary = summary
	cc.toolsVersion = version
	cc.mu.Unlock()

	return summary
}

// InvalidateBootstrapFiles clears the bootstrap files cache.
func (cc *ContextCache) InvalidateBootstrapFiles() {
	cc.mu.Lock()
	cc.bootstrapFiles = make(map[string]*CachedContent)
	cc.mu.Unlock()
}

// InvalidateSkills clears the skills summary cache.
func (cc *ContextCache) InvalidateSkills() {
	cc.mu.Lock()
	cc.skillsSummary = nil
	cc.skillsModTimes = make(map[string]time.Time)
	cc.mu.Unlock()
}

// InvalidateTools clears the tools summary cache.
func (cc *ContextCache) InvalidateTools() {
	cc.mu.Lock()
	cc.toolsSummary = ""
	cc.toolsVersion++
	cc.mu.Unlock()
}

// InvalidateAll clears all caches.
func (cc *ContextCache) InvalidateAll() {
	cc.InvalidateBootstrapFiles()
	cc.InvalidateSkills()
	cc.InvalidateTools()
}

// SetModel sets the current model and invalidates all caches if the model changed.
// Returns true if the model changed and caches were invalidated.
func (cc *ContextCache) SetModel(model string) bool {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	// If model hasn't changed, nothing to do
	if cc.currentModel == model {
		return false
	}

	// Model changed - invalidate all caches
	cc.currentModel = model
	cc.bootstrapFiles = make(map[string]*CachedContent)
	cc.skillsSummary = nil
	cc.skillsModTimes = make(map[string]time.Time)
	cc.toolsSummary = ""
	cc.toolsVersion++

	return true
}

// GetCurrentModel returns the current model name.
func (cc *ContextCache) GetCurrentModel() string {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	return cc.currentModel
}

// GetCacheStats returns cache statistics for debugging.
func (cc *ContextCache) GetCacheStats() map[string]interface{} {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	return map[string]interface{}{
		"bootstrap_files_cached": len(cc.bootstrapFiles),
		"skills_cached":          cc.skillsSummary != nil,
		"tools_cached":           cc.toolsSummary != "",
		"tools_version":          cc.toolsVersion,
		"current_model":          cc.currentModel,
	}
}
