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

// memoryOffsetStore implements watcher.OffsetStore with an in-memory map.
type memoryOffsetStore struct {
	mu      sync.Mutex
	offsets map[string]int64
}

func newMemoryOffsetStore() *memoryOffsetStore {
	return &memoryOffsetStore{offsets: make(map[string]int64)}
}

func (m *memoryOffsetStore) key(projectID, agentType, filePath string) string {
	return projectID + ":" + agentType + ":" + filePath
}

func (m *memoryOffsetStore) LoadOffset(projectID, agentType, filePath string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.offsets[m.key(projectID, agentType, filePath)], nil
}

func (m *memoryOffsetStore) SaveOffset(projectID, agentType, filePath string, offset int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.offsets[m.key(projectID, agentType, filePath)] = offset
	return nil
}

func (m *memoryOffsetStore) DeleteOffset(projectID, agentType, filePath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.offsets, m.key(projectID, agentType, filePath))
	return nil
}

func (m *memoryOffsetStore) DeleteOffsets(projectID, agentType string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	prefix := projectID + ":" + agentType + ":"
	for k := range m.offsets {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			delete(m.offsets, k)
		}
	}
	return nil
}

func (m *memoryOffsetStore) getOffset(projectID, agentType, filePath string) int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.offsets[m.key(projectID, agentType, filePath)]
}

