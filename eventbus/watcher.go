package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watcher monitors bind-mounted event directories for JSON event files
// written by container hook scripts. It replaces the TCP-based Listener
// with a filesystem-based approach that works regardless of host firewall
// configuration or container network mode (including air-gapped).
//
// Each container has its own subdirectory under the base directory:
//
//	<baseDir>/<containerName>/events/*.json
//
// The watcher uses two complementary strategies:
//   - fsnotify for low-latency event detection on Linux (sub-millisecond)
//   - polling every [pollInterval] as a reliable fallback for all platforms,
//     including Docker Desktop where fsnotify events may not propagate
//     across the VM boundary
type Watcher struct {
	baseDir      string
	handler      func(ContainerEvent)
	fsWatcher    *fsnotify.Watcher
	pollInterval time.Duration
	cancel       context.CancelFunc
	wg           sync.WaitGroup
}

// maxEventFileSize caps individual event files to prevent abuse (64 KB).
const maxEventFileSize = 64 * 1024

// staleTmpAge is how long a .tmp file can exist before the polling loop
// cleans it up. Orphaned .tmp files indicate a crashed write.
const staleTmpAge = 30 * time.Second

// fsnotifyDebounce is the delay before processing after an fsnotify event,
// allowing rapid events to be batched into a single processDir call.
const fsnotifyDebounce = 50 * time.Millisecond

// NewWatcher creates a Watcher that reads event files from subdirectories
// of baseDir. The handler is called for each valid event (typically
// Store.HandleEvent). The pollInterval controls how often the fallback
// polling loop scans for new files.
func NewWatcher(baseDir string, handler func(ContainerEvent), pollInterval time.Duration) *Watcher {
	return &Watcher{
		baseDir:      baseDir,
		handler:      handler,
		pollInterval: pollInterval,
	}
}

// eventDirForContainer returns the events subdirectory path for a container.
func (w *Watcher) eventDirForContainer(containerName string) string {
	return filepath.Join(w.baseDir, containerName)
}

// Start begins watching for event files. It processes any existing files
// first (crash recovery), then starts the fsnotify watcher and polling
// loop. Call Shutdown to stop.
func (w *Watcher) Start(ctx context.Context) error {
	if err := os.MkdirAll(w.baseDir, 0o755); err != nil {
		return fmt.Errorf("creating event base directory: %w", err)
	}

	// Process leftover files from a previous run (crash recovery).
	w.processAllDirs()

	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		slog.Warn("fsnotify unavailable, using polling only", "err", err)
	} else {
		w.fsWatcher = fsw
		// Watch the base directory for new container subdirectories.
		if watchErr := fsw.Add(w.baseDir); watchErr != nil {
			slog.Warn("failed to watch base directory", "err", watchErr)
		}
		// Watch existing container event directories.
		w.watchExistingDirs()
	}

	childCtx, cancel := context.WithCancel(ctx)
	w.cancel = cancel

	if w.fsWatcher != nil {
		w.wg.Add(1)
		go w.fsnotifyLoop(childCtx)
	}

	w.wg.Add(1)
	go w.pollLoop(childCtx)

	slog.Info("event watcher started", "baseDir", w.baseDir, "pollInterval", w.pollInterval)
	return nil
}

// Shutdown stops the watcher gracefully, processing any remaining files.
// The context parameter is unused — drain is handled via WaitGroup. The
// signature matches the old Listener.Shutdown for drop-in compatibility.
func (w *Watcher) Shutdown(_ context.Context) error {
	if w.cancel != nil {
		w.cancel()
	}
	w.wg.Wait()

	if w.fsWatcher != nil {
		if err := w.fsWatcher.Close(); err != nil {
			slog.Warn("fsnotify close error", "err", err)
		}
	}

	// Final sweep to catch anything written during shutdown.
	w.processAllDirs()

	slog.Info("event watcher stopped")
	return nil
}

// WatchContainerDir adds an fsnotify watch for a specific container's
// event directory. Called when a new container is created to enable
// immediate event detection without waiting for the next poll cycle.
func (w *Watcher) WatchContainerDir(containerName string) {
	if w.fsWatcher == nil {
		return
	}
	if err := w.fsWatcher.Add(w.eventDirForContainer(containerName)); err != nil {
		slog.Debug("failed to watch container event dir", "container", containerName, "err", err)
	}
}

// UnwatchContainerDir removes the fsnotify watch for a container's
// event directory. Called when a container is deleted.
func (w *Watcher) UnwatchContainerDir(containerName string) {
	if w.fsWatcher == nil {
		return
	}
	_ = w.fsWatcher.Remove(w.eventDirForContainer(containerName))
}

// CleanupContainerDir processes remaining events and removes the
// container's directory. Called when a container is deleted.
// Double-processing is safe — processFile deletes files after handling,
// so a concurrent fsnotify-triggered processDir on the same directory
// will find no files.
func (w *Watcher) CleanupContainerDir(containerName string) {
	containerDir := filepath.Join(w.baseDir, containerName)

	w.processDir(w.eventDirForContainer(containerName))
	w.UnwatchContainerDir(containerName)

	if err := os.RemoveAll(containerDir); err != nil {
		slog.Warn("failed to clean up container event dir", "dir", containerDir, "err", err)
	}
}

