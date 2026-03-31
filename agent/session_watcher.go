package agent

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
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
	parser   SessionParser
	homeDir  string
	project  ProjectInfo
	callback func(ParsedEvent)
	logger   *slog.Logger

	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu          sync.Mutex
	tailedFiles map[string]context.CancelFunc // path → cancel for each tailed file
}

// NewSessionWatcher creates a watcher for JSONL session files.
// The parser converts lines into ParsedEvents. The callback receives
// each event (typically wired to the eventbus). The homeDir and project
// are passed to the parser's FindSessionFiles for file discovery.
func NewSessionWatcher(parser SessionParser, homeDir string, project ProjectInfo, callback func(ParsedEvent)) *SessionWatcher {
	return &SessionWatcher{
		parser:      parser,
		homeDir:     homeDir,
		project:     project,
		callback:    callback,
		tailedFiles: make(map[string]context.CancelFunc),
		logger:      slog.Default().With("component", "session_watcher", "project", project.ProjectID),
	}
}

// Start begins watching for session files. It discovers existing session
// files via the parser's FindSessionFiles and tails them. Periodically
// re-discovers to pick up new sessions.
func (sw *SessionWatcher) Start(ctx context.Context) error {
	ctx, sw.cancel = context.WithCancel(ctx)

	// Ensure session directory exists so the agent has somewhere to write.
	sessionDir := sw.parser.SessionDir(sw.homeDir, sw.project)
	if err := os.MkdirAll(sessionDir, 0o700); err != nil {
		return err
	}

	// Discover and tail any existing session files.
	sw.discoverAndTail(ctx)

	// Periodically re-discover new session files.
	sw.wg.Add(1)
	go sw.pollForNewFiles(ctx)

	return nil
}

// Stop signals the watcher to stop and waits for goroutines to finish.
func (sw *SessionWatcher) Stop() {
	if sw.cancel != nil {
		sw.cancel()
	}
	sw.wg.Wait()
	sw.mu.Lock()
	clear(sw.tailedFiles)
	sw.mu.Unlock()
}

// discoverAndTail calls FindSessionFiles and starts tailing any files
// not already being tailed.
func (sw *SessionWatcher) discoverAndTail(ctx context.Context) {
	files := sw.parser.FindSessionFiles(sw.homeDir, sw.project)

	sw.mu.Lock()
	defer sw.mu.Unlock()

	for _, path := range files {
		if _, exists := sw.tailedFiles[path]; exists {
			continue
		}
		sw.logger.Info("tailing session file", "path", filepath.Base(path))
		tailCtx, tailCancel := context.WithCancel(ctx)
		sw.tailedFiles[path] = tailCancel
		sw.startTailing(tailCtx, path)
	}
}

// pollForNewFiles periodically re-discovers session files.
func (sw *SessionWatcher) pollForNewFiles(ctx context.Context) {
	defer sw.wg.Done()

	ticker := time.NewTicker(sessionPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sw.discoverAndTail(ctx)
		}
	}
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
	defer func() { _ = f.Close() }()

	// Seek to end — we only want new lines from this point forward.
	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		sw.logger.Warn("failed to seek to end", "path", path, "err", err)
		return
	}

	reader := bufio.NewReader(f)

	// Read any lines already present to minimize initial latency.
	sw.readNewLines(reader)

	ticker := time.NewTicker(sessionPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
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
		line = bytes.TrimRight(line, "\r\n ")
		if len(line) == 0 {
			continue
		}

		events := sw.parser.ParseLine(line)
		for _, event := range events {
			sw.callback(event)
		}
	}
}

