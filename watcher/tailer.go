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

// tailedFile holds the per-file state for a file being tailed.
type tailedFile struct {
	path     string
	file     *os.File
	reader   *bufio.Reader
	offset   int64  // bytes consumed so far (complete lines only)
	partial  []byte // incomplete trailing line buffered across polls
	openFail bool   // true if the file could not be opened
}

// FileTailer monitors files for new lines appended over time.
// Files are discovered via a pluggable Discover function and tailed
// from the last stored offset (or the start if no offset exists).
//
// All file I/O runs in a single goroutine to minimize wake-ups and
// goroutine count. With N files, the tailer uses 1 goroutine instead
// of N+1.
//
// Lifecycle: single-use. Call Start once to begin watching, Stop once
// to shut down. Do not call Start again after Stop.
type FileTailer struct {
	cfg    TailerConfig
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu    sync.Mutex
	files map[string]*tailedFile // keyed by path
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
		cfg:   cfg,
		files: make(map[string]*tailedFile),
	}
}

// Start begins discovering and tailing files. It processes any existing
// files immediately, then polls for new files and new lines at the
// configured interval. Call Stop to shut down.
func (t *FileTailer) Start(ctx context.Context) {
	ctx, t.cancel = context.WithCancel(ctx)

	// Discover and open existing files, read their initial content.
	t.discoverFiles()
	t.readAllFiles()

	// Single goroutine for periodic discovery + reading.
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
	for _, tf := range t.files {
		if tf.file != nil {
			_ = tf.file.Close()
		}
	}
	clear(t.files)
	t.mu.Unlock()
}

// pollLoop periodically re-discovers files and reads new lines from all
// tracked files. This is the single goroutine that replaces N per-file
// goroutines, reducing wake-ups from N*0.5/s to 0.5/s total.
func (t *FileTailer) pollLoop(ctx context.Context) {
	defer t.wg.Done()

	ticker := time.NewTicker(t.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			t.discoverFiles()
			t.readAllFiles()
		}
	}
}

// discoverFiles calls Discover and opens any files not already being tailed.
// Prunes stored offsets for files that have disappeared from the discovery list.
func (t *FileTailer) discoverFiles() {
	discovered := t.cfg.Discover()
	discoveredSet := make(map[string]struct{}, len(discovered))
	for _, f := range discovered {
		discoveredSet[f] = struct{}{}
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	// Prune files no longer in discovery list.
	for path, tf := range t.files {
		if _, still := discoveredSet[path]; !still {
			if tf.file != nil {
				_ = tf.file.Close()
			}
			if t.cfg.OffsetStore != nil {
				if err := t.cfg.OffsetStore.DeleteOffset(t.cfg.ProjectID, t.cfg.AgentType, path); err != nil {
					t.cfg.Logger.Warn("failed to delete stale offset", "path", path, "err", err)
				}
			}
			delete(t.files, path)
		}
	}

	// Open newly discovered files.
	for _, path := range discovered {
		if _, exists := t.files[path]; exists {
			continue
		}

		tf := t.openFile(path)
		if tf == nil {
			// Track as failed so discoverFiles retries on next cycle.
			t.files[path] = &tailedFile{path: path, openFail: true}
			continue
		}
		t.cfg.Logger.Info("tailing file", "path", path)
		t.files[path] = tf
	}

	// Retry files that previously failed to open.
	for path, tf := range t.files {
		if !tf.openFail {
			continue
		}
		if _, still := discoveredSet[path]; !still {
			continue
		}
		reopened := t.openFile(path)
		if reopened != nil {
			t.cfg.Logger.Info("tailing file (retry)", "path", path)
			t.files[path] = reopened
		}
	}
}

// openFile opens a file and seeks to the stored offset. Returns nil if
// the file cannot be opened.
func (t *FileTailer) openFile(path string) *tailedFile {
	f, err := os.Open(path)
	if err != nil {
		t.cfg.Logger.Warn("failed to open file for tailing", "path", path, "err", err)
		return nil
	}

	var startOffset int64
	if t.cfg.OffsetStore != nil {
		stored, loadErr := t.cfg.OffsetStore.LoadOffset(t.cfg.ProjectID, t.cfg.AgentType, path)
		if loadErr != nil {
			t.cfg.Logger.Warn("failed to load offset, reading from start", "path", path, "err", loadErr)
		} else if stored > 0 {
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

	return &tailedFile{
		path:   path,
		file:   f,
		reader: bufio.NewReader(f),
		offset: startOffset,
	}
}

// readAllFiles reads new lines from every tracked file. Called from the
// single poll goroutine — no per-file goroutines needed.
func (t *FileTailer) readAllFiles() {
	t.mu.Lock()
	defer t.mu.Unlock()

	for _, tf := range t.files {
		if tf.file == nil || tf.openFail {
			continue
		}
		newBytes := t.readNewLines(tf)
		if newBytes > 0 {
			tf.offset += int64(newBytes)
			t.persistOffset(tf.path, tf.offset)
		}
	}
}

// readNewLines reads all available complete lines from a tailed file and
// dispatches them via OnLine. Returns the total number of bytes consumed
// (complete lines including newlines).
func (t *FileTailer) readNewLines(tf *tailedFile) int {
	bytesConsumed := 0

	for {
		chunk, err := tf.reader.ReadBytes('\n')
		bytesConsumed += len(chunk)

		if err != nil {
			// Incomplete line — buffer it for next poll. Subtract only the
			// bytes just read (chunk), not the entire accumulated partial
			// buffer, to avoid a negative return when partial data spans
			// multiple poll cycles.
			tf.partial = append(tf.partial, chunk...)
			bytesConsumed -= len(chunk)
			return bytesConsumed
		}

		// Prepend any buffered partial data.
		if len(tf.partial) > 0 {
			// The partial bytes were excluded from consumed in a previous
			// call, so count them now that the line is complete.
			bytesConsumed += len(tf.partial)
			chunk = append(tf.partial, chunk...)
			tf.partial = nil
		}

		line := bytes.TrimRight(chunk, "\r\n ")
		if len(line) == 0 {
			continue
		}

		t.cfg.OnLine(tf.path, line)
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
