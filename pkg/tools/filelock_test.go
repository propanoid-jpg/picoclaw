// PicoClaw - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 PicoClaw contributors

package tools

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestFileLockManager_BasicLocking(t *testing.T) {
	lockMgr := newFileLockManager()
	testPath := "/tmp/test.txt"

	// Lock and unlock should work without error
	lockMgr.Lock(testPath)
	lockMgr.Unlock(testPath)
}

func TestFileLockManager_WithLock(t *testing.T) {
	lockMgr := newFileLockManager()
	testPath := "/tmp/test.txt"

	executed := false
	err := lockMgr.WithLock(testPath, func() error {
		executed = true
		return nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if !executed {
		t.Error("Expected function to be executed")
	}
}

func TestFileLockManager_ConcurrentWrites(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "concurrent_test.txt")
	lockMgr := GetGlobalFileLockManager()

	// Number of concurrent writers
	numWriters := 10
	writesPerWriter := 100

	var wg sync.WaitGroup
	wg.Add(numWriters)

	// Each goroutine appends its ID multiple times
	for i := 0; i < numWriters; i++ {
		go func(writerID int) {
			defer wg.Done()
			for j := 0; j < writesPerWriter; j++ {
				err := lockMgr.WithLock(testFile, func() error {
					// Read current content
					content := ""
					if data, err := os.ReadFile(testFile); err == nil {
						content = string(data)
					}

					// Append writer ID
					content += "W"
					time.Sleep(1 * time.Microsecond) // Small delay to increase chance of collision

					// Write back
					return os.WriteFile(testFile, []byte(content), 0644)
				})
				if err != nil {
					t.Errorf("Write error: %v", err)
				}
			}
		}(i)
	}

	wg.Wait()

	// Read final content
	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatal(err)
	}

	// Should have exactly numWriters * writesPerWriter 'W' characters
	expectedLen := numWriters * writesPerWriter
	if len(data) != expectedLen {
		t.Errorf("Expected %d characters, got %d. File locking may not be working correctly.", expectedLen, len(data))
	}
}

func TestFileLockManager_PathNormalization(t *testing.T) {
	tmpDir := t.TempDir()
	lockMgr := newFileLockManager()

	// Create test file
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Different path representations should use the same lock
	relativePath := "test.txt"
	absolutePath := testFile

	// Both should get the same lock (we can't directly test this,
	// but we can verify they both work without deadlocking)
	err1 := lockMgr.WithLock(absolutePath, func() error {
		// Try to get lock with different path format in a goroutine
		// If they use different locks, this won't block
		// If they use the same lock, this should be serialized
		return nil
	})

	err2 := lockMgr.WithLock(relativePath, func() error {
		return nil
	})

	if err1 != nil {
		t.Errorf("Error with absolute path: %v", err1)
	}
	if err2 != nil {
		t.Errorf("Error with relative path: %v", err2)
	}
}

func TestFileLockManager_MultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()
	lockMgr := GetGlobalFileLockManager()

	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")

	// Should be able to lock different files concurrently
	var wg sync.WaitGroup
	wg.Add(2)

	start := make(chan bool)

	// Writer for file1
	go func() {
		defer wg.Done()
		<-start
		lockMgr.WithLock(file1, func() error {
			time.Sleep(10 * time.Millisecond)
			return os.WriteFile(file1, []byte("file1"), 0644)
		})
	}()

	// Writer for file2
	go func() {
		defer wg.Done()
		<-start
		lockMgr.WithLock(file2, func() error {
			time.Sleep(10 * time.Millisecond)
			return os.WriteFile(file2, []byte("file2"), 0644)
		})
	}()

	// Start both goroutines at the same time
	startTime := time.Now()
	close(start)
	wg.Wait()
	duration := time.Since(startTime)

	// Both should run concurrently (different files)
	// If they were serialized, it would take ~20ms
	// If concurrent, it should take ~10ms
	if duration > 15*time.Millisecond {
		t.Logf("Warning: Operations may have been serialized (took %v). Different files should be lockable concurrently.", duration)
	}

	// Verify both files were written
	if data, err := os.ReadFile(file1); err != nil || string(data) != "file1" {
		t.Error("file1 not written correctly")
	}
	if data, err := os.ReadFile(file2); err != nil || string(data) != "file2" {
		t.Error("file2 not written correctly")
	}
}

func TestFileLockManager_SameFileSerialized(t *testing.T) {
	tmpDir := t.TempDir()
	lockMgr := GetGlobalFileLockManager()
	testFile := filepath.Join(tmpDir, "test.txt")

	var wg sync.WaitGroup
	wg.Add(2)

	start := make(chan bool)

	writeOrder := make([]int, 0, 2)
	var orderMu sync.Mutex

	// First writer
	go func() {
		defer wg.Done()
		<-start
		lockMgr.WithLock(testFile, func() error {
			time.Sleep(10 * time.Millisecond)
			orderMu.Lock()
			writeOrder = append(writeOrder, 1)
			orderMu.Unlock()
			return os.WriteFile(testFile, []byte("writer1"), 0644)
		})
	}()

	// Second writer
	go func() {
		defer wg.Done()
		<-start
		lockMgr.WithLock(testFile, func() error {
			time.Sleep(10 * time.Millisecond)
			orderMu.Lock()
			writeOrder = append(writeOrder, 2)
			orderMu.Unlock()
			return os.WriteFile(testFile, []byte("writer2"), 0644)
		})
	}()

	// Start both goroutines at the same time
	startTime := time.Now()
	close(start)
	wg.Wait()
	duration := time.Since(startTime)

	// Same file should be serialized, taking ~20ms
	if duration < 18*time.Millisecond {
		t.Errorf("Operations should be serialized for same file (took %v, expected ~20ms)", duration)
	}

	// Verify writes were serialized (should have both entries)
	if len(writeOrder) != 2 {
		t.Errorf("Expected 2 writes, got %d", len(writeOrder))
	}
}

func TestGlobalFileLockManager(t *testing.T) {
	// Verify GetGlobalFileLockManager returns the same instance
	mgr1 := GetGlobalFileLockManager()
	mgr2 := GetGlobalFileLockManager()

	if mgr1 != mgr2 {
		t.Error("GetGlobalFileLockManager should return the same instance")
	}
}
