// PicoClaw - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 PicoClaw contributors

package agent

import (
	"fmt"
)

// ContextBudget manages the allocation of the model's context window
// across different components: system prompt, history, and output.
type ContextBudget struct {
	contextWindow int // Total context window size
	maxTokens     int // Max output tokens

	// Budget allocations (in tokens)
	systemPromptBudget int // 20% of context window
	historyBudget      int // 70% of context window
	outputBudget       int // Actual output limit from config
	memoryBudget       int // Subset of system prompt budget for memory
}

// NewContextBudget creates a new context budget manager.
// It allocates the context window as follows:
// - System prompt: 20% (includes bootstrap files, skills, tools, memory)
// - History: 70% (conversation messages and tool calls)
// - Output: From maxTokens config (typically 4096-8192)
func NewContextBudget(contextWindow, maxTokens int) *ContextBudget {
	// Allocate budgets
	systemPromptBudget := contextWindow / 5                     // 20%
	historyBudget := (contextWindow * 7) / 10                   // 70%
	memoryBudget := systemPromptBudget / 2                      // Half of system prompt budget for memory

	return &ContextBudget{
		contextWindow:      contextWindow,
		maxTokens:          maxTokens,
		systemPromptBudget: systemPromptBudget,
		historyBudget:      historyBudget,
		outputBudget:       maxTokens,
		memoryBudget:       memoryBudget,
	}
}

// GetSystemPromptBudget returns the token budget for the system prompt.
func (cb *ContextBudget) GetSystemPromptBudget() int {
	return cb.systemPromptBudget
}

// GetHistoryBudget returns the token budget for conversation history.
func (cb *ContextBudget) GetHistoryBudget() int {
	return cb.historyBudget
}

// GetOutputBudget returns the token budget for output generation.
func (cb *ContextBudget) GetOutputBudget() int {
	return cb.outputBudget
}

// GetMemoryBudget returns the token budget for memory context.
func (cb *ContextBudget) GetMemoryBudget() int {
	return cb.memoryBudget
}

// GetContextWindow returns the total context window size.
func (cb *ContextBudget) GetContextWindow() int {
	return cb.contextWindow
}

// CheckSystemPromptFits checks if the system prompt fits within budget.
// Returns an error if it exceeds the budget.
func (cb *ContextBudget) CheckSystemPromptFits(tokenCount int) error {
	if tokenCount > cb.systemPromptBudget {
		return fmt.Errorf("system prompt too large: %d tokens exceeds budget of %d tokens (%.1f%% over)",
			tokenCount, cb.systemPromptBudget,
			float64(tokenCount-cb.systemPromptBudget)*100.0/float64(cb.systemPromptBudget))
	}
	return nil
}

// CheckHistoryFits checks if the history fits within budget.
// Returns true if it fits, false if summarization is needed.
func (cb *ContextBudget) CheckHistoryFits(tokenCount int) bool {
	return tokenCount <= cb.historyBudget
}

// GetSummarizationThreshold returns the token count at which
// summarization should be triggered (60% of history budget).
func (cb *ContextBudget) GetSummarizationThreshold() int {
	return (cb.historyBudget * 60) / 100
}

// CheckTotalFits checks if the total token count (system + history + output)
// fits within the context window.
func (cb *ContextBudget) CheckTotalFits(systemTokens, historyTokens int) error {
	total := systemTokens + historyTokens + cb.outputBudget
	if total > cb.contextWindow {
		return fmt.Errorf("total context too large: %d tokens (system=%d + history=%d + output=%d) exceeds window of %d tokens",
			total, systemTokens, historyTokens, cb.outputBudget, cb.contextWindow)
	}
	return nil
}

// GetBudgetSummary returns a human-readable summary of the budget allocation.
func (cb *ContextBudget) GetBudgetSummary() string {
	return fmt.Sprintf("ContextBudget: window=%d, system=%d (20%%), history=%d (70%%), output=%d, memory=%d",
		cb.contextWindow, cb.systemPromptBudget, cb.historyBudget, cb.outputBudget, cb.memoryBudget)
}
