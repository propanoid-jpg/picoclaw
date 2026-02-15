package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// TestEditTool_EditFile_Success verifies successful file editing
func TestEditTool_EditFile_Success(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("Hello World\nThis is a test"), 0644)

	tool := NewEditFileTool(tmpDir, true)
	ctx := context.Background()
	args := map[string]interface{}{
		"path":     testFile,
		"old_text": "World",
		"new_text": "Universe",
	}

	result := tool.Execute(ctx, args)

	// Success should not be an error
	if result.IsError {
		t.Errorf("Expected success, got IsError=true: %s", result.ForLLM)
	}

	// Should return SilentResult
	if !result.Silent {
		t.Errorf("Expected Silent=true for EditFile, got false")
	}

	// ForUser should be empty (silent result)
	if result.ForUser != "" {
		t.Errorf("Expected ForUser to be empty for SilentResult, got: %s", result.ForUser)
	}

	// Verify file was actually edited
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read edited file: %v", err)
	}
	contentStr := string(content)
	if !strings.Contains(contentStr, "Hello Universe") {
		t.Errorf("Expected file to contain 'Hello Universe', got: %s", contentStr)
	}
	if strings.Contains(contentStr, "Hello World") {
		t.Errorf("Expected 'Hello World' to be replaced, got: %s", contentStr)
	}
}

// TestEditTool_EditFile_NotFound verifies error handling for non-existent file
func TestEditTool_EditFile_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "nonexistent.txt")

	tool := NewEditFileTool(tmpDir, true)
	ctx := context.Background()
	args := map[string]interface{}{
		"path":     testFile,
		"old_text": "old",
		"new_text": "new",
	}

	result := tool.Execute(ctx, args)

	// Should return error result
	if !result.IsError {
		t.Errorf("Expected error for non-existent file")
	}

	// Should mention file not found
	if !strings.Contains(result.ForLLM, "not found") && !strings.Contains(result.ForUser, "not found") {
		t.Errorf("Expected 'file not found' message, got ForLLM: %s", result.ForLLM)
	}
}

// TestEditTool_EditFile_OldTextNotFound verifies error when old_text doesn't exist
func TestEditTool_EditFile_OldTextNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("Hello World"), 0644)

	tool := NewEditFileTool(tmpDir, true)
	ctx := context.Background()
	args := map[string]interface{}{
		"path":     testFile,
		"old_text": "Goodbye",
		"new_text": "Hello",
	}

	result := tool.Execute(ctx, args)

	// Should return error result
	if !result.IsError {
		t.Errorf("Expected error when old_text not found")
	}

	// Should mention old_text not found
	if !strings.Contains(result.ForLLM, "not found") && !strings.Contains(result.ForUser, "not found") {
		t.Errorf("Expected 'not found' message, got ForLLM: %s", result.ForLLM)
	}
}

// TestEditTool_EditFile_MultipleMatches verifies error when old_text appears multiple times
func TestEditTool_EditFile_MultipleMatches(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test test test"), 0644)

	tool := NewEditFileTool(tmpDir, true)
	ctx := context.Background()
	args := map[string]interface{}{
		"path":     testFile,
		"old_text": "test",
		"new_text": "done",
	}

	result := tool.Execute(ctx, args)

	// Should return error result
	if !result.IsError {
		t.Errorf("Expected error when old_text appears multiple times")
	}

	// Should mention multiple occurrences
	if !strings.Contains(result.ForLLM, "times") && !strings.Contains(result.ForUser, "times") {
		t.Errorf("Expected 'multiple times' message, got ForLLM: %s", result.ForLLM)
	}
}

// TestEditTool_EditFile_OutsideAllowedDir verifies error when path is outside allowed directory
func TestEditTool_EditFile_OutsideAllowedDir(t *testing.T) {
	tmpDir := t.TempDir()
	otherDir := t.TempDir()
	testFile := filepath.Join(otherDir, "test.txt")
	os.WriteFile(testFile, []byte("content"), 0644)

	tool := NewEditFileTool(tmpDir, true) // Restrict to tmpDir
	ctx := context.Background()
	args := map[string]interface{}{
		"path":     testFile,
		"old_text": "content",
		"new_text": "new",
	}

	result := tool.Execute(ctx, args)

	// Should return error result
	if !result.IsError {
		t.Errorf("Expected error when path is outside allowed directory")
	}

	// Should mention outside allowed directory
	if !strings.Contains(result.ForLLM, "outside") && !strings.Contains(result.ForUser, "outside") {
		t.Errorf("Expected 'outside allowed' message, got ForLLM: %s", result.ForLLM)
	}
}

