package providers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/sipeed/picoclaw/pkg/auth"
)

type ClaudeProvider struct {
	client      *anthropic.Client
	tokenSource func() (string, error)
}

func NewClaudeProvider(token string) *ClaudeProvider {
	client := anthropic.NewClient(
		option.WithAPIKey(token),
		option.WithBaseURL("https://api.anthropic.com"),
	)
	return &ClaudeProvider{client: &client}
}

func NewClaudeProviderWithTokenSource(token string, tokenSource func() (string, error)) *ClaudeProvider {
	p := NewClaudeProvider(token)
	p.tokenSource = tokenSource
	return p
}

func (p *ClaudeProvider) Chat(ctx context.Context, messages []Message, tools []ToolDefinition, model string, options map[string]interface{}) (*LLMResponse, error) {
	var opts []option.RequestOption
	if p.tokenSource != nil {
		tok, err := p.tokenSource()
		if err != nil {
			return nil, fmt.Errorf("refreshing token: %w", err)
		}
		opts = append(opts, option.WithAPIKey(tok))
	}

	params, err := buildClaudeParams(messages, tools, model, options)
	if err != nil {
		return nil, err
	}

	resp, err := p.client.Messages.New(ctx, params, opts...)
	if err != nil {
		return nil, fmt.Errorf("claude API call: %w", err)
	}

	return parseClaudeResponse(resp), nil
}

func (p *ClaudeProvider) GetDefaultModel() string {
	return "claude-sonnet-4-5-20250929"
}

// convertContentToAnthropicBlocks converts MessageContent to Anthropic content blocks
func convertContentToAnthropicBlocks(content MessageContent) []anthropic.ContentBlockParamUnion {
	var blocks []anthropic.ContentBlockParamUnion

	switch v := content.(type) {
	case string:
		// Simple text content
		blocks = append(blocks, anthropic.NewTextBlock(v))
	case []ContentBlock:
		// Structured content with text and/or images
		for _, block := range v {
			switch block.Type {
			case "text":
				blocks = append(blocks, anthropic.NewTextBlock(block.Text))
			case "image":
				if block.Source != nil {
					blocks = append(blocks, anthropic.NewImageBlockBase64(
						block.Source.MediaType,
						block.Source.Data,
					))
				}
			}
		}
	default:
		// Fallback to empty text block if unknown type
		blocks = append(blocks, anthropic.NewTextBlock(""))
	}

	return blocks
}

func buildClaudeParams(messages []Message, tools []ToolDefinition, model string, options map[string]interface{}) (anthropic.MessageNewParams, error) {
	var system []anthropic.TextBlockParam
	var anthropicMessages []anthropic.MessageParam

	// Track if we should enable prompt caching (only for supported models)
	enableCaching := false
	if val, ok := options["enable_prompt_caching"]; ok {
		if b, ok := val.(bool); ok {
			enableCaching = b
		}
	}

	for _, msg := range messages {
		switch msg.Role {
		case "system":
			// System messages are always text
			textContent := msg.GetTextContent()
			block := anthropic.TextBlockParam{
				Text: textContent,
			}

			// Enable caching for system blocks if requested
			// Cache the main system prompt but not the last block (which may be dynamic memory)
			if enableCaching && len(system) > 0 {
				// Mark previous block as cacheable
				cacheControl := anthropic.NewCacheControlEphemeralParam()
				system[len(system)-1].CacheControl = cacheControl
			}

			system = append(system, block)
		case "user":
			if msg.ToolCallID != "" {
				// Tool result - always text
				textContent := msg.GetTextContent()
				anthropicMessages = append(anthropicMessages,
					anthropic.NewUserMessage(anthropic.NewToolResultBlock(msg.ToolCallID, textContent, false)),
				)
			} else {
				// User message - can have text + images
				blocks := convertContentToAnthropicBlocks(msg.Content)
				anthropicMessages = append(anthropicMessages,
					anthropic.NewUserMessage(blocks...),
				)
			}
		case "assistant":
			if len(msg.ToolCalls) > 0 {
				var blocks []anthropic.ContentBlockParamUnion
				textContent := msg.GetTextContent()
				if textContent != "" {
					blocks = append(blocks, anthropic.NewTextBlock(textContent))
				}
				for _, tc := range msg.ToolCalls {
					blocks = append(blocks, anthropic.NewToolUseBlock(tc.ID, tc.Arguments, tc.Name))
				}
				anthropicMessages = append(anthropicMessages, anthropic.NewAssistantMessage(blocks...))
			} else {
				textContent := msg.GetTextContent()
				anthropicMessages = append(anthropicMessages,
					anthropic.NewAssistantMessage(anthropic.NewTextBlock(textContent)),
				)
			}
		case "tool":
			textContent := msg.GetTextContent()
			anthropicMessages = append(anthropicMessages,
				anthropic.NewUserMessage(anthropic.NewToolResultBlock(msg.ToolCallID, textContent, false)),
			)
		}
	}

	maxTokens := int64(4096)
	if mt, ok := options["max_tokens"].(int); ok {
		maxTokens = int64(mt)
	}

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		Messages:  anthropicMessages,
		MaxTokens: maxTokens,
	}

	if len(system) > 0 {
		params.System = system
	}

	if temp, ok := options["temperature"].(float64); ok {
		params.Temperature = anthropic.Float(temp)
	}

	if len(tools) > 0 {
		params.Tools = translateToolsForClaude(tools)
	}

	return params, nil
}

