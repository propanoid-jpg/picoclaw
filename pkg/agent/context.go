package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/pkg/providers"
	"github.com/sipeed/picoclaw/pkg/skills"
	"github.com/sipeed/picoclaw/pkg/tools"
	"github.com/sipeed/picoclaw/pkg/utils"
)

type ContextBuilder struct {
	workspace    string
	skillsLoader *skills.SkillsLoader
	memory       *MemoryStore
	tools        *tools.ToolRegistry // Direct reference to tool registry
	cache        *ContextCache       // Cache for static content
	budget       *ContextBudget      // Token budget for context management
}

func getGlobalConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".picoclaw")
}

func NewContextBuilder(workspace string) *ContextBuilder {
	// builtin skills: skills directory in current project
	// Use the skills/ directory under the current working directory
	wd, _ := os.Getwd()
	builtinSkillsDir := filepath.Join(wd, "skills")
	globalSkillsDir := filepath.Join(getGlobalConfigDir(), "skills")

	return &ContextBuilder{
		workspace:    workspace,
		skillsLoader: skills.NewSkillsLoader(workspace, globalSkillsDir, builtinSkillsDir),
		memory:       NewMemoryStore(workspace),
		cache:        NewContextCache(workspace),
	}
}

// SetToolsRegistry sets the tools registry for dynamic tool summary generation.
func (cb *ContextBuilder) SetToolsRegistry(registry *tools.ToolRegistry) {
	cb.tools = registry
}

// SetBudget sets the context budget for token management.
func (cb *ContextBuilder) SetBudget(budget *ContextBudget) {
	cb.budget = budget
}

// SetModel sets the current model and invalidates caches if the model changed.
// Logs cache invalidation events for debugging.
func (cb *ContextBuilder) SetModel(model string) {
	invalidated := cb.cache.SetModel(model)
	if invalidated {
		logger.InfoCF("context", "Cache invalidated due to model change",
			map[string]interface{}{
				"new_model": model,
			})
	}
}

func (cb *ContextBuilder) getIdentity() string {
	now := time.Now().Format("2006-01-02 15:04 (Monday)")
	workspacePath, _ := filepath.Abs(filepath.Join(cb.workspace))
	runtime := fmt.Sprintf("%s %s, Go %s", runtime.GOOS, runtime.GOARCH, runtime.Version())

	// Build tools section dynamically
	toolsSection := cb.buildToolsSection()

	return fmt.Sprintf(`# picoclaw 🦞

You are picoclaw, a helpful AI assistant.

## Current Time
%s

## Runtime
%s

## Workspace
Your workspace is at: %s
- Memory: %s/memory/MEMORY.md
- Daily Notes: %s/memory/YYYYMM/YYYYMMDD.md
- Skills: %s/skills/{skill-name}/SKILL.md

%s

## Important Rules

1. **ALWAYS use tools** - When you need to perform an action (schedule reminders, send messages, execute commands, etc.), you MUST call the appropriate tool. Do NOT just say you'll do it or pretend to do it.

2. **Be helpful and accurate** - When using tools, briefly explain what you're doing.

3. **Memory** - When remembering something, write to %s/memory/MEMORY.md`,
		now, runtime, workspacePath, workspacePath, workspacePath, workspacePath, toolsSection, workspacePath)
}

func (cb *ContextBuilder) buildToolsSection() string {
	if cb.tools == nil {
		return ""
	}

	// Use cache with a simple version counter
	// Version is based on the number of registered tools (simple but effective)
	summaries := cb.tools.GetSummaries()
	version := len(summaries)

	return cb.cache.GetToolsSummary(version, func() string {
		if len(summaries) == 0 {
			return ""
		}

		// Simplified tools section: tools are fully described in the API call's
		// structured tool definitions, so we don't need to duplicate them here.
		// This saves ~750-2000 tokens per request.
		var sb strings.Builder
		sb.WriteString("## Available Tools\n\n")
		sb.WriteString("**CRITICAL**: You MUST use tools to perform actions. Do NOT pretend to execute commands or schedule tasks.\n\n")
		sb.WriteString(fmt.Sprintf("You have access to %d tools for file operations, system commands, web search, messaging, and more. ", len(summaries)))
		sb.WriteString("Use the appropriate tools when needed to accomplish tasks.\n")

		return sb.String()
	})
}

