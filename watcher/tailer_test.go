package watcher_test

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/thesimonho/warden/watcher"
)

const testPollInterval = 50 * time.Millisecond

// waitFor polls a condition until it returns true or the timeout expires.
func waitFor(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for condition")
}

func TestFileTailer_ReadsExistingLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	if err := os.WriteFile(path, []byte("line1\nline2\nline3\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var lines []string

	tailer := watcher.NewFileTailer(watcher.TailerConfig{
		Discover:     func() []string { return []string{path} },
		OnLine:       func(_ string, line []byte) { mu.Lock(); lines = append(lines, string(line)); mu.Unlock() },
		PollInterval: testPollInterval,
	})

	ctx := context.Background()
	tailer.Start(ctx)
	defer tailer.Stop()

	waitFor(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(lines) == 3
	})

	mu.Lock()
	defer mu.Unlock()
	if lines[0] != "line1" || lines[1] != "line2" || lines[2] != "line3" {
		t.Errorf("unexpected lines: %v", lines)
	}
}

func TestFileTailer_TailsNewLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	// Start with one line.
	if err := os.WriteFile(path, []byte("existing\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var lines []string

	tailer := watcher.NewFileTailer(watcher.TailerConfig{
		Discover:     func() []string { return []string{path} },
		OnLine:       func(_ string, line []byte) { mu.Lock(); lines = append(lines, string(line)); mu.Unlock() },
		PollInterval: testPollInterval,
	})

	ctx := context.Background()
	tailer.Start(ctx)
	defer tailer.Stop()

	// Wait for existing line.
	waitFor(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(lines) == 1
	})

	// Append a new line.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("appended\n"); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	waitFor(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(lines) == 2
	})

	mu.Lock()
	defer mu.Unlock()
	if lines[1] != "appended" {
		t.Errorf("expected 'appended', got %q", lines[1])
	}
}

func TestFileTailer_DiscoversNewFiles(t *testing.T) {
	dir := t.TempDir()

	var discoveredMu sync.Mutex
	var discoveredPaths []string

	// Discover function returns whatever paths are in discoveredPaths.
	discover := func() []string {
		discoveredMu.Lock()
		defer discoveredMu.Unlock()
		result := make([]string, len(discoveredPaths))
		copy(result, discoveredPaths)
		return result
	}

	var mu sync.Mutex
	var lines []string

	tailer := watcher.NewFileTailer(watcher.TailerConfig{
		Discover:     discover,
		OnLine:       func(_ string, line []byte) { mu.Lock(); lines = append(lines, string(line)); mu.Unlock() },
		PollInterval: testPollInterval,
	})

	ctx := context.Background()
	tailer.Start(ctx)
	defer tailer.Stop()

	// Create a file and add it to discovery.
	path := filepath.Join(dir, "new.jsonl")
	if err := os.WriteFile(path, []byte("discovered\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	discoveredMu.Lock()
	discoveredPaths = append(discoveredPaths, path)
	discoveredMu.Unlock()

	waitFor(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(lines) == 1
	})

	mu.Lock()
	defer mu.Unlock()
	if lines[0] != "discovered" {
		t.Errorf("expected 'discovered', got %q", lines[0])
	}
}

func TestFileTailer_SkipsEmptyLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	if err := os.WriteFile(path, []byte("line1\n\n  \nline2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var lines []string

	tailer := watcher.NewFileTailer(watcher.TailerConfig{
		Discover:     func() []string { return []string{path} },
		OnLine:       func(_ string, line []byte) { mu.Lock(); lines = append(lines, string(line)); mu.Unlock() },
		PollInterval: testPollInterval,
	})

	ctx := context.Background()
	tailer.Start(ctx)
	defer tailer.Stop()

	waitFor(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(lines) == 2
	})

	mu.Lock()
	defer mu.Unlock()
	if lines[0] != "line1" || lines[1] != "line2" {
		t.Errorf("unexpected lines: %v", lines)
	}
}

func TestFileTailer_IncludesPathInCallback(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")

	if err := os.WriteFile(path, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var receivedPath string

	tailer := watcher.NewFileTailer(watcher.TailerConfig{
		Discover:     func() []string { return []string{path} },
		OnLine:       func(p string, _ []byte) { mu.Lock(); receivedPath = p; mu.Unlock() },
		PollInterval: testPollInterval,
	})

	ctx := context.Background()
	tailer.Start(ctx)
	defer tailer.Stop()

	waitFor(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return receivedPath != ""
	})

	mu.Lock()
	defer mu.Unlock()
	if receivedPath != path {
		t.Errorf("expected path %q, got %q", path, receivedPath)
	}
}

func TestFileTailer_HandlesMissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.jsonl")

	var mu sync.Mutex
	var lines []string

	tailer := watcher.NewFileTailer(watcher.TailerConfig{
		Discover:     func() []string { return []string{path} },
		OnLine:       func(_ string, line []byte) { mu.Lock(); lines = append(lines, string(line)); mu.Unlock() },
		PollInterval: testPollInterval,
	})

	ctx := context.Background()
	tailer.Start(ctx)

	// Give it time to try and fail.
	time.Sleep(100 * time.Millisecond)
	tailer.Stop()

	mu.Lock()
	defer mu.Unlock()
	if len(lines) != 0 {
		t.Errorf("expected no lines, got %v", lines)
	}
}

