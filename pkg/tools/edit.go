package tools

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// EditFileTool edits a file by replacing old_text with new_text.
// The old_text must exist exactly in the file.
type EditFileTool struct {
	allowedDir string
	restrict   bool
}

// NewEditFileTool creates a new EditFileTool with optional directory restriction.
func NewEditFileTool(allowedDir string, restrict bool) *EditFileTool {
	return &EditFileTool{
		allowedDir: allowedDir,
		restrict:   restrict,
	}
}

func (t *EditFileTool) Name() string {
	return "edit_file"
}

func (t *EditFileTool) Description() string {
	return "Edit a file by replacing old_text with new_text. The old_text must exist exactly in the file."
}

func (t *EditFileTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "The file path to edit",
			},
			"old_text": map[string]interface{}{
				"type":        "string",
				"description": "The exact text to find and replace",
			},
			"new_text": map[string]interface{}{
				"type":        "string",
				"description": "The text to replace with",
			},
		},
		"required": []string{"path", "old_text", "new_text"},
	}
}

func (t *EditFileTool) Execute(ctx context.Context, args map[string]interface{}) *ToolResult {
	path, ok := args["path"].(string)
	if !ok {
		return ErrorResult("path is required")
	}

	oldText, ok := args["old_text"].(string)
	if !ok {
		return ErrorResult("old_text is required")
	}

	newText, ok := args["new_text"].(string)
	if !ok {
		return ErrorResult("new_text is required")
	}

	resolvedPath, err := validatePath(path, t.allowedDir, t.restrict)
	if err != nil {
		return ErrorResult(err.Error())
	}

	// Acquire file lock for thread-safe read-modify-write
	lockMgr := GetGlobalFileLockManager()
	var result *ToolResult
	lockErr := lockMgr.WithLock(resolvedPath, func() error {
		if _, err := os.Stat(resolvedPath); os.IsNotExist(err) {
			result = ErrorResult(fmt.Sprintf("file not found: %s", path))
			return nil
		}

		content, err := os.ReadFile(resolvedPath)
		if err != nil {
			result = ErrorResult(fmt.Sprintf("failed to read file: %v", err))
			return nil
		}

		contentStr := string(content)

		if !strings.Contains(contentStr, oldText) {
			result = ErrorResult("old_text not found in file. Make sure it matches exactly")
			return nil
		}

		count := strings.Count(contentStr, oldText)
		if count > 1 {
			result = ErrorResult(fmt.Sprintf("old_text appears %d times. Please provide more context to make it unique", count))
			return nil
		}

		newContent := strings.Replace(contentStr, oldText, newText, 1)

		if err := os.WriteFile(resolvedPath, []byte(newContent), 0644); err != nil {
			result = ErrorResult(fmt.Sprintf("failed to write file: %v", err))
			return nil
		}

		result = SilentResult(fmt.Sprintf("File edited: %s", path))
		return nil
	})

	if lockErr != nil {
		return ErrorResult(fmt.Sprintf("lock error: %v", lockErr))
	}

	return result
}

type AppendFileTool struct {
	workspace string
	restrict  bool
}

func NewAppendFileTool(workspace string, restrict bool) *AppendFileTool {
	return &AppendFileTool{workspace: workspace, restrict: restrict}
}

func (t *AppendFileTool) Name() string {
	return "append_file"
}

func (t *AppendFileTool) Description() string {
	return "Append content to the end of a file"
}

func (t *AppendFileTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "The file path to append to",
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "The content to append",
			},
		},
		"required": []string{"path", "content"},
	}
}

func (t *AppendFileTool) Execute(ctx context.Context, args map[string]interface{}) *ToolResult {
	path, ok := args["path"].(string)
	if !ok {
		return ErrorResult("path is required")
	}

	content, ok := args["content"].(string)
	if !ok {
		return ErrorResult("content is required")
	}

	resolvedPath, err := validatePath(path, t.workspace, t.restrict)
	if err != nil {
		return ErrorResult(err.Error())
	}

	// Acquire file lock for thread-safe append
	lockMgr := GetGlobalFileLockManager()
	var result *ToolResult
	lockErr := lockMgr.WithLock(resolvedPath, func() error {
		f, err := os.OpenFile(resolvedPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			result = ErrorResult(fmt.Sprintf("failed to open file: %v", err))
			return nil
		}
		defer f.Close()

		if _, err := f.WriteString(content); err != nil {
			result = ErrorResult(fmt.Sprintf("failed to append to file: %v", err))
			return nil
		}

		result = SilentResult(fmt.Sprintf("Appended to %s", path))
		return nil
	})

	if lockErr != nil {
		return ErrorResult(fmt.Sprintf("lock error: %v", lockErr))
	}

	return result
}
