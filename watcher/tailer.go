// Package watcher provides generic file-watching primitives with zero
// internal module dependencies. Agent-specific parsers live in agent/<name>/;
// this package handles only the I/O mechanics of discovering and tailing files.
package watcher

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"
)

// DefaultPollInterval is how often the tailer checks for new lines
// and discovers new files when no custom interval is specified.
const DefaultPollInterval = 2 * time.Second

// TailerConfig configures a FileTailer.
type TailerConfig struct {
	// Discover returns the current list of file paths to tail.
	// Called periodically to pick up new files (e.g. new sessions).
	Discover func() []string

	// OnLine is called for each complete line read from a tailed file.
	// The path identifies which file the line came from.
	OnLine func(path string, line []byte)

	// PollInterval controls how often the tailer checks for new lines
	// and re-discovers files. Defaults to 2s if zero.
	PollInterval time.Duration

	// Logger is the structured logger. Defaults to slog.Default() if nil.
	Logger *slog.Logger

	// OffsetStore persists byte offsets so the tailer can resume from
	// where it left off after a restart. Nil means read from byte 0.
	OffsetStore OffsetStore

	// ProjectID identifies the project for offset storage.
	ProjectID string

	// AgentType identifies the agent type for offset storage.
	AgentType string
}

// FileTailer monitors files for new lines appended over time.
// Files are discovered via a pluggable Discover function and tailed
// from the last stored offset (or the start if no offset exists).
// Periodic polling picks up new content and new files.
//
// Lifecycle: single-use. Call Start once to begin watching, Stop once
// to shut down. Do not call Start again after Stop.
type FileTailer struct {
	cfg    TailerConfig
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu          sync.Mutex
	tailedFiles map[string]struct{} // tracks which paths are being tailed
}

