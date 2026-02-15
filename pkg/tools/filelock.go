// PicoClaw - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 PicoClaw contributors

package tools

import (
	"path/filepath"
	"sync"
)

// FileLockManager provides application-level file locking to prevent
// concurrent file modifications that could cause corruption.
//
// This uses in-process mutex-based locking rather than OS-level file locks
// because:
// - Cross-platform compatible (no platform-specific syscalls)
// - Lightweight (perfect for PicoClaw's minimal resource requirements)
// - Simple implementation (no lock release on crash handling needed)
// - Addresses the primary issue (intra-process concurrency between tool calls)
type FileLockManager struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

// newFileLockManager creates a new FileLockManager.
func newFileLockManager() *FileLockManager {
	return &FileLockManager{
		locks: make(map[string]*sync.Mutex),
	}
}

// getLock returns the mutex for the given file path, creating it if needed.
// Normalizes the path to absolute form for consistent locking.
func (flm *FileLockManager) getLock(path string) *sync.Mutex {
	// Normalize path to absolute form
	absPath, err := filepath.Abs(path)
	if err != nil {
		// If we can't resolve the absolute path, use the path as-is
		absPath = filepath.Clean(path)
	}

	flm.mu.Lock()
	defer flm.mu.Unlock()

	lock, exists := flm.locks[absPath]
	if !exists {
		lock = &sync.Mutex{}
		flm.locks[absPath] = lock
	}

	return lock
}

// Lock acquires a lock on the given file path.
// The caller must call Unlock when done.
func (flm *FileLockManager) Lock(path string) {
	lock := flm.getLock(path)
	lock.Lock()
}

// Unlock releases the lock on the given file path.
func (flm *FileLockManager) Unlock(path string) {
	lock := flm.getLock(path)
	lock.Unlock()
}

// WithLock executes the given function while holding a lock on the file path.
// This is the recommended way to use the file lock manager as it ensures
// proper lock release even if the function panics.
func (flm *FileLockManager) WithLock(path string, fn func() error) error {
	flm.Lock(path)
	defer flm.Unlock(path)
	return fn()
}

// Global file lock manager instance
var globalFileLockManager = newFileLockManager()

// GetGlobalFileLockManager returns the global file lock manager instance.
// All file operations should use this instance to coordinate access.
func GetGlobalFileLockManager() *FileLockManager {
	return globalFileLockManager
}
