package watcher

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/devsync/dev-sync/internal/ignore"
	"github.com/devsync/dev-sync/internal/logging"
	"github.com/devsync/dev-sync/internal/safety"
)

// WatchCallback is invoked after events are debounced and coalesced.
type WatchCallback func(changedPaths []string)

// Watcher manages recursive filesystem watching with debouncing and editor-save coalescing.
type Watcher struct {
	sourceRoot string
	matcher    *ignore.IgnoreMatcher
	debounceMs time.Duration
	fsWatcher  *fsnotify.Watcher
	mu         sync.Mutex
	pending    map[string]time.Time
	stopCh     chan struct{}
	doneCh     chan struct{}
}

// NewWatcher creates a recursive directory watcher.
func NewWatcher(sourceRoot string, matcher *ignore.IgnoreMatcher, debounceMs int) (*Watcher, error) {
	normSource, err := safety.NormalizePath(sourceRoot)
	if err != nil {
		return nil, err
	}

	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize fsnotify watcher: %w", err)
	}

	if debounceMs <= 0 {
		debounceMs = 150
	}

	w := &Watcher{
		sourceRoot: normSource,
		matcher:    matcher,
		debounceMs: time.Duration(debounceMs) * time.Millisecond,
		fsWatcher:  fsw,
		pending:    make(map[string]time.Time),
		stopCh:     make(chan struct{}),
		doneCh:     make(chan struct{}),
	}

	// Recursively add all non-ignored directories to watcher
	if err := w.watchRecursive(normSource); err != nil {
		_ = fsw.Close()
		return nil, err
	}

	return w, nil
}

func (w *Watcher) watchRecursive(root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(w.sourceRoot, path)
		if err != nil {
			return err
		}
		relNorm := filepath.ToSlash(rel)

		// Check if directory is ignored
		if w.matcher != nil && w.matcher.ShouldIgnore(relNorm, true) {
			return filepath.SkipDir
		}

		if err := w.fsWatcher.Add(path); err != nil {
			logging.Debug("failed to add watch on %s: %v", path, err)
		}
		return nil
	})
}

// Start begins the event processing loop.
func (w *Watcher) Start(ctx context.Context, callback WatchCallback) {
	go func() {
		defer close(w.doneCh)
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-w.stopCh:
				return

			case event, ok := <-w.fsWatcher.Events:
				if !ok {
					return
				}
				w.handleFSEvent(event)

			case err, ok := <-w.fsWatcher.Errors:
				if !ok {
					return
				}
				logging.Warn("Filesystem watcher error: %v", err)

			case <-ticker.C:
				w.flushDebounced(callback)
			}
		}
	}()
}

func (w *Watcher) handleFSEvent(event fsnotify.Event) {
	rel, err := filepath.Rel(w.sourceRoot, event.Name)
	if err != nil {
		return
	}
	relNorm := filepath.ToSlash(rel)

	// Check if ignored
	isDir := false
	if fi, err := os.Stat(event.Name); err == nil {
		isDir = fi.IsDir()
		// If a new directory was created, dynamically add it to the watcher
		if isDir && (event.Has(fsnotify.Create)) {
			if w.matcher == nil || !w.matcher.ShouldIgnore(relNorm, true) {
				_ = w.fsWatcher.Add(event.Name)
				logging.Debug("Added watch for new directory: %s", relNorm)
			}
		}
	}

	if w.matcher != nil && w.matcher.ShouldIgnore(relNorm, isDir) {
		return
	}

	w.mu.Lock()
	w.pending[relNorm] = time.Now()
	w.mu.Unlock()
}

func (w *Watcher) flushDebounced(callback WatchCallback) {
	w.mu.Lock()
	now := time.Now()
	var ready []string

	for path, lastSeen := range w.pending {
		if now.Sub(lastSeen) >= w.debounceMs {
			ready = append(ready, path)
			delete(w.pending, path)
		}
	}
	w.mu.Unlock()

	if len(ready) > 0 && callback != nil {
		callback(ready)
	}
}

// Stop terminates the watcher cleanly.
func (w *Watcher) Stop() {
	select {
	case <-w.stopCh:
		return
	default:
		close(w.stopCh)
	}
	_ = w.fsWatcher.Close()
	<-w.doneCh
}
