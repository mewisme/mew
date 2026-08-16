// Package watch provides file watching and a restart supervisor.
package watch

import (
	"errors"
	"path/filepath"
	"strings"
)

// ErrWatcherClosed is returned by Add/Remove when the watcher is already closed.
var ErrWatcherClosed = errors.New("watch: watcher is closed")

// Op describes a file operation.
type Op uint32

const (
	OpCreate Op = 1 << iota
	OpWrite
	OpRemove
	OpRename
)

// Event is a file change notification.
type Event struct {
	Path string
	Op   Op
}

// Backend identifies the active watcher implementation.
type Backend string

const (
	BackendNative  Backend = "native"
	BackendPolling Backend = "polling"
)

// Watcher watches files and directories for changes.
type Watcher interface {
	// Add starts watching a path (file or directory, recursively).
	Add(path string) error
	// Remove stops watching a path.
	Remove(path string) error
	// Events returns the event channel.
	Events() <-chan Event
	// Errors returns the error channel (non-fatal errors).
	Errors() <-chan error
	// Close stops watching and releases resources. Idempotent and
	// concurrency-safe. Must not be called concurrently with Add/Remove.
	Close() error
	// Backend returns the active backend implementation.
	Backend() Backend
}

// NewWatcher creates the best available watcher for the current platform.
// Prefers native (fsnotify) when available and operational. Falls back to
// polling when native initialization fails.
func NewWatcher() (Watcher, error) {
	nw, err := newNativeWatcher()
	if err == nil {
		return nw, nil
	}
	return newPollingWatcher(defaultPollInterval), nil
}

// NormalizePath returns the canonical absolute path for p,
// resolving symlinks when possible. It is the single canonical
// path-identity function for the watch graph and event pipeline.
func NormalizePath(p string) string {
	return normalizePath(p)
}

func isHiddenDir(name string) bool {
	return strings.HasPrefix(name, ".") && name != "." && name != ".."
}

func normalizePath(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return filepath.Clean(p)
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		return real
	}
	return abs
}