// TestEditTool_EditFile_MissingPath verifies error handling for missing path
func TestEditTool_EditFile_MissingPath(t *testing.T) {
	tool := NewEditFileTool("", false)
	ctx := context.Background()
	args := map[string]interface{}{
		"old_text": "old",
		"new_text": "new",
	}

	result := tool.Execute(ctx, args)

	// Should return error result
	if !result.IsError {
		t.Errorf("Expected error when path is missing")
	}
}

// TestEditTool_EditFile_MissingOldText verifies error handling for missing old_text
func TestEditTool_EditFile_MissingOldText(t *testing.T) {
	tool := NewEditFileTool("", false)
	ctx := context.Background()
	args := map[string]interface{}{
		"path":     "/tmp/test.txt",
		"new_text": "new",
	}

	result := tool.Execute(ctx, args)

	// Should return error result
	if !result.IsError {
		t.Errorf("Expected error when old_text is missing")
	}
}

// TestEditTool_EditFile_MissingNewText verifies error handling for missing new_text
func TestEditTool_EditFile_MissingNewText(t *testing.T) {
	tool := NewEditFileTool("", false)
	ctx := context.Background()
	args := map[string]interface{}{
		"path":     "/tmp/test.txt",
		"old_text": "old",
	}

	result := tool.Execute(ctx, args)

	// Should return error result
	if !result.IsError {
		t.Errorf("Expected error when new_text is missing")
	}
}

// TestEditTool_AppendFile_Success verifies successful file appending
func TestEditTool_AppendFile_Success(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("Initial content"), 0644)

	tool := NewAppendFileTool("", false)
	ctx := context.Background()
	args := map[string]interface{}{
		"path":    testFile,
		"content": "\nAppended content",
	}

	result := tool.Execute(ctx, args)

	// Success should not be an error
	if result.IsError {
		t.Errorf("Expected success, got IsError=true: %s", result.ForLLM)
	}

	// Should return SilentResult
	if !result.Silent {
		t.Errorf("Expected Silent=true for AppendFile, got false")
	}

	// ForUser should be empty (silent result)
	if result.ForUser != "" {
		t.Errorf("Expected ForUser to be empty for SilentResult, got: %s", result.ForUser)
	}

	// Verify content was actually appended
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	contentStr := string(content)
	if !strings.Contains(contentStr, "Initial content") {
		t.Errorf("Expected original content to remain, got: %s", contentStr)
	}
	if !strings.Contains(contentStr, "Appended content") {
		t.Errorf("Expected appended content, got: %s", contentStr)
	}
}

// TestEditTool_AppendFile_MissingPath verifies error handling for missing path
func TestEditTool_AppendFile_MissingPath(t *testing.T) {
	tool := NewAppendFileTool("", false)
	ctx := context.Background()
	args := map[string]interface{}{
		"content": "test",
	}

	result := tool.Execute(ctx, args)

	// Should return error result
	if !result.IsError {
		t.Errorf("Expected error when path is missing")
	}
}

// TestEditTool_AppendFile_MissingContent verifies error handling for missing content
func TestEditTool_AppendFile_MissingContent(t *testing.T) {
	tool := NewAppendFileTool("", false)
	ctx := context.Background()
	args := map[string]interface{}{
		"path": "/tmp/test.txt",
	}

	result := tool.Execute(ctx, args)

	// Should return error result
	if !result.IsError {
		t.Errorf("Expected error when content is missing")
	}
}

// TestEditTool_ConcurrentAppends verifies that concurrent appends don't corrupt files
func TestEditTool_ConcurrentAppends(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "concurrent_append.txt")
	os.WriteFile(testFile, []byte(""), 0644) // Start with empty file

	tool := NewAppendFileTool(tmpDir, true)
	ctx := context.Background()

	// Number of concurrent appends
	numAppends := 50
	var wg sync.WaitGroup
	wg.Add(numAppends)

	// Each goroutine appends a unique line
	for i := 0; i < numAppends; i++ {
		go func(id int) {
			defer wg.Done()
			args := map[string]interface{}{
				"path":    testFile,
				"content": fmt.Sprintf("Line %d\n", id),
			}
			result := tool.Execute(ctx, args)
			if result.IsError {
				t.Errorf("Append %d failed: %s", id, result.ForLLM)
			}
		}(i)
	}

	wg.Wait()

	// Read final content
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatal(err)
	}

	// Count lines - should have exactly numAppends lines
	lines := strings.Split(string(content), "\n")
	nonEmptyLines := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			nonEmptyLines++
		}
	}

	if nonEmptyLines != numAppends {
		t.Errorf("Expected %d lines, got %d. File may be corrupted due to concurrent access:\n%s",
			numAppends, nonEmptyLines, string(content))
	}

	// Verify each line appears exactly once
	linesSeen := make(map[string]int)
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			linesSeen[line]++
		}
	}

	for line, count := range linesSeen {
		if count != 1 {
			t.Errorf("Line '%s' appears %d times (expected 1). Concurrent writes may have corrupted the file.", line, count)
		}
	}
}