// watchExistingDirs adds fsnotify watches for all existing container
// event directories under the base directory.
func (w *Watcher) watchExistingDirs() {
	entries, err := os.ReadDir(w.baseDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		eventDir := w.eventDirForContainer(entry.Name())
		if info, statErr := os.Stat(eventDir); statErr == nil && info.IsDir() {
			if watchErr := w.fsWatcher.Add(eventDir); watchErr != nil {
				slog.Debug("failed to watch event dir", "dir", eventDir, "err", watchErr)
			}
		}
	}
}

// fsnotifyLoop handles fsnotify events, triggering directory processing
// on file creation. Uses a small debounce to batch rapid events.
func (w *Watcher) fsnotifyLoop(ctx context.Context) {
	defer w.wg.Done()

	timer := time.NewTimer(fsnotifyDebounce)
	timer.Stop()

	pendingDirs := make(map[string]struct{})

	for {
		select {
		case <-ctx.Done():
			timer.Stop()
			return

		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return
			}

			if event.Op&(fsnotify.Create|fsnotify.Rename) != 0 {
				dir := filepath.Dir(event.Name)

				// New subdirectory under baseDir — start watching its events/ dir.
				if dir == w.baseDir {
					eventDir := filepath.Join(event.Name, "events")
					if info, err := os.Stat(eventDir); err == nil && info.IsDir() {
						if watchErr := w.fsWatcher.Add(eventDir); watchErr == nil {
							pendingDirs[eventDir] = struct{}{}
						}
					}
					continue
				}

				// New .json file in an events directory.
				if strings.HasSuffix(event.Name, ".json") && !strings.HasPrefix(filepath.Base(event.Name), ".") {
					pendingDirs[dir] = struct{}{}
					// Drain the timer before resetting to avoid spurious ticks.
					if !timer.Stop() {
						select {
						case <-timer.C:
						default:
						}
					}
					timer.Reset(fsnotifyDebounce)
				}
			}

		case <-timer.C:
			for dir := range pendingDirs {
				w.processDir(dir)
			}
			pendingDirs = make(map[string]struct{})

		case err, ok := <-w.fsWatcher.Errors:
			if !ok {
				return
			}
			slog.Warn("fsnotify error", "err", err)
		}
	}
}

// pollLoop periodically scans all container event directories for new
// files. This is the reliable fallback that works on all platforms,
// catching any files that fsnotify may miss (e.g. on Docker Desktop).
func (w *Watcher) pollLoop(ctx context.Context) {
	defer w.wg.Done()

	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.processAllDirs()
		}
	}
}

// processAllDirs scans all container subdirectories and processes
// event files in each.
func (w *Watcher) processAllDirs() {
	entries, err := os.ReadDir(w.baseDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		w.processDir(w.eventDirForContainer(entry.Name()))
	}
}

// processDir reads and processes all .json event files in a single
// directory. Files are processed in filename-sorted order (roughly
// chronological due to epoch_ns prefix). Each file is deleted after
// successful processing. Orphaned .tmp files older than staleTmpAge
// are cleaned up.
func (w *Watcher) processDir(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	var jsonFiles []string
	now := time.Now()

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()

		// Clean up orphaned .tmp files from crashed writes.
		if strings.HasSuffix(name, ".tmp") {
			if info, infoErr := entry.Info(); infoErr == nil && now.Sub(info.ModTime()) > staleTmpAge {
				_ = os.Remove(filepath.Join(dir, name))
			}
			continue
		}

		// Only process .json files that don't start with . (hidden/tmp).
		if strings.HasSuffix(name, ".json") && !strings.HasPrefix(name, ".") {
			jsonFiles = append(jsonFiles, name)
		}
	}

	if len(jsonFiles) == 0 {
		return
	}

	sort.Strings(jsonFiles)

	for _, name := range jsonFiles {
		w.processFile(filepath.Join(dir, name))
	}
}

// processFile reads a single event file, parses it, calls the handler,
// and deletes the file. Invalid or oversized files are logged and deleted
// to prevent stuck events. Reads the file directly and checks size from
// the read data to avoid a redundant stat syscall and TOCTOU window.
func (w *Watcher) processFile(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return // File already processed or removed by a concurrent sweep.
	}

	if int64(len(data)) > maxEventFileSize {
		slog.Warn("event file too large, skipping", "path", path, "size", len(data))
		_ = os.Remove(path)
		return
	}

	var event ContainerEvent
	if err := json.Unmarshal(data, &event); err != nil {
		slog.Warn("invalid JSON in event file", "path", path, "err", err)
		_ = os.Remove(path)
		return
	}

	if event.Type == "" || event.ContainerName == "" {
		slog.Warn("event file missing required fields", "path", path)
		_ = os.Remove(path)
		return
	}

	// Use the timestamp from the event payload (set by the container).
	// Fall back to current time if not provided.
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	slog.Debug("event received",
		"type", event.Type,
		"container", event.ContainerName,
		"worktree", event.WorktreeID,
	)

	event.Source = SourceEventDir
	w.handler(event)

	if err := os.Remove(path); err != nil {
		slog.Warn("failed to delete processed event file", "path", path, "err", err)
	}
}
