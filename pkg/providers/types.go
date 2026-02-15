package providers

import (
	"context"
	"encoding/json"
)

type ToolCall struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type,omitempty"`
	Function  *FunctionCall          `json:"function,omitempty"`
	Name      string                 `json:"name,omitempty"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type LLMResponse struct {
	Content      string     `json:"content"`
	ToolCalls    []ToolCall `json:"tool_calls,omitempty"`
	FinishReason string     `json:"finish_reason"`
	Usage        *UsageInfo `json:"usage,omitempty"`
}

type UsageInfo struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ContentBlock represents a piece of content (text or image)
type ContentBlock struct {
	Type      string       `json:"type"` // "text" or "image"
	Text      string       `json:"text,omitempty"`
	Source    *ImageSource `json:"source,omitempty"`
	MediaType string       `json:"media_type,omitempty"` // For image blocks
}

// ImageSource represents an image in a content block
type ImageSource struct {
	Type      string `json:"type"`       // "base64" or "url"
	MediaType string `json:"media_type"` // e.g., "image/jpeg"
	Data      string `json:"data"`       // base64-encoded image data
}

// MessageContent can be either a string or []ContentBlock
type MessageContent interface{}

type Message struct {
	Role       string         `json:"role"`
	Content    MessageContent `json:"content"` // Can be string or []ContentBlock
	ToolCalls  []ToolCall     `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
}

// GetTextContent extracts text content from a Message, handling both string and []ContentBlock types
func (m *Message) GetTextContent() string {
	switch v := m.Content.(type) {
	case string:
		return v
	case []ContentBlock:
		for _, block := range v {
			if block.Type == "text" {
				return block.Text
			}
		}
		return ""
	default:
		return ""
	}
}

// MarshalJSON custom marshaler to handle MessageContent properly
func (m Message) MarshalJSON() ([]byte, error) {
	type Alias Message
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(&m),
	})
}

type LLMProvider interface {
	Chat(ctx context.Context, messages []Message, tools []ToolDefinition, model string, options map[string]interface{}) (*LLMResponse, error)
	GetDefaultModel() string
}

type ToolDefinition struct {
	Type     string                 `json:"type"`
	Function ToolFunctionDefinition `json:"function"`
}

type ToolFunctionDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}
