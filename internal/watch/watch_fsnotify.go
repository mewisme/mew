package watch

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
)

type nativeWatcher struct {
	w      *fsnotify.Watcher
	events chan Event
	errs   chan error
	done   chan struct{}

	mu sync.Mutex
	wg sync.WaitGroup

	closed bool
}

func newNativeWatcher() (*nativeWatcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	nw := &nativeWatcher{
		w:      w,
		events: make(chan Event, 256),
		errs:   make(chan error, 16),
		done:   make(chan struct{}),
	}
	nw.wg.Add(1)
	go nw.loop()
	return nw, nil
}

func (nw *nativeWatcher) Backend() Backend { return BackendNative }

func (nw *nativeWatcher) Add(path string) error {
	nw.mu.Lock()
	if nw.closed {
		nw.mu.Unlock()
		return ErrWatcherClosed
	}
	nw.mu.Unlock()

	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return nw.addDir(path)
	}
	return nw.w.Add(path)
}

func (nw *nativeWatcher) addDir(dir string) error {
	if err := nw.w.Add(dir); err != nil {
		return err
	}
	// Walk subdirectories, skipping hidden dirs and noise trees.
	return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip inaccessible entries
		}
		if !d.IsDir() {
			return nil
		}
		if path == dir {
			return nil // already added above
		}
		base := filepath.Base(path)
		if shouldSkipDir(base) {
			return filepath.SkipDir
		}
		if err := nw.w.Add(path); err != nil {
			return nil // non-fatal: permission errors, etc.
		}
		return nil
	})
}

func (nw *nativeWatcher) Remove(path string) error {
	nw.mu.Lock()
	if nw.closed {
		nw.mu.Unlock()
		return ErrWatcherClosed
	}
	nw.mu.Unlock()
	return nw.w.Remove(path)
}

func (nw *nativeWatcher) Events() <-chan Event { return nw.events }
func (nw *nativeWatcher) Errors() <-chan error { return nw.errs }

func (nw *nativeWatcher) Close() error {
	nw.mu.Lock()
	if nw.closed {
		nw.mu.Unlock()
		return nil
	}
	nw.closed = true
	nw.mu.Unlock()
	close(nw.done)
	nw.wg.Wait() // wait for loop goroutine to terminate
	return nw.w.Close()
}

func (nw *nativeWatcher) loop() {
	defer nw.wg.Done()
	defer close(nw.events)
	defer close(nw.errs)

	for {
		select {
		case <-nw.done:
			return
		case evt, ok := <-nw.w.Events:
			if !ok {
				return
			}
			op := toOp(evt.Op)
			// Skip CHMOD-only events on directories; these are noise.
			if evt.Has(fsnotify.Chmod) && isDir(evt.Name) {
				continue
			}
			path := normalizePath(evt.Name)

			// When a new directory is created or moved into a watched
			// dir, recursively register its existing subtree.
			if (evt.Has(fsnotify.Create) || evt.Has(fsnotify.Rename)) && isDir(evt.Name) {
				base := filepath.Base(evt.Name)
				if !shouldSkipDir(base) {
					_ = nw.addDir(evt.Name)
				}
			}

			nw.sendEvent(Event{Path: path, Op: op})
		case err, ok := <-nw.w.Errors:
			if !ok {
				return
			}
			nw.sendError(err)
		}
	}
}

// sendEvent delivers an event without blocking shutdown.
func (nw *nativeWatcher) sendEvent(evt Event) {
	select {
	case nw.events <- evt:
	case <-nw.done:
	}
}

// sendError delivers an error without blocking shutdown.
func (nw *nativeWatcher) sendError(err error) {
	select {
	case nw.errs <- err:
	case <-nw.done:
	}
}

func toOp(op fsnotify.Op) Op {
	var o Op
	if op&fsnotify.Create != 0 {
		o |= OpCreate
	}
	if op&fsnotify.Write != 0 {
		o |= OpWrite
	}
	if op&fsnotify.Remove != 0 {
		o |= OpRemove
	}
	if op&fsnotify.Rename != 0 {
		o |= OpRename
	}
	return o
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// shouldSkipDir returns true when a directory name matches the exclusion
// policy shared by both watcher backends.
func shouldSkipDir(name string) bool {
	if isHiddenDir(name) {
		return true
	}
	return segmentSkipped(name)
}
