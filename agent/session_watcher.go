package agent

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// sessionPollInterval is how often the watcher checks for new lines
// when fsnotify doesn't fire (e.g. Docker Desktop VM boundary).
const sessionPollInterval = 2 * time.Second

// SessionWatcher monitors a directory for JSONL session files and tails
// them line-by-line, feeding parsed events to a callback. It handles
// session file rotation (new session → new .jsonl file appears).
//
// Lifecycle: one watcher per project, created when a container starts,
// stopped when the container stops.
type SessionWatcher struct {
	parser    SessionParser
	sessionDir string
	callback  func(ParsedEvent)
	logger    *slog.Logger

	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu          sync.Mutex
	currentFile string // path of the file currently being tailed
}

// NewSessionWatcher creates a watcher for JSONL session files.
// The parser converts lines into ParsedEvents. The callback receives
// each event (typically wired to the eventbus).
func NewSessionWatcher(parser SessionParser, sessionDir string, callback func(ParsedEvent)) *SessionWatcher {
	return &SessionWatcher{
		parser:     parser,
		sessionDir: sessionDir,
		callback:   callback,
		logger:     slog.Default().With("component", "session_watcher", "dir", sessionDir),
	}
}

// Start begins watching for session files. It finds the most recent
// .jsonl file in the session directory (if any) and tails it. When new
// .jsonl files appear, it switches to tailing the new file.
func (sw *SessionWatcher) Start(ctx context.Context) error {
	ctx, sw.cancel = context.WithCancel(ctx)

	// Ensure session directory exists before watching.
	if err := os.MkdirAll(sw.sessionDir, 0o700); err != nil {
		return err
	}

	// Start tailing the most recent session file, if one exists.
	if latest := sw.findLatestJSONL(); latest != "" {
		sw.startTailing(ctx, latest)
	}

	// Watch for new .jsonl files (session rotation).
	sw.wg.Add(1)
	go sw.watchForNewFiles(ctx)

	return nil
}

// Stop signals the watcher to stop and waits for goroutines to finish.
func (sw *SessionWatcher) Stop() {
	if sw.cancel != nil {
		sw.cancel()
	}
	sw.wg.Wait()
}

// findLatestJSONL returns the path of the most recently modified .jsonl
// file in the session directory, or "" if none exist.
func (sw *SessionWatcher) findLatestJSONL() string {
	entries, err := os.ReadDir(sw.sessionDir)
	if err != nil {
		return ""
	}

	var latestPath string
	var latestTime time.Time

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(latestTime) {
			latestTime = info.ModTime()
			latestPath = filepath.Join(sw.sessionDir, entry.Name())
		}
	}
	return latestPath
}

// watchForNewFiles uses fsnotify to detect new .jsonl files appearing
// in the session directory. Falls back to polling when fsnotify doesn't
// fire (Docker Desktop).
func (sw *SessionWatcher) watchForNewFiles(ctx context.Context) {
	defer sw.wg.Done()

	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		sw.logger.Warn("fsnotify unavailable, using polling only", "err", err)
		sw.pollForNewFiles(ctx)
		return
	}
	defer fsw.Close()

	if err := fsw.Add(sw.sessionDir); err != nil {
		sw.logger.Warn("failed to watch session dir, using polling only", "err", err)
		sw.pollForNewFiles(ctx)
		return
	}

	ticker := time.NewTicker(sessionPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-fsw.Events:
			if !ok {
				return
			}
			if event.Op&(fsnotify.Create|fsnotify.Write) != 0 && strings.HasSuffix(event.Name, ".jsonl") {
				sw.switchToFile(ctx, event.Name)
			}
		case err, ok := <-fsw.Errors:
			if !ok {
				return
			}
			sw.logger.Warn("fsnotify error", "err", err)
		case <-ticker.C:
			// Polling fallback: check for newer files.
			if latest := sw.findLatestJSONL(); latest != "" {
				sw.switchToFile(ctx, latest)
			}
		}
	}
}

// pollForNewFiles is the pure-polling fallback when fsnotify is unavailable.
func (sw *SessionWatcher) pollForNewFiles(ctx context.Context) {
	ticker := time.NewTicker(sessionPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if latest := sw.findLatestJSONL(); latest != "" {
				sw.switchToFile(ctx, latest)
			}
		}
	}
}

// switchToFile starts tailing a new file if it differs from the current one.
func (sw *SessionWatcher) switchToFile(ctx context.Context, path string) {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	if path == sw.currentFile {
		return
	}

	sw.logger.Info("switching to new session file", "path", filepath.Base(path))
	sw.currentFile = path
	sw.startTailing(ctx, path)
}

// startTailing opens a file and begins reading new lines from the end.
// Each line is parsed and events are delivered to the callback.
func (sw *SessionWatcher) startTailing(ctx context.Context, path string) {
	sw.wg.Add(1)
	go func() {
		defer sw.wg.Done()
		sw.tailFile(ctx, path)
	}()
}

// tailFile reads a JSONL file from the current end, then polls for new
// lines. It processes each complete line through the parser.
func (sw *SessionWatcher) tailFile(ctx context.Context, path string) {
	f, err := os.Open(path)
	if err != nil {
		sw.logger.Warn("failed to open session file", "path", path, "err", err)
		return
	}
	defer f.Close()

	// Seek to end — we only want new lines from this point forward.
	// For existing files, we don't replay history.
	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		sw.logger.Warn("failed to seek to end", "path", path, "err", err)
		return
	}

	reader := bufio.NewReader(f)
	ticker := time.NewTicker(sessionPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Check if we've been superseded by a newer file.
			sw.mu.Lock()
			isStale := sw.currentFile != path
			sw.mu.Unlock()
			if isStale {
				return
			}

			sw.readNewLines(reader)
		}
	}
}

// readNewLines reads all available complete lines from the reader and
// dispatches parsed events.
func (sw *SessionWatcher) readNewLines(reader *bufio.Reader) {
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			// No more complete lines available — wait for next poll.
			return
		}

		// Skip empty lines.
		line = trimLine(line)
		if len(line) == 0 {
			continue
		}

		events := sw.parser.ParseLine(line)
		for _, event := range events {
			sw.callback(event)
		}
	}
}

// trimLine removes trailing whitespace (newlines, carriage returns).
func trimLine(line []byte) []byte {
	for len(line) > 0 && (line[len(line)-1] == '\n' || line[len(line)-1] == '\r' || line[len(line)-1] == ' ') {
		line = line[:len(line)-1]
	}
	return line
}
