package db

import (
	"log/slog"
	"sync/atomic"
)

// AuditMode controls which events are persisted to the audit log.
type AuditMode string

const (
	// AuditOff disables all audit logging. Nothing is written.
	AuditOff AuditMode = "off"
	// AuditStandard logs core operational events: session lifecycle,
	// terminal lifecycle, worktree lifecycle, budget, and system events.
	AuditStandard AuditMode = "standard"
	// AuditDetailed logs everything in standard plus agent tool use,
	// permissions, subagents, config changes, user prompts, and debug
	// events (auto-captured backend warnings/errors via slog tee).
	AuditDetailed AuditMode = "detailed"
)

// AuditWriter gates audit log writes based on the current audit mode.
// All audit writes across the codebase go through this single entry point.
// Thread-safe for concurrent reads (mode) and writes.
type AuditWriter struct {
	store          *Store
	mode           atomic.Value    // stores AuditMode
	standardEvents map[string]bool // events allowed in standard mode
}

// NewAuditWriter creates a writer that persists entries to the given store,
// filtered by the initial audit mode. The standardEvents set defines which
// events are logged in standard mode — events not in this set are only
// logged in detailed mode. Pass AuditOff as mode to start silent.
func NewAuditWriter(store *Store, mode AuditMode, standardEvents map[string]bool) *AuditWriter {
	w := &AuditWriter{store: store, standardEvents: standardEvents}
	w.mode.Store(mode)
	return w
}

// Write persists an audit entry if the current mode allows it.
// Drops silently when mode is off or the event is not in the
// current mode's allowlist. Logs a warning on DB write failure
// so callers don't need error handling.
func (w *AuditWriter) Write(entry Entry) {
	if w == nil || w.store == nil {
		return
	}

	mode := w.mode.Load().(AuditMode)

	if mode == AuditOff {
		return
	}
	if mode == AuditStandard && !w.standardEvents[entry.Event] {
		return
	}

	if err := w.store.Write(entry); err != nil {
		slog.Warn("failed to write audit log entry", "event", entry.Event, "err", err)
	}
}

// SetMode updates the audit mode at runtime. Takes effect immediately
// for subsequent Write calls.
func (w *AuditWriter) SetMode(mode AuditMode) {
	if w != nil {
		w.mode.Store(mode)
	}
}

// Mode returns the current audit mode.
func (w *AuditWriter) Mode() AuditMode {
	if w == nil {
		return AuditOff
	}
	return w.mode.Load().(AuditMode)
}