// TestEditTool_ConcurrentEdits verifies that concurrent edits are serialized properly
func TestEditTool_ConcurrentEdits(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "concurrent_edit.txt")
	os.WriteFile(testFile, []byte("Counter: 0"), 0644)

	tool := NewEditFileTool(tmpDir, true)
	ctx := context.Background()

	// Number of concurrent edits
	numEdits := 20
	var wg sync.WaitGroup
	wg.Add(numEdits)

	// Each goroutine increments the counter
	for i := 0; i < numEdits; i++ {
		go func(id int) {
			defer wg.Done()

			// Read current value
			content, _ := os.ReadFile(testFile)
			currentLine := string(content)

			// Parse current counter (this is racy by design to test locking)
			var currentValue int
			fmt.Sscanf(currentLine, "Counter: %d", &currentValue)

			// Increment
			newValue := currentValue + 1

			args := map[string]interface{}{
				"path":     testFile,
				"old_text": currentLine,
				"new_text": fmt.Sprintf("Counter: %d", newValue),
			}

			// This will fail sometimes due to the race, but that's expected
			// The important thing is that the file doesn't get corrupted
			_ = tool.Execute(ctx, args)
		}(i)
	}

	wg.Wait()

	// Read final content - should be valid (not corrupted)
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatal(err)
	}

	// Verify content matches pattern "Counter: N"
	contentStr := string(content)
	var finalValue int
	n, err := fmt.Sscanf(contentStr, "Counter: %d", &finalValue)
	if n != 1 || err != nil {
		t.Errorf("File appears to be corrupted. Expected 'Counter: N', got: '%s'", contentStr)
	}

	// Final value should be >= 1 (at least some edits should succeed)
	if finalValue < 1 {
		t.Errorf("Expected at least one successful edit, got counter value: %d", finalValue)
	}

	// Note: We don't expect finalValue == numEdits because edits will conflict
	// The important thing is that the file is not corrupted
	t.Logf("Concurrent edits test: %d edits attempted, final counter value: %d (file integrity maintained)", numEdits, finalValue)
}

// TestEditTool_MixedConcurrentOperations verifies mixed appends and edits don't corrupt files
func TestEditTool_MixedConcurrentOperations(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "mixed_ops.txt")
	os.WriteFile(testFile, []byte("Initial content\n"), 0644)

	appendTool := NewAppendFileTool(tmpDir, true)
	editTool := NewEditFileTool(tmpDir, true)
	ctx := context.Background()

	numOps := 30
	var wg sync.WaitGroup
	wg.Add(numOps)

	// Mix of appends and edits
	for i := 0; i < numOps; i++ {
		if i%2 == 0 {
			// Append operation
			go func(id int) {
				defer wg.Done()
				args := map[string]interface{}{
					"path":    testFile,
					"content": fmt.Sprintf("Append %d\n", id),
				}
				_ = appendTool.Execute(ctx, args)
			}(i)
		} else {
			// Edit operation (try to change "Initial" to "Modified")
			go func(id int) {
				defer wg.Done()
				args := map[string]interface{}{
					"path":     testFile,
					"old_text": "Initial",
					"new_text": "Modified",
				}
				// This might fail if already modified, but shouldn't corrupt the file
				_ = editTool.Execute(ctx, args)
			}(i)
		}
	}

	wg.Wait()

	// Read final content - verify it's not corrupted
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatal(err)
	}

	contentStr := string(content)

	// Should contain either "Initial" or "Modified" (not both, not corrupted)
	hasInitial := strings.Contains(contentStr, "Initial content")
	hasModified := strings.Contains(contentStr, "Modified content")

	if !hasInitial && !hasModified {
		t.Errorf("File appears to be corrupted. Expected 'Initial' or 'Modified', got:\n%s", contentStr)
	}

	// Count append lines
	appendCount := 0
	for _, line := range strings.Split(contentStr, "\n") {
		if strings.HasPrefix(line, "Append ") {
			appendCount++
		}
	}

	if appendCount == 0 {
		t.Error("Expected at least some appends to succeed")
	}

	t.Logf("Mixed operations test: %d operations attempted, %d successful appends, file integrity maintained", numOps, appendCount)
}