// NewFileTailer creates a tailer with the given configuration.
// The Discover and OnLine callbacks are required.
func NewFileTailer(cfg TailerConfig) *FileTailer {
	if cfg.PollInterval == 0 {
		cfg.PollInterval = DefaultPollInterval
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &FileTailer{
		cfg:         cfg,
		tailedFiles: make(map[string]struct{}),
	}
}

// Start begins discovering and tailing files. It processes any existing
// files immediately, then polls for new files and new lines at the
// configured interval. Call Stop to shut down.
func (t *FileTailer) Start(ctx context.Context) {
	ctx, t.cancel = context.WithCancel(ctx)

	// Discover and tail any existing files.
	t.discoverAndTail(ctx)

	// Periodically re-discover new files.
	t.wg.Add(1)
	go t.pollLoop(ctx)
}

// Stop signals the tailer to stop and waits for all goroutines to finish.
func (t *FileTailer) Stop() {
	if t.cancel != nil {
		t.cancel()
	}
	t.wg.Wait()
	t.mu.Lock()
	clear(t.tailedFiles)
	t.mu.Unlock()
}

// discoverAndTail calls Discover and starts tailing any files not
// already being tailed. Prunes stored offsets for files that have
// disappeared from the discovery list.
func (t *FileTailer) discoverAndTail(ctx context.Context) {
	files := t.cfg.Discover()
	discovered := make(map[string]struct{}, len(files))
	for _, f := range files {
		discovered[f] = struct{}{}
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	// Prune offsets for files no longer discovered.
	if t.cfg.OffsetStore != nil {
		for path := range t.tailedFiles {
			if _, still := discovered[path]; !still {
				if err := t.cfg.OffsetStore.DeleteOffset(t.cfg.ProjectID, t.cfg.AgentType, path); err != nil {
					t.cfg.Logger.Warn("failed to delete stale offset", "path", path, "err", err)
				}
				delete(t.tailedFiles, path)
			}
		}
	}

	for _, path := range files {
		if _, exists := t.tailedFiles[path]; exists {
			continue
		}
		t.cfg.Logger.Info("tailing file", "path", path)
		t.tailedFiles[path] = struct{}{}
		t.wg.Add(1)
		go func(p string) {
			defer t.wg.Done()
			if !t.tailFile(ctx, p) {
				// Open failed — remove from tracked set so the next
				// discovery cycle can retry.
				t.mu.Lock()
				delete(t.tailedFiles, p)
				t.mu.Unlock()
			}
		}(path)
	}
}

// pollLoop periodically re-discovers files and checks for new lines.
func (t *FileTailer) pollLoop(ctx context.Context) {
	defer t.wg.Done()

	ticker := time.NewTicker(t.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			t.discoverAndTail(ctx)
		}
	}
}

// tailFile opens a file and tails it from the stored offset (or byte 0
// if no offset exists or the file is smaller than the stored offset).
// Returns false if the file could not be opened.
func (t *FileTailer) tailFile(ctx context.Context, path string) bool {
	f, err := os.Open(path)
	if err != nil {
		t.cfg.Logger.Warn("failed to open file for tailing", "path", path, "err", err)
		return false
	}
	defer func() { _ = f.Close() }()

	// Resume from stored offset when available.
	var startOffset int64
	if t.cfg.OffsetStore != nil {
		stored, loadErr := t.cfg.OffsetStore.LoadOffset(t.cfg.ProjectID, t.cfg.AgentType, path)
		if loadErr != nil {
			t.cfg.Logger.Warn("failed to load offset, reading from start", "path", path, "err", loadErr)
		} else if stored > 0 {
			// Safety: if file is smaller than stored offset, the file was
			// truncated or replaced — reset to 0.
			info, statErr := f.Stat()
			if statErr == nil && info.Size() < stored {
				t.cfg.Logger.Info("file smaller than stored offset, resetting", "path", path, "stored", stored, "size", info.Size())
				stored = 0
			}
			if stored > 0 {
				if _, seekErr := f.Seek(stored, io.SeekStart); seekErr != nil {
					t.cfg.Logger.Warn("failed to seek, reading from start", "path", path, "err", seekErr)
				} else {
					startOffset = stored
				}
			}
		}
	}

	reader := bufio.NewReader(f)
	consumed := startOffset
	var partial []byte

	// Process all existing lines from the current position.
	var newBytes int
	partial, newBytes = t.readNewLines(reader, path, partial)
	consumed += int64(newBytes)
	t.persistOffset(path, consumed)

	ticker := time.NewTicker(t.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return true
		case <-ticker.C:
			partial, newBytes = t.readNewLines(reader, path, partial)
			if newBytes > 0 {
				consumed += int64(newBytes)
				t.persistOffset(path, consumed)
			}
		}
	}
}

// persistOffset saves the current byte offset if an OffsetStore is configured.
func (t *FileTailer) persistOffset(path string, offset int64) {
	if t.cfg.OffsetStore == nil {
		return
	}
	if err := t.cfg.OffsetStore.SaveOffset(t.cfg.ProjectID, t.cfg.AgentType, path, offset); err != nil {
		t.cfg.Logger.Warn("failed to save offset", "path", path, "err", err)
	}
}

// readNewLines reads all available complete lines from the reader and
// dispatches them via OnLine. Returns any incomplete trailing data as
// partial and the total number of bytes consumed (including newlines).
func (t *FileTailer) readNewLines(reader *bufio.Reader, path string, partial []byte) ([]byte, int) {
	bytesConsumed := 0

	for {
		chunk, err := reader.ReadBytes('\n')
		bytesConsumed += len(chunk)

		if err != nil {
			// Incomplete line — buffer it for next poll.
			partial = append(partial, chunk...)
			// Don't count partial bytes as consumed — they haven't been
			// delivered yet and we need to re-read them if we restart.
			bytesConsumed -= len(partial)
			return partial, bytesConsumed
		}

		// Prepend any buffered partial data.
		if len(partial) > 0 {
			// The partial bytes were excluded from consumed in a previous
			// call, so count them now that the line is complete.
			bytesConsumed += len(partial)
			chunk = append(partial, chunk...)
			partial = nil
		}

		line := bytes.TrimRight(chunk, "\r\n ")
		if len(line) == 0 {
			continue
		}

		t.cfg.OnLine(path, line)
	}
}
