package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// writeEventFile writes a JSON event file atomically using the same
// pattern as the container-side warden-write-event.sh script.
func writeEventFile(t *testing.T, dir string, event ContainerEvent) string {
	t.Helper()

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}

	name := fmt.Sprintf("%d-%d.json", time.Now().UnixNano(), os.Getpid())
	tmpPath := filepath.Join(dir, "."+name+".tmp")
	finalPath := filepath.Join(dir, name)

	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		t.Fatalf("write tmp file: %v", err)
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		t.Fatalf("rename to final: %v", err)
	}

	return finalPath
}

// setupTestWatcher creates a temporary directory structure and a watcher
// with a short poll interval for testing.
func setupTestWatcher(t *testing.T) (string, string, *Watcher, *eventCollector) {
	t.Helper()

	baseDir := t.TempDir()
	containerName := "test-container"
	eventDir := filepath.Join(baseDir, containerName)

	if err := os.MkdirAll(eventDir, 0o777); err != nil {
		t.Fatalf("create event dir: %v", err)
	}

	collector := &eventCollector{}
	w := NewWatcher(baseDir, collector.handle, 100*time.Millisecond)

	return baseDir, eventDir, w, collector
}

// eventCollector collects events for assertions in tests.
type eventCollector struct {
	mu     sync.Mutex
	events []ContainerEvent
}

func (c *eventCollector) handle(event ContainerEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, event)
}

func (c *eventCollector) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.events)
}

func (c *eventCollector) get(i int) ContainerEvent {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.events[i]
}