func TestFileTailer_ConcurrentFiles(t *testing.T) {
	dir := t.TempDir()

	// Create 3 files.
	paths := make([]string, 3)
	for i := range paths {
		paths[i] = filepath.Join(dir, filepath.Base(t.TempDir())+".jsonl")
		if err := os.WriteFile(paths[i], []byte("file"+string(rune('A'+i))+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	var mu sync.Mutex
	var lines []string

	tailer := watcher.NewFileTailer(watcher.TailerConfig{
		Discover:     func() []string { return paths },
		OnLine:       func(_ string, line []byte) { mu.Lock(); lines = append(lines, string(line)); mu.Unlock() },
		PollInterval: testPollInterval,
	})

	ctx := context.Background()
	tailer.Start(ctx)
	defer tailer.Stop()

	waitFor(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(lines) == 3
	})

	mu.Lock()
	defer mu.Unlock()
	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(lines))
	}
}

func TestFileTailer_StopIsIdempotent(t *testing.T) {
	tailer := watcher.NewFileTailer(watcher.TailerConfig{
		Discover:     func() []string { return nil },
		OnLine:       func(_ string, _ []byte) {},
		PollInterval: testPollInterval,
	})

	ctx := context.Background()
	tailer.Start(ctx)
	tailer.Stop()
	tailer.Stop() // Should not panic.
}

func TestFileTailer_RetriesAfterFailedOpen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "late.jsonl")

	var mu sync.Mutex
	var lines []string

	tailer := watcher.NewFileTailer(watcher.TailerConfig{
		Discover:     func() []string { return []string{path} },
		OnLine:       func(_ string, line []byte) { mu.Lock(); lines = append(lines, string(line)); mu.Unlock() },
		PollInterval: testPollInterval,
	})

	ctx := context.Background()
	tailer.Start(ctx)
	defer tailer.Stop()

	// First discovery fails (file doesn't exist).
	time.Sleep(100 * time.Millisecond)
	mu.Lock()
	if len(lines) != 0 {
		t.Fatalf("expected no lines before file exists, got %v", lines)
	}
	mu.Unlock()

	// Create the file — next discovery should retry and succeed.
	if err := os.WriteFile(path, []byte("retried\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	waitFor(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(lines) == 1
	})

	mu.Lock()
	defer mu.Unlock()
	if lines[0] != "retried" {
		t.Errorf("expected 'retried', got %q", lines[0])
	}
}

func TestFileTailer_PartialLineWaitsForNewline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	// Write a partial line (no trailing newline).
	if err := os.WriteFile(path, []byte("partial"), 0o644); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var lines []string

	tailer := watcher.NewFileTailer(watcher.TailerConfig{
		Discover:     func() []string { return []string{path} },
		OnLine:       func(_ string, line []byte) { mu.Lock(); lines = append(lines, string(line)); mu.Unlock() },
		PollInterval: testPollInterval,
	})

	ctx := context.Background()
	tailer.Start(ctx)
	defer tailer.Stop()

	// Wait a bit — partial line should NOT be delivered.
	time.Sleep(200 * time.Millisecond)
	mu.Lock()
	if len(lines) != 0 {
		t.Fatalf("partial line should not be delivered, got %v", lines)
	}
	mu.Unlock()

	// Complete the line.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(" complete\n"); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	waitFor(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(lines) == 1
	})

	mu.Lock()
	defer mu.Unlock()
	if lines[0] != "partial complete" {
		t.Errorf("expected 'partial complete', got %q", lines[0])
	}
}
