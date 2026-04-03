// Package watcher provides generic file-watching primitives with zero
// internal module dependencies. Agent-specific parsers live in agent/<name>/;
// this package handles only the I/O mechanics of discovering and tailing files.
package watcher

import (
	"bufio"
	"bytes"
	"context"
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
}

// FileTailer monitors files for new lines appended over time.
// Files are discovered via a pluggable Discover function and tailed
// from the start, with periodic polling for new content. Designed for
// JSONL session files that are written to continuously.
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
// already being tailed.
func (t *FileTailer) discoverAndTail(ctx context.Context) {
	files := t.cfg.Discover()

	t.mu.Lock()
	defer t.mu.Unlock()

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

// tailFile reads a file from the start, processing all existing lines,
// then polls for new appended lines. Partial lines (no trailing newline)
// are buffered until the next read completes them. Returns false if the
// file could not be opened.
func (t *FileTailer) tailFile(ctx context.Context, path string) bool {
	f, err := os.Open(path)
	if err != nil {
		t.cfg.Logger.Warn("failed to open file for tailing", "path", path, "err", err)
		return false
	}
	defer func() { _ = f.Close() }()

	reader := bufio.NewReader(f)
	var partial []byte

	// Process all existing lines from the start.
	partial = t.readNewLines(reader, path, partial)

	ticker := time.NewTicker(t.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return true
		case <-ticker.C:
			partial = t.readNewLines(reader, path, partial)
		}
	}
}

// readNewLines reads all available complete lines from the reader and
// dispatches them via OnLine. Any incomplete trailing data is returned
// as partial so it can be prepended to the next read.
func (t *FileTailer) readNewLines(reader *bufio.Reader, path string, partial []byte) []byte {
	for {
		chunk, err := reader.ReadBytes('\n')
		if err != nil {
			// Incomplete line — buffer it for next poll.
			partial = append(partial, chunk...)
			return partial
		}

		// Prepend any buffered partial data.
		if len(partial) > 0 {
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