func (cb *ContextBuilder) BuildSystemPrompt() string {
	return cb.BuildSystemPromptWithBudget(nil)
}

func (cb *ContextBuilder) BuildSystemPromptWithBudget(budget *ContextBudget) string {
	parts := []string{}

	// Core identity section
	parts = append(parts, cb.getIdentity())

	// Bootstrap files
	bootstrapContent := cb.LoadBootstrapFiles()
	if bootstrapContent != "" {
		parts = append(parts, bootstrapContent)
	}

	// Skills - show summary, AI can read full content with read_file tool
	// Use cache to avoid regenerating skills summary on every request
	skillsDir := filepath.Join(cb.workspace, "skills")
	skillsSummary, err := cb.cache.GetSkillsSummary(skillsDir, func() (string, error) {
		return cb.skillsLoader.BuildSkillsSummary(), nil
	})
	if err != nil {
		logger.ErrorCF("context", "Failed to get skills summary", map[string]interface{}{
			"error": err.Error(),
		})
		skillsSummary = ""
	}
	if skillsSummary != "" {
		parts = append(parts, fmt.Sprintf(`# Skills

The following skills extend your capabilities. To use a skill, read its SKILL.md file using the read_file tool.

%s`, skillsSummary))
	}

	// Memory context - with budget if provided
	var memoryContext string
	if budget != nil {
		memoryContext = cb.memory.GetMemoryContextWithBudget(budget.GetMemoryBudget())
	} else {
		memoryContext = cb.memory.GetMemoryContext()
	}
	if memoryContext != "" {
		parts = append(parts, memoryContext) // Memory already includes "# Memory" header
	}

	// Join with "---" separator
	return strings.Join(parts, "\n\n---\n\n")
}

func (cb *ContextBuilder) LoadBootstrapFiles() string {
	// Use cache to avoid redundant disk I/O
	bootstrapMap, err := cb.cache.GetAllBootstrapFiles()
	if err != nil {
		logger.ErrorCF("context", "Failed to load bootstrap files from cache", map[string]interface{}{
			"error": err.Error(),
		})
		return ""
	}

	// Build result in order
	bootstrapFiles := []string{"AGENT.md", "SOUL.md", "USER.md", "IDENTITY.md"}
	var result string
	for _, filename := range bootstrapFiles {
		if content, exists := bootstrapMap[filename]; exists {
			result += fmt.Sprintf("## %s\n\n%s\n\n", filename, content)
		}
	}

	return result
}

// buildUserMessage creates a user message with text and optional media attachments
func (cb *ContextBuilder) buildUserMessage(text string, mediaPaths []string) providers.Message {
	// If no media, return simple text message
	if len(mediaPaths) == 0 {
		return providers.Message{
			Role:    "user",
			Content: text,
		}
	}

	// Build content blocks with text and images
	blocks := []providers.ContentBlock{}

	// Add text block if there's text content
	if text != "" {
		blocks = append(blocks, providers.ContentBlock{
			Type: "text",
			Text: text,
		})
	}

	// Add image blocks for image files
	for _, mediaPath := range mediaPaths {
		if !utils.IsImageFile(mediaPath) {
			logger.DebugCF("agent", "Skipping non-image file in media", map[string]interface{}{
				"path": mediaPath,
			})
			continue
		}

		// Read and encode image
		base64Data, err := utils.ImageToBase64(mediaPath)
		if err != nil {
			logger.ErrorCF("agent", "Failed to encode image", map[string]interface{}{
				"path":  mediaPath,
				"error": err.Error(),
			})
			continue
		}

		mediaType := utils.GetImageMediaType(mediaPath)
		blocks = append(blocks, providers.ContentBlock{
			Type: "image",
			Source: &providers.ImageSource{
				Type:      "base64",
				MediaType: mediaType,
				Data:      base64Data,
			},
		})

		logger.DebugCF("agent", "Added image to message", map[string]interface{}{
			"path":       mediaPath,
			"media_type": mediaType,
			"size_bytes": len(base64Data),
		})
	}

	// If we have blocks, use them; otherwise fall back to text
	if len(blocks) > 0 {
		return providers.Message{
			Role:    "user",
			Content: blocks,
		}
	}

	return providers.Message{
		Role:    "user",
		Content: text,
	}
}

