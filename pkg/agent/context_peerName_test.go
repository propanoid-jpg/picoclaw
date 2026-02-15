package agent

import (
	"testing"
)

func TestExtractPeerName(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]string
		expected string
	}{
		{
			name:     "nil metadata",
			metadata: nil,
			expected: "",
		},
		{
			name:     "empty metadata",
			metadata: map[string]string{},
			expected: "",
		},
		{
			name: "display_name (Discord) - highest priority",
			metadata: map[string]string{
				"display_name": "Alice",
				"username":     "alice123",
				"first_name":   "Alice Smith",
			},
			expected: "Alice",
		},
		{
			name: "sender_name (DingTalk) - second priority",
			metadata: map[string]string{
				"sender_name": "Bob Chen",
				"username":    "bobchen",
			},
			expected: "Bob Chen",
		},
		{
			name: "user_name (WhatsApp) - third priority",
			metadata: map[string]string{
				"user_name": "Charlie",
				"username":  "charlie123",
			},
			expected: "Charlie",
		},
		{
			name: "first_name (Telegram) - fourth priority",
			metadata: map[string]string{
				"first_name": "David",
				"username":   "david123",
			},
			expected: "David",
		},
		{
			name: "username only - lowest priority",
			metadata: map[string]string{
				"username": "eve_user",
			},
			expected: "eve_user",
		},
		{
			name: "no relevant fields",
			metadata: map[string]string{
				"user_id":  "12345",
				"is_group": "false",
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractPeerName(tt.metadata)
			if result != tt.expected {
				t.Errorf("extractPeerName() = %q, want %q", result, tt.expected)
			}
		})
	}
}