func TestFileTailer_ResumesFromStoredOffset(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	// Write 3 lines.
	if err := os.WriteFile(path, []byte("line1\nline2\nline3\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	store := newMemoryOffsetStore()
	// Pre-store an offset past "line1\nline2\n" (12 bytes).
	_ = store.SaveOffset("proj", "claude-code", path, 12)

	var mu sync.Mutex
	var lines []string

	tailer := watcher.NewFileTailer(watcher.TailerConfig{
		Discover:     func() []string { return []string{path} },
		OnLine:       func(_ string, line []byte) { mu.Lock(); lines = append(lines, string(line)); mu.Unlock() },
		PollInterval: testPollInterval,
		OffsetStore:  store,
		ProjectID:    "proj",
		AgentType:    "claude-code",
	})

	ctx := context.Background()
	tailer.Start(ctx)
	defer tailer.Stop()

	waitFor(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(lines) == 1
	})

	mu.Lock()
	defer mu.Unlock()
	if lines[0] != "line3" {
		t.Errorf("expected 'line3', got %q", lines[0])
	}
}

func TestFileTailer_ResetsOnTruncation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	// Write a small file.
	if err := os.WriteFile(path, []byte("fresh\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	store := newMemoryOffsetStore()
	// Store an offset larger than the file — simulates truncation.
	_ = store.SaveOffset("proj", "claude-code", path, 99999)

	var mu sync.Mutex
	var lines []string

	tailer := watcher.NewFileTailer(watcher.TailerConfig{
		Discover:     func() []string { return []string{path} },
		OnLine:       func(_ string, line []byte) { mu.Lock(); lines = append(lines, string(line)); mu.Unlock() },
		PollInterval: testPollInterval,
		OffsetStore:  store,
		ProjectID:    "proj",
		AgentType:    "claude-code",
	})

	ctx := context.Background()
	tailer.Start(ctx)
	defer tailer.Stop()

	// Should reset to 0 and read the line.
	waitFor(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(lines) == 1
	})

	mu.Lock()
	defer mu.Unlock()
	if lines[0] != "fresh" {
		t.Errorf("expected 'fresh', got %q", lines[0])
	}
}

func TestFileTailer_PersistsOffset(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	if err := os.WriteFile(path, []byte("line1\nline2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	store := newMemoryOffsetStore()
	var mu sync.Mutex
	var lines []string

	tailer := watcher.NewFileTailer(watcher.TailerConfig{
		Discover:     func() []string { return []string{path} },
		OnLine:       func(_ string, line []byte) { mu.Lock(); lines = append(lines, string(line)); mu.Unlock() },
		PollInterval: testPollInterval,
		OffsetStore:  store,
		ProjectID:    "proj",
		AgentType:    "claude-code",
	})

	ctx := context.Background()
	tailer.Start(ctx)

	waitFor(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(lines) == 2
	})

	tailer.Stop()

	// Offset should be 12 (len("line1\nline2\n")).
	offset := store.getOffset("proj", "claude-code", path)
	if offset != 12 {
		t.Errorf("expected offset 12, got %d", offset)
	}
}

func TestFileTailer_NilOffsetStoreReadsFromStart(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	if err := os.WriteFile(path, []byte("line1\nline2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var lines []string

	// No OffsetStore — should read from byte 0.
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

func TestFileTailer_PrunesOffsetsForDisappearedFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	if err := os.WriteFile(path, []byte("line1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	store := newMemoryOffsetStore()
	var mu sync.Mutex
	var lines []string

	// Mutable discovery list.
	var discMu sync.Mutex
	discoveredPaths := []string{path}
	discover := func() []string {
		discMu.Lock()
		defer discMu.Unlock()
		result := make([]string, len(discoveredPaths))
		copy(result, discoveredPaths)
		return result
	}

	tailer := watcher.NewFileTailer(watcher.TailerConfig{
		Discover:     discover,
		OnLine:       func(_ string, line []byte) { mu.Lock(); lines = append(lines, string(line)); mu.Unlock() },
		PollInterval: testPollInterval,
		OffsetStore:  store,
		ProjectID:    "proj",
		AgentType:    "claude-code",
	})

	ctx := context.Background()
	tailer.Start(ctx)
	defer tailer.Stop()

	// Wait for line to be processed.
	waitFor(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(lines) == 1
	})

	// Verify offset was stored.
	if offset := store.getOffset("proj", "claude-code", path); offset == 0 {
		t.Fatal("expected non-zero offset after reading")
	}

	// Remove file from discovery list.
	discMu.Lock()
	discoveredPaths = nil
	discMu.Unlock()

	// Wait for discovery to prune.
	waitFor(t, 2*time.Second, func() bool {
		return store.getOffset("proj", "claude-code", path) == 0
	})
}

func TestFileTailer_OffsetAdvancesOnAppend(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	if err := os.WriteFile(path, []byte("line1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	store := newMemoryOffsetStore()
	var mu sync.Mutex
	var lines []string

	tailer := watcher.NewFileTailer(watcher.TailerConfig{
		Discover:     func() []string { return []string{path} },
		OnLine:       func(_ string, line []byte) { mu.Lock(); lines = append(lines, string(line)); mu.Unlock() },
		PollInterval: testPollInterval,
		OffsetStore:  store,
		ProjectID:    "proj",
		AgentType:    "claude-code",
	})

	ctx := context.Background()
	tailer.Start(ctx)
	defer tailer.Stop()

	waitFor(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(lines) == 1
	})

	offsetAfterFirst := store.getOffset("proj", "claude-code", path)
	if offsetAfterFirst != 6 { // "line1\n" = 6 bytes
		t.Fatalf("expected offset 6 after first line, got %d", offsetAfterFirst)
	}

	// Append another line.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("line2\n"); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	waitFor(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(lines) == 2
	})

	offsetAfterSecond := store.getOffset("proj", "claude-code", path)
	if offsetAfterSecond != 12 { // "line1\nline2\n" = 12 bytes
		t.Errorf("expected offset 12 after second line, got %d", offsetAfterSecond)
	}
}

func TestFileTailer_ResumeAndAppend(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	// Write 3 lines.
	if err := os.WriteFile(path, []byte("old1\nold2\nnew1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	store := newMemoryOffsetStore()
	// Pre-store offset past first 2 lines: "old1\nold2\n" = 10 bytes.
	_ = store.SaveOffset("proj", "claude-code", path, 10)

	var mu sync.Mutex
	var lines []string

	tailer := watcher.NewFileTailer(watcher.TailerConfig{
		Discover:     func() []string { return []string{path} },
		OnLine:       func(_ string, line []byte) { mu.Lock(); lines = append(lines, string(line)); mu.Unlock() },
		PollInterval: testPollInterval,
		OffsetStore:  store,
		ProjectID:    "proj",
		AgentType:    "claude-code",
	})

	ctx := context.Background()
	tailer.Start(ctx)
	defer tailer.Stop()

	// Should only get "new1" initially.
	waitFor(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(lines) == 1
	})

	// Append more.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("new2\n"); err != nil {
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
	if lines[0] != "new1" {
		t.Errorf("expected 'new1', got %q", lines[0])
	}
	if lines[1] != "new2" {
		t.Errorf("expected 'new2', got %q", lines[1])
	}
}

func TestFileTailer_OffsetNotSavedForPartialLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	// Write a complete line + partial.
	if err := os.WriteFile(path, []byte("complete\npartial"), 0o644); err != nil {
		t.Fatal(err)
	}

	store := newMemoryOffsetStore()
	var mu sync.Mutex
	var lines []string

	tailer := watcher.NewFileTailer(watcher.TailerConfig{
		Discover:     func() []string { return []string{path} },
		OnLine:       func(_ string, line []byte) { mu.Lock(); lines = append(lines, string(line)); mu.Unlock() },
		PollInterval: testPollInterval,
		OffsetStore:  store,
		ProjectID:    "proj",
		AgentType:    "claude-code",
	})

	ctx := context.Background()
	tailer.Start(ctx)

	// Wait for the complete line.
	waitFor(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(lines) == 1
	})

	// Give time for offset to settle.
	time.Sleep(100 * time.Millisecond)

	// Offset should be at 9 (len("complete\n")), not including "partial".
	offset := store.getOffset("proj", "claude-code", path)
	if offset != 9 {
		t.Errorf("expected offset 9 (excluding partial), got %d", offset)
	}

	tailer.Stop()
}

func TestFileTailer_ZeroOffsetReadsFromStart(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	if err := os.WriteFile(path, []byte("line1\nline2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	store := newMemoryOffsetStore()
	// Explicitly store offset 0 — should read everything.
	_ = store.SaveOffset("proj", "claude-code", path, 0)

	var mu sync.Mutex
	var lines []string

	tailer := watcher.NewFileTailer(watcher.TailerConfig{
		Discover:     func() []string { return []string{path} },
		OnLine:       func(_ string, line []byte) { mu.Lock(); lines = append(lines, string(line)); mu.Unlock() },
		PollInterval: testPollInterval,
		OffsetStore:  store,
		ProjectID:    "proj",
		AgentType:    "claude-code",
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

func TestFileTailer_ResumeAtPartialLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	// Write "line1\nparti" — offset stored past line1, partial "parti" remains.
	if err := os.WriteFile(path, []byte("line1\nparti"), 0o644); err != nil {
		t.Fatal(err)
	}

	store := newMemoryOffsetStore()
	// Pre-store offset past "line1\n" (6 bytes) — reader starts at "parti" (no newline).
	_ = store.SaveOffset("proj", "claude-code", path, 6)

	var mu sync.Mutex
	var lines []string

	tailer := watcher.NewFileTailer(watcher.TailerConfig{
		Discover:     func() []string { return []string{path} },
		OnLine:       func(_ string, line []byte) { mu.Lock(); lines = append(lines, string(line)); mu.Unlock() },
		PollInterval: testPollInterval,
		OffsetStore:  store,
		ProjectID:    "proj",
		AgentType:    "claude-code",
	})

	ctx := context.Background()
	tailer.Start(ctx)
	defer tailer.Stop()

	// No complete line yet — nothing delivered.
	time.Sleep(150 * time.Millisecond)
	mu.Lock()
	if len(lines) != 0 {
		t.Fatalf("expected no lines from partial, got %v", lines)
	}
	mu.Unlock()

	// Complete the line.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("al_done\n"); err != nil {
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
	if lines[0] != "partial_done" {
		t.Errorf("expected 'partial_done', got %q", lines[0])
	}
}

func TestFileTailer_MultipleFilesIndependentOffsets(t *testing.T) {
	dir := t.TempDir()
	pathA := filepath.Join(dir, "a.jsonl")
	pathB := filepath.Join(dir, "b.jsonl")

	// File A: 3 lines, offset past first 2.
	if err := os.WriteFile(pathA, []byte("a1\na2\na3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// File B: 2 lines, no stored offset (read from start).
	if err := os.WriteFile(pathB, []byte("b1\nb2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	store := newMemoryOffsetStore()
	_ = store.SaveOffset("proj", "claude-code", pathA, 6) // past "a1\na2\n"

	var mu sync.Mutex
	var lines []string

	tailer := watcher.NewFileTailer(watcher.TailerConfig{
		Discover:     func() []string { return []string{pathA, pathB} },
		OnLine:       func(_ string, line []byte) { mu.Lock(); lines = append(lines, string(line)); mu.Unlock() },
		PollInterval: testPollInterval,
		OffsetStore:  store,
		ProjectID:    "proj",
		AgentType:    "claude-code",
	})

	ctx := context.Background()
	tailer.Start(ctx)
	defer tailer.Stop()

	// Should get: a3 (from A, resumed) + b1, b2 (from B, full read) = 3 lines.
	waitFor(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(lines) == 3
	})

	mu.Lock()
	defer mu.Unlock()

	// Verify we got a3, b1, b2 (order between files is non-deterministic).
	got := make(map[string]bool)
	for _, l := range lines {
		got[l] = true
	}
	for _, expected := range []string{"a3", "b1", "b2"} {
		if !got[expected] {
			t.Errorf("missing expected line %q, got %v", expected, lines)
		}
	}
	// Verify we did NOT get a1 or a2.
	for _, unexpected := range []string{"a1", "a2"} {
		if got[unexpected] {
			t.Errorf("should not have received %q (skipped by offset), got %v", unexpected, lines)
		}
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

func TestFileTailer_MultiplePartialWritesBeforeNewline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	// Write first partial chunk (no newline).
	if err := os.WriteFile(path, []byte("aaa"), 0o644); err != nil {
		t.Fatal(err)
	}

	store := newMemoryOffsetStore()
	var mu sync.Mutex
	var lines []string

	tailer := watcher.NewFileTailer(watcher.TailerConfig{
		Discover:     func() []string { return []string{path} },
		OnLine:       func(_ string, line []byte) { mu.Lock(); lines = append(lines, string(line)); mu.Unlock() },
		PollInterval: testPollInterval,
		OffsetStore:  store,
		ProjectID:    "proj",
		AgentType:    "claude-code",
	})

	ctx := context.Background()
	tailer.Start(ctx)
	defer tailer.Stop()

	// Wait for first poll to buffer partial "aaa".
	time.Sleep(150 * time.Millisecond)

	// Append second partial chunk (still no newline).
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("bbb"); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	// Wait for second poll to buffer "bbb" on top of "aaa".
	time.Sleep(150 * time.Millisecond)

	// No line should be delivered yet.
	mu.Lock()
	if len(lines) != 0 {
		t.Fatalf("expected no lines from partial writes, got %v", lines)
	}
	mu.Unlock()

	// Complete the line.
	f, err = os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("ccc\n"); err != nil {
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
	if lines[0] != "aaabbbccc" {
		t.Errorf("expected 'aaabbbccc', got %q", lines[0])
	}

	// Offset should cover the entire line: len("aaabbbccc\n") = 10.
	offset := store.getOffset("proj", "claude-code", path)
	if offset != 10 {
		t.Errorf("expected offset 10, got %d", offset)
	}
}