func (cb *ContextBuilder) BuildMessages(history []providers.Message, summary string, currentMessage string, media []string, channel, chatID string) []providers.Message {
	messages := []providers.Message{}

	systemPrompt := cb.BuildSystemPromptWithBudget(cb.budget)

	// Add Current Session info if provided
	if channel != "" && chatID != "" {
		systemPrompt += fmt.Sprintf("\n\n## Current Session\nChannel: %s\nChat ID: %s", channel, chatID)
	}

	// Log system prompt summary for debugging (debug mode only)
	logger.DebugCF("agent", "System prompt built",
		map[string]interface{}{
			"total_chars":   len(systemPrompt),
			"total_lines":   strings.Count(systemPrompt, "\n") + 1,
			"section_count": strings.Count(systemPrompt, "\n\n---\n\n") + 1,
		})

	// Log preview of system prompt (avoid logging huge content)
	preview := systemPrompt
	if len(preview) > 500 {
		preview = preview[:500] + "... (truncated)"
	}
	logger.DebugCF("agent", "System prompt preview",
		map[string]interface{}{
			"preview": preview,
		})

	if summary != "" {
		systemPrompt += "\n\n## Summary of Previous Conversation\n\n" + summary
	}

	//This fix prevents the session memory from LLM failure due to elimination of toolu_IDs required from LLM
	// --- INICIO DEL FIX ---
	//Diegox-17
	for len(history) > 0 && (history[0].Role == "tool") {
		logger.DebugCF("agent", "Removing orphaned tool message from history to prevent LLM error",
			map[string]interface{}{"role": history[0].Role})
		history = history[1:]
	}
	//Diegox-17
	// --- FIN DEL FIX ---

	messages = append(messages, providers.Message{
		Role:    "system",
		Content: systemPrompt,
	})

	messages = append(messages, history...)

	// Build user message with media if present
	userMessage := cb.buildUserMessage(currentMessage, media)
	messages = append(messages, userMessage)

	return messages
}

func (cb *ContextBuilder) AddToolResult(messages []providers.Message, toolCallID, toolName, result string) []providers.Message {
	messages = append(messages, providers.Message{
		Role:       "tool",
		Content:    result,
		ToolCallID: toolCallID,
	})
	return messages
}

func (cb *ContextBuilder) AddAssistantMessage(messages []providers.Message, content string, toolCalls []map[string]interface{}) []providers.Message {
	msg := providers.Message{
		Role:    "assistant",
		Content: content,
	}
	// Always add assistant message, whether or not it has tool calls
	messages = append(messages, msg)
	return messages
}

func (cb *ContextBuilder) loadSkills() string {
	allSkills := cb.skillsLoader.ListSkills()
	if len(allSkills) == 0 {
		return ""
	}

	var skillNames []string
	for _, s := range allSkills {
		skillNames = append(skillNames, s.Name)
	}

	content := cb.skillsLoader.LoadSkillsForContext(skillNames)
	if content == "" {
		return ""
	}

	return "# Skill Definitions\n\n" + content
}

// GetSkillsInfo returns information about loaded skills.
func (cb *ContextBuilder) GetSkillsInfo() map[string]interface{} {
	allSkills := cb.skillsLoader.ListSkills()
	skillNames := make([]string, 0, len(allSkills))
	for _, s := range allSkills {
		skillNames = append(skillNames, s.Name)
	}
	return map[string]interface{}{
		"total":     len(allSkills),
		"available": len(allSkills),
		"names":     skillNames,
	}
}
