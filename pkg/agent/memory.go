// PicoClaw - Ultra-lightweight personal AI agent
// Inspired by and based on nanobot: https://github.com/HKUDS/nanobot
// License: MIT
//
// Copyright (c) 2026 PicoClaw contributors

package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/sipeed/picoclaw/pkg/tools"
)

// MemoryStore manages persistent memory for the agent.
// - Long-term memory: memory/MEMORY.md
// - Daily notes: memory/YYYYMM/YYYYMMDD.md
type MemoryStore struct {
	workspace  string
	memoryDir  string
	memoryFile string
}

// NewMemoryStore creates a new MemoryStore with the given workspace path.
// It ensures the memory directory exists.
func NewMemoryStore(workspace string) *MemoryStore {
	memoryDir := filepath.Join(workspace, "memory")
	memoryFile := filepath.Join(memoryDir, "MEMORY.md")

	// Ensure memory directory exists
	os.MkdirAll(memoryDir, 0755)

	return &MemoryStore{
		workspace:  workspace,
		memoryDir:  memoryDir,
		memoryFile: memoryFile,
	}
}

// getTodayFile returns the path to today's daily note file (memory/YYYYMM/YYYYMMDD.md).
func (ms *MemoryStore) getTodayFile() string {
	today := time.Now().Format("20060102") // YYYYMMDD
	monthDir := today[:6]                  // YYYYMM
	filePath := filepath.Join(ms.memoryDir, monthDir, today+".md")
	return filePath
}

// ReadLongTerm reads the long-term memory (MEMORY.md).
// Returns empty string if the file doesn't exist.
func (ms *MemoryStore) ReadLongTerm() string {
	if data, err := os.ReadFile(ms.memoryFile); err == nil {
		return string(data)
	}
	return ""
}

// WriteLongTerm writes content to the long-term memory file (MEMORY.md).
func (ms *MemoryStore) WriteLongTerm(content string) error {
	lockMgr := tools.GetGlobalFileLockManager()
	return lockMgr.WithLock(ms.memoryFile, func() error {
		return os.WriteFile(ms.memoryFile, []byte(content), 0644)
	})
}

// ReadToday reads today's daily note.
// Returns empty string if the file doesn't exist.
func (ms *MemoryStore) ReadToday() string {
	todayFile := ms.getTodayFile()
	if data, err := os.ReadFile(todayFile); err == nil {
		return string(data)
	}
	return ""
}

// AppendToday appends content to today's daily note.
// If the file doesn't exist, it creates a new file with a date header.
func (ms *MemoryStore) AppendToday(content string) error {
	todayFile := ms.getTodayFile()

	lockMgr := tools.GetGlobalFileLockManager()
	return lockMgr.WithLock(todayFile, func() error {
		// Ensure month directory exists
		monthDir := filepath.Dir(todayFile)
		os.MkdirAll(monthDir, 0755)

		var existingContent string
		if data, err := os.ReadFile(todayFile); err == nil {
			existingContent = string(data)
		}

		var newContent string
		if existingContent == "" {
			// Add header for new day
			header := fmt.Sprintf("# %s\n\n", time.Now().Format("2006-01-02"))
			newContent = header + content
		} else {
			// Append to existing content
			newContent = existingContent + "\n" + content
		}

		return os.WriteFile(todayFile, []byte(newContent), 0644)
	})
}

// GetRecentDailyNotes returns daily notes from the last N days.
// Contents are joined with "---" separator.
func (ms *MemoryStore) GetRecentDailyNotes(days int) string {
	var notes []string

	for i := 0; i < days; i++ {
		date := time.Now().AddDate(0, 0, -i)
		dateStr := date.Format("20060102") // YYYYMMDD
		monthDir := dateStr[:6]            // YYYYMM
		filePath := filepath.Join(ms.memoryDir, monthDir, dateStr+".md")

		if data, err := os.ReadFile(filePath); err == nil {
			notes = append(notes, string(data))
		}
	}

	if len(notes) == 0 {
		return ""
	}

	// Join with separator
	var result string
	for i, note := range notes {
		if i > 0 {
			result += "\n\n---\n\n"
		}
		result += note
	}
	return result
}

// GetMemoryContext returns formatted memory context for the agent prompt.
// Includes long-term memory and recent daily notes.
func (ms *MemoryStore) GetMemoryContext() string {
	return ms.GetMemoryContextWithBudget(0) // No budget limit
}

// GetMemoryContextWithBudget returns formatted memory context within a token budget.
// Strategy:
//   - Priority 1: Recent daily notes (last 3 days)
//   - Priority 2: Long-term memory (MEMORY.md), truncated if needed
//   - If MEMORY.md exceeds budget, keep first 40% + last 40% (skip middle)
func (ms *MemoryStore) GetMemoryContextWithBudget(maxTokens int) string {
	var parts []string

	// Recent daily notes (last 3 days) - highest priority
	recentNotes := ms.GetRecentDailyNotes(3)
	recentNotesTokens := estimateTokensInText(recentNotes)

	// Long-term memory
	longTerm := ms.ReadLongTerm()
	longTermTokens := estimateTokensInText(longTerm)

	// If no budget limit, return everything
	if maxTokens == 0 {
		if longTerm != "" {
			parts = append(parts, "## Long-term Memory\n\n"+longTerm)
		}
		if recentNotes != "" {
			parts = append(parts, "## Recent Daily Notes\n\n"+recentNotes)
		}
	} else {
		// With budget: prioritize recent notes, truncate long-term memory if needed
		remainingBudget := maxTokens

		// Add recent notes first (they're more important)
		if recentNotes != "" && recentNotesTokens <= remainingBudget {
			parts = append(parts, "## Recent Daily Notes\n\n"+recentNotes)
			remainingBudget -= recentNotesTokens
		}

		// Add long-term memory, truncating if necessary
		if longTerm != "" {
			if longTermTokens <= remainingBudget {
				// Fits entirely
				parts = append(parts, "## Long-term Memory\n\n"+longTerm)
			} else if remainingBudget > 100 {
				// Truncate: keep first 40% and last 40%, skip middle
				// This preserves both old and new content
				runes := []rune(longTerm)
				totalRunes := len(runes)

				// Calculate how many characters we can keep (roughly 4 chars per token)
				maxChars := remainingBudget * 4
				keepStart := (maxChars * 40) / 100
				keepEnd := (maxChars * 40) / 100

				if keepStart+keepEnd < totalRunes {
					truncated := string(runes[:keepStart]) +
						"\n\n[...middle section truncated to fit token budget...]\n\n" +
						string(runes[totalRunes-keepEnd:])
					parts = append(parts, "## Long-term Memory\n\n"+truncated)
				} else {
					// Just take what we can from the start
					truncated := string(runes[:maxChars])
					parts = append(parts, "## Long-term Memory\n\n"+truncated)
				}
			}
		}
	}

	if len(parts) == 0 {
		return ""
	}

	// Join parts with separator
	var result string
	for i, part := range parts {
		if i > 0 {
			result += "\n\n---\n\n"
		}
		result += part
	}
	return fmt.Sprintf("# Memory\n\n%s", result)
}
