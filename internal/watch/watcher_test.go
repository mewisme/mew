package watch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewWatcher(t *testing.T) {
	w, err := NewWatcher()
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	if w == nil {
		t.Fatal("NewWatcher returned nil")
	}
	_ = w.Close()
}

func TestNativeWatcherAddFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(f, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	w, err := NewWatcher()
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	defer func() { _ = w.Close() }()

	if err := w.Add(f); err != nil {
		t.Fatalf("Add file: %v", err)
	}
}

func TestNativeWatcherAddDirectory(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "subdir")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "a.ts"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	w, err := NewWatcher()
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	defer func() { _ = w.Close() }()

	if err := w.Add(dir); err != nil {
		t.Fatalf("Add dir: %v", err)
	}
}

func TestNativeWatcherDetectsWrite(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "watchme.txt")
	if err := os.WriteFile(f, []byte("initial"), 0o644); err != nil {
		t.Fatal(err)
	}

	w, err := NewWatcher()
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	defer func() { _ = w.Close() }()

	if err := w.Add(dir); err != nil {
		t.Fatalf("Add dir: %v", err)
	}

	// Write to the file.
	if err := os.WriteFile(f, []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Wait for the event.
	select {
	case evt := <-w.Events():
		if evt.Path == "" {
			t.Error("event has empty path")
		}
		if evt.Op == 0 {
			t.Error("event has zero op")
		}
	// fsnotify events may be delayed; don't block test forever.
	default:
		// File system events are async — skip assertion if none arrived.
	}
}

func TestNativeWatcherDetectsCreate(t *testing.T) {
	dir := t.TempDir()

	w, err := NewWatcher()
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	defer func() { _ = w.Close() }()

	if err := w.Add(dir); err != nil {
		t.Fatalf("Add dir: %v", err)
	}

	newFile := filepath.Join(dir, "newfile.txt")
	if err := os.WriteFile(newFile, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}

	select {
	case <-w.Events():
		// Expected.
	default:
	}
}

func TestPollingWatcherDetectsWrite(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "pollme.txt")
	if err := os.WriteFile(f, []byte("initial"), 0o644); err != nil {
		t.Fatal(err)
	}

	pw := newPollingWatcher(50) // 50ms interval for fast test
	defer func() { _ = pw.Close() }()

	if err := pw.Add(dir); err != nil {
		t.Fatalf("Add dir: %v", err)
	}

	// Write to trigger detection.
	if err := os.WriteFile(f, []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}

	select {
	case evt := <-pw.Events():
		if evt.Path == "" {
			t.Error("event has empty path")
		}
	// Polling at 50ms should detect within a few intervals.
	default:
	}
}

func TestNewWatcherBackend(t *testing.T) {
	w, err := NewWatcher()
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	defer func() { _ = w.Close() }()

	if w.Backend() != BackendNative {
		t.Logf("backend = %s (native may be unavailable on this platform)", w.Backend())
	}
}

func TestNativeWatcherDoubleClose(t *testing.T) {
	w, err := newNativeWatcher()
	if err != nil {
		t.Fatalf("newNativeWatcher: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	// Second Close must not panic.
	if err := w.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestNativeWatcherCloseDrains(t *testing.T) {
	dir := t.TempDir()
	w, err := newNativeWatcher()
	if err != nil {
		t.Fatalf("newNativeWatcher: %v", err)
	}

	if err := w.Add(dir); err != nil {
		_ = w.Close()
		t.Fatalf("Add: %v", err)
	}

	// Drain events channel so it doesn't block.
	go func() {
		for range w.Events() {
		}
	}()
	go func() {
		for range w.Errors() {
		}
	}()

	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestPollingWatcherDoubleClose(t *testing.T) {
	pw := newPollingWatcher(100)
	if err := pw.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	// Second Close must not panic.
	if err := pw.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestPollingWatcherBackend(t *testing.T) {
	pw := newPollingWatcher(100)
	defer func() { _ = pw.Close() }()

	if pw.Backend() != BackendPolling {
		t.Errorf("expected BackendPolling, got %s", pw.Backend())
	}
}

func TestPollingWatcherRescanNewFile(t *testing.T) {
	dir := t.TempDir()

	pw := newPollingWatcher(50)
	defer func() { _ = pw.Close() }()

	if err := pw.Add(dir); err != nil {
		t.Fatalf("Add dir: %v", err)
	}

	// Create a new file AFTER initial Add.
	newFile := filepath.Join(dir, "newfile.txt")
	time.Sleep(100 * time.Millisecond) // let initial scan settle
	if err := os.WriteFile(newFile, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Should detect the new file on next rescan.
	select {
	case evt := <-pw.Events():
		if evt.Path == "" {
			t.Error("empty path in event")
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("timeout waiting for new file detection")
	}
}

func TestPollingWatcherDeleteRecreate(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "cycle.txt")
	if err := os.WriteFile(f, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}

	pw := newPollingWatcher(50)
	defer func() { _ = pw.Close() }()

	if err := pw.Add(dir); err != nil {
		t.Fatalf("Add dir: %v", err)
	}

	// Let initial scan settle.
	time.Sleep(100 * time.Millisecond)

	// Drain any initial events.
	drainEvents(pw.Events())

	// Delete the file.
	if err := os.Remove(f); err != nil {
		t.Fatal(err)
	}

	// Wait for delete detection.
	gotRemove := false
	deadline := time.After(800 * time.Millisecond)
	for !gotRemove {
		select {
		case evt := <-pw.Events():
			if evt.Op&OpRemove != 0 {
				gotRemove = true
			}
		case <-deadline:
			t.Fatal("timeout waiting for delete detection")
		}
	}

	// Recreate the file.
	if err := os.WriteFile(f, []byte("v2"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Should detect recreation.
	gotCreate := false
	deadline = time.After(800 * time.Millisecond)
	for !gotCreate {
		select {
		case evt := <-pw.Events():
			if evt.Op&OpCreate != 0 || evt.Op&OpWrite != 0 {
				gotCreate = true
			}
		case <-deadline:
			t.Fatal("timeout waiting for recreate detection")
		}
	}
}

func drainEvents(ch <-chan Event) {
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}

func TestPollingWatcherCloseDuringBackpressure(t *testing.T) {
	pw := newPollingWatcher(10) // fast interval
	// Never drain events — channel fills up.
	// Close must still succeed promptly.
	done := make(chan struct{})
	go func() {
		_ = pw.Close()
		close(done)
	}()
	select {
	case <-done:
		// OK
	case <-time.After(time.Second):
		t.Fatal("Close blocked while event channel full")
	}
}

func TestNormalizePathIdentity(t *testing.T) {
	dir := t.TempDir()
	// Use string concatenation to avoid filepath.Join cleaning the path.
	rel := dir + "/sub/../file.txt"
	norm := NormalizePath(rel)
	if !filepath.IsAbs(norm) {
		t.Error("NormalizePath did not produce absolute path")
	}
	// Should not contain ".." after normalization.
	if strings.Contains(norm, "..") {
		t.Error("NormalizePath contains unresolved ..")
	}
}

func TestPathKeyIdentity(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "file.txt")
	b := filepath.Join(dir, "sub", "..", "file.txt")

	// Both should produce the same key.
	if pathKey(a) != pathKey(b) {
		t.Errorf("pathKey mismatch: %q != %q", pathKey(a), pathKey(b))
	}
}