func translateToolsForClaude(tools []ToolDefinition) []anthropic.ToolUnionParam {
	result := make([]anthropic.ToolUnionParam, 0, len(tools))
	for _, t := range tools {
		tool := anthropic.ToolParam{
			Name: t.Function.Name,
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: t.Function.Parameters["properties"],
			},
		}
		if desc := t.Function.Description; desc != "" {
			tool.Description = anthropic.String(desc)
		}
		if req, ok := t.Function.Parameters["required"].([]interface{}); ok {
			required := make([]string, 0, len(req))
			for _, r := range req {
				if s, ok := r.(string); ok {
					required = append(required, s)
				}
			}
			tool.InputSchema.Required = required
		}
		result = append(result, anthropic.ToolUnionParam{OfTool: &tool})
	}
	return result
}

func parseClaudeResponse(resp *anthropic.Message) *LLMResponse {
	var content string
	var toolCalls []ToolCall

	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			tb := block.AsText()
			content += tb.Text
		case "tool_use":
			tu := block.AsToolUse()
			var args map[string]interface{}
			if err := json.Unmarshal(tu.Input, &args); err != nil {
				args = map[string]interface{}{"raw": string(tu.Input)}
			}
			toolCalls = append(toolCalls, ToolCall{
				ID:        tu.ID,
				Name:      tu.Name,
				Arguments: args,
			})
		}
	}

	finishReason := "stop"
	switch resp.StopReason {
	case anthropic.StopReasonToolUse:
		finishReason = "tool_calls"
	case anthropic.StopReasonMaxTokens:
		finishReason = "length"
	case anthropic.StopReasonEndTurn:
		finishReason = "stop"
	}

	return &LLMResponse{
		Content:      content,
		ToolCalls:    toolCalls,
		FinishReason: finishReason,
		Usage: &UsageInfo{
			PromptTokens:     int(resp.Usage.InputTokens),
			CompletionTokens: int(resp.Usage.OutputTokens),
			TotalTokens:      int(resp.Usage.InputTokens + resp.Usage.OutputTokens),
		},
	}
}

func createClaudeTokenSource() func() (string, error) {
	return func() (string, error) {
		cred, err := auth.GetCredential("anthropic")
		if err != nil {
			return "", fmt.Errorf("loading auth credentials: %w", err)
		}
		if cred == nil {
			return "", fmt.Errorf("no credentials for anthropic. Run: picoclaw auth login --provider anthropic")
		}
		return cred.AccessToken, nil
	}
}
