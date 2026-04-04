package agent

import (
	"context"
	"log/slog"
	"os"

	"github.com/thesimonho/warden/watcher"
)

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
	tailer   *watcher.FileTailer
}

// NewSessionWatcher creates a watcher for JSONL session files.
// The parser converts lines into ParsedEvents. The callback receives
// each event (typically wired to the eventbus). The homeDir and project
// are passed to the parser's FindSessionFiles for file discovery.
// The offsetStore (optional, may be nil) persists byte offsets so the
// tailer resumes from where it left off after a restart.
func NewSessionWatcher(
	parser SessionParser,
	homeDir string,
	project ProjectInfo,
	callback func(ParsedEvent),
	offsetStore watcher.OffsetStore,
) *SessionWatcher {
	sw := &SessionWatcher{
		parser:   parser,
		homeDir:  homeDir,
		project:  project,
		callback: callback,
		logger:   slog.Default().With("component", "session_watcher", "project", project.ProjectID),
	}

	sw.tailer = watcher.NewFileTailer(watcher.TailerConfig{
		Discover: func() []string {
			return sw.parser.FindSessionFiles(sw.homeDir, sw.project)
		},
		OnLine: func(path string, line []byte) {
			events := sw.parser.ParseLine(line)
			for i := range events {
				events[i].SourceLine = line
				events[i].SourceIndex = i
				sw.callback(events[i])
			}
		},
		PollInterval: watcher.DefaultPollInterval,
		Logger:       sw.logger,
		OffsetStore:  offsetStore,
		ProjectID:    project.ProjectID,
		AgentType:    project.AgentType,
	})

	return sw
}

// Start begins watching for session files. It discovers existing session
// files via the parser's FindSessionFiles and tails them. Periodically
// re-discovers to pick up new sessions.
func (sw *SessionWatcher) Start(ctx context.Context) error {
	// Ensure session directory exists so the agent has somewhere to write.
	sessionDir := sw.parser.SessionDir(sw.homeDir, sw.project)
	if err := os.MkdirAll(sessionDir, 0o700); err != nil {
		return err
	}

	sw.tailer.Start(ctx)
	return nil
}

// Stop signals the watcher to stop and waits for goroutines to finish.
func (sw *SessionWatcher) Stop() {
	sw.tailer.Stop()
}