func (c *eventCollector) waitFor(t *testing.T, n int, timeout time.Duration) {
	t.Helper()
	deadline := time.After(timeout)
	for {
		if c.count() >= n {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for %d events (got %d)", n, c.count())
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func TestWatcher_ProcessesAtomicWrite(t *testing.T) {
	t.Parallel()

	_, eventDir, w, collector := setupTestWatcher(t)

	if err := w.Start(context.Background()); err != nil {
		t.Fatalf("start watcher: %v", err)
	}
	defer w.Shutdown(context.Background()) //nolint:errcheck

	writeEventFile(t, eventDir, ContainerEvent{
		Type:          EventHeartbeat,
		ContainerName: "test-container",
		Timestamp:     time.Now(),
	})

	collector.waitFor(t, 1, 3*time.Second)

	got := collector.get(0)
	if got.Type != EventHeartbeat {
		t.Errorf("got type %q, want %q", got.Type, EventHeartbeat)
	}
	if got.ContainerName != "test-container" {
		t.Errorf("got container %q, want %q", got.ContainerName, "test-container")
	}
}

func TestWatcher_IgnoresTmpFiles(t *testing.T) {
	t.Parallel()

	_, eventDir, w, collector := setupTestWatcher(t)

	// Write a .tmp file (simulates in-progress write).
	tmpPath := filepath.Join(eventDir, ".12345-99.json.tmp")
	data := []byte(`{"type":"heartbeat","containerName":"test"}`)
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		t.Fatalf("write tmp: %v", err)
	}

	if err := w.Start(context.Background()); err != nil {
		t.Fatalf("start watcher: %v", err)
	}
	defer w.Shutdown(context.Background()) //nolint:errcheck

	// Wait long enough for a poll cycle.
	time.Sleep(300 * time.Millisecond)

	if collector.count() != 0 {
		t.Errorf("expected 0 events, got %d", collector.count())
	}
}

func TestWatcher_DeletesInvalidJSON(t *testing.T) {
	t.Parallel()

	_, eventDir, w, _ := setupTestWatcher(t)

	// Write an invalid JSON file.
	badPath := filepath.Join(eventDir, "99999-1.json")
	if err := os.WriteFile(badPath, []byte("not json"), 0o644); err != nil {
		t.Fatalf("write bad file: %v", err)
	}

	if err := w.Start(context.Background()); err != nil {
		t.Fatalf("start watcher: %v", err)
	}
	defer w.Shutdown(context.Background()) //nolint:errcheck

	// Wait for processing.
	time.Sleep(300 * time.Millisecond)

	// File should be deleted.
	if _, err := os.Stat(badPath); !os.IsNotExist(err) {
		t.Errorf("invalid JSON file should have been deleted")
	}
}

func TestWatcher_ProcessExistingFiles(t *testing.T) {
	t.Parallel()

	_, eventDir, w, collector := setupTestWatcher(t)

	// Write files BEFORE starting the watcher (crash recovery).
	for i := 0; i < 3; i++ {
		writeEventFile(t, eventDir, ContainerEvent{
			Type:          EventHeartbeat,
			ContainerName: "test-container",
			Timestamp:     time.Now(),
		})
		time.Sleep(time.Millisecond) // Ensure unique filenames.
	}

	if err := w.Start(context.Background()); err != nil {
		t.Fatalf("start watcher: %v", err)
	}
	defer w.Shutdown(context.Background()) //nolint:errcheck

	collector.waitFor(t, 3, 3*time.Second)

	if collector.count() != 3 {
		t.Errorf("expected 3 events, got %d", collector.count())
	}
}

func TestWatcher_ConcurrentContainerDirs(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	collector := &eventCollector{}
	w := NewWatcher(baseDir, collector.handle, 100*time.Millisecond)

	// Create two container directories.
	for _, name := range []string{"container-a", "container-b"} {
		dir := filepath.Join(baseDir, name)
		if err := os.MkdirAll(dir, 0o777); err != nil {
			t.Fatalf("create dir: %v", err)
		}
	}

	if err := w.Start(context.Background()); err != nil {
		t.Fatalf("start watcher: %v", err)
	}
	defer w.Shutdown(context.Background()) //nolint:errcheck

	// Write to both directories.
	writeEventFile(t, filepath.Join(baseDir, "container-a"), ContainerEvent{
		Type:          EventSessionStart,
		ContainerName: "container-a",
		Timestamp:     time.Now(),
	})
	writeEventFile(t, filepath.Join(baseDir, "container-b"), ContainerEvent{
		Type:          EventSessionEnd,
		ContainerName: "container-b",
		Timestamp:     time.Now(),
	})

	collector.waitFor(t, 2, 3*time.Second)

	if collector.count() != 2 {
		t.Errorf("expected 2 events, got %d", collector.count())
	}
}

func TestWatcher_CleanupContainerDir(t *testing.T) {
	t.Parallel()

	baseDir, eventDir, w, collector := setupTestWatcher(t)

	if err := w.Start(context.Background()); err != nil {
		t.Fatalf("start watcher: %v", err)
	}
	defer w.Shutdown(context.Background()) //nolint:errcheck

	// Write an event, then clean up.
	writeEventFile(t, eventDir, ContainerEvent{
		Type:          EventHeartbeat,
		ContainerName: "test-container",
		Timestamp:     time.Now(),
	})

	// Give the watcher a moment to process.
	time.Sleep(300 * time.Millisecond)

	w.CleanupContainerDir("test-container")

	// Directory should be removed.
	containerDir := filepath.Join(baseDir, "test-container")
	if _, err := os.Stat(containerDir); !os.IsNotExist(err) {
		t.Errorf("container directory should have been removed")
	}

	// Event should have been processed.
	if collector.count() < 1 {
		t.Errorf("expected at least 1 event, got %d", collector.count())
	}
}

func TestWatcher_FallbackTimestamp(t *testing.T) {
	t.Parallel()

	_, eventDir, w, collector := setupTestWatcher(t)

	if err := w.Start(context.Background()); err != nil {
		t.Fatalf("start watcher: %v", err)
	}
	defer w.Shutdown(context.Background()) //nolint:errcheck

	// Write event without a timestamp — watcher should assign one.
	writeEventFile(t, eventDir, ContainerEvent{
		Type:          EventHeartbeat,
		ContainerName: "test-container",
	})

	collector.waitFor(t, 1, 3*time.Second)

	got := collector.get(0)
	if got.Timestamp.IsZero() {
		t.Errorf("expected non-zero timestamp for event without timestamp")
	}
}

func TestWatcher_OversizedFileDeleted(t *testing.T) {
	t.Parallel()

	_, eventDir, w, collector := setupTestWatcher(t)

	// Write a file larger than maxEventFileSize (64 KB).
	path := filepath.Join(eventDir, "99999-1.json")
	bigData := make([]byte, maxEventFileSize+1)
	for i := range bigData {
		bigData[i] = 'x'
	}
	if err := os.WriteFile(path, bigData, 0o644); err != nil {
		t.Fatalf("write oversized file: %v", err)
	}

	if err := w.Start(context.Background()); err != nil {
		t.Fatalf("start watcher: %v", err)
	}
	defer w.Shutdown(context.Background()) //nolint:errcheck

	time.Sleep(300 * time.Millisecond)

	if collector.count() != 0 {
		t.Errorf("expected 0 events for oversized file, got %d", collector.count())
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("oversized file should have been deleted")
	}
}

func TestWatcher_CleanupDrainsUnprocessedEvents(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	containerName := "drain-test"
	eventDir := filepath.Join(baseDir, containerName)

	if err := os.MkdirAll(eventDir, 0o777); err != nil {
		t.Fatalf("create event dir: %v", err)
	}

	collector := &eventCollector{}
	// Use a very long poll interval so polling doesn't process the file.
	w := NewWatcher(baseDir, collector.handle, 1*time.Hour)

	if err := w.Start(context.Background()); err != nil {
		t.Fatalf("start watcher: %v", err)
	}
	defer w.Shutdown(context.Background()) //nolint:errcheck

	// Write event — with 1h poll interval, only CleanupContainerDir
	// or fsnotify will process it.
	writeEventFile(t, eventDir, ContainerEvent{
		Type:          EventSessionStart,
		ContainerName: containerName,
		Timestamp:     time.Now(),
	})

	// CleanupContainerDir should drain the unprocessed event.
	w.CleanupContainerDir(containerName)

	if collector.count() != 1 {
		t.Errorf("expected 1 event drained by cleanup, got %d", collector.count())
	}

	// Directory should be removed.
	if _, err := os.Stat(filepath.Join(baseDir, containerName)); !os.IsNotExist(err) {
		t.Errorf("container directory should have been removed")
	}
}

func TestWatcher_ShutdownWithoutEvents(t *testing.T) {
	t.Parallel()

	_, _, w, _ := setupTestWatcher(t)

	if err := w.Start(context.Background()); err != nil {
		t.Fatalf("start watcher: %v", err)
	}

	// Shutdown immediately — should not deadlock or panic.
	if err := w.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown error: %v", err)
	}
}

func TestWatcher_WatchAndUnwatchContainerDir(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	containerName := "watch-test"
	eventDir := filepath.Join(baseDir, containerName)

	if err := os.MkdirAll(eventDir, 0o777); err != nil {
		t.Fatalf("create event dir: %v", err)
	}

	collector := &eventCollector{}
	// Long poll interval — events should arrive via fsnotify only.
	w := NewWatcher(baseDir, collector.handle, 1*time.Hour)

	if err := w.Start(context.Background()); err != nil {
		t.Fatalf("start watcher: %v", err)
	}
	defer w.Shutdown(context.Background()) //nolint:errcheck

	// Manually register the container directory for fsnotify.
	w.WatchContainerDir(containerName)

	writeEventFile(t, eventDir, ContainerEvent{
		Type:          EventHeartbeat,
		ContainerName: containerName,
		Timestamp:     time.Now(),
	})

	// Should arrive via fsnotify fast path (not polling).
	collector.waitFor(t, 1, 3*time.Second)

	if collector.get(0).ContainerName != containerName {
		t.Errorf("got container %q, want %q", collector.get(0).ContainerName, containerName)
	}

	// Unwatch — should not panic.
	w.UnwatchContainerDir(containerName)
}

func TestWatcher_NonExistentDirSilentlyIgnored(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	collector := &eventCollector{}
	w := NewWatcher(baseDir, collector.handle, 100*time.Millisecond)

	if err := w.Start(context.Background()); err != nil {
		t.Fatalf("start watcher: %v", err)
	}
	defer w.Shutdown(context.Background()) //nolint:errcheck

	// Cleanup of a container that was never created should not panic.
	w.CleanupContainerDir("nonexistent")

	// Watch/Unwatch should also not panic.
	w.WatchContainerDir("nonexistent")
	w.UnwatchContainerDir("nonexistent")
}

func TestWatcher_MissingRequiredFields(t *testing.T) {
	t.Parallel()

	_, eventDir, w, collector := setupTestWatcher(t)

	if err := w.Start(context.Background()); err != nil {
		t.Fatalf("start watcher: %v", err)
	}
	defer w.Shutdown(context.Background()) //nolint:errcheck

	// Write event missing containerName.
	path := filepath.Join(eventDir, "99999-1.json")
	data := []byte(`{"type":"heartbeat"}`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	time.Sleep(300 * time.Millisecond)

	// Should not be processed.
	if collector.count() != 0 {
		t.Errorf("expected 0 events for missing required fields, got %d", collector.count())
	}

	// File should be deleted.
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("file with missing fields should have been deleted")
	}
}
