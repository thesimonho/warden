package db

import (
	"context"
	"log/slog"
	"strings"
	"unicode"
)

// SlogHandler is a slog.Handler that writes backend log records to the audit log.
// It wraps a delegate handler so logs still go to their original destination (stderr).
type SlogHandler struct {
	delegate slog.Handler
	writer   *AuditWriter
	attrs    []slog.Attr
	group    string
}

// NewSlogHandler creates a handler that writes to both the delegate and the audit log.
// If writer is nil, only the delegate receives records.
func NewSlogHandler(delegate slog.Handler, writer *AuditWriter) *SlogHandler {
	return &SlogHandler{delegate: delegate, writer: writer}
}

// Enabled reports whether the handler handles records at the given level.
func (h *SlogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.delegate.Enabled(ctx, level)
}

// Handle writes the record to the delegate and, if the writer is non-nil,
// also writes a backend entry to the audit log. Only warnings and errors
// are written — INFO-level traffic is too noisy. These events have
// auto-generated snake_case names and are classified as debug category
// (detailed mode only) by the AuditWriter's standardEvents allowlist.
func (h *SlogHandler) Handle(ctx context.Context, record slog.Record) error {
	// Always forward to the delegate first.
	if err := h.delegate.Handle(ctx, record); err != nil {
		return err
	}

	if h.writer == nil || record.Level < slog.LevelWarn || h.writer.Mode() == AuditOff {
		return nil
	}

	attrs := make(map[string]any, record.NumAttrs()+len(h.attrs))

	// Include pre-set attrs from WithAttrs.
	for _, a := range h.attrs {
		key := a.Key
		if h.group != "" {
			key = h.group + "." + key
		}
		attrs[key] = resolveAttrValue(a.Value)
	}

	// Include record attrs.
	record.Attrs(func(a slog.Attr) bool {
		key := a.Key
		if h.group != "" {
			key = h.group + "." + key
		}
		attrs[key] = resolveAttrValue(a.Value)
		return true
	})

	entry := Entry{
		Timestamp: record.Time.UTC(),
		Source:    SourceBackend,
		Level:     slogLevelToLevel(record.Level),
		Event:     toSnakeCase(record.Message),
		Message:   record.Message,
	}
	if len(attrs) > 0 {
		entry.Attrs = attrs
	}

	h.writer.Write(entry)
	return nil
}

// slogLevelToLevel maps a slog.Level to the audit log Level type.
func slogLevelToLevel(l slog.Level) Level {
	switch {
	case l >= slog.LevelError:
		return LevelError
	case l >= slog.LevelWarn:
		return LevelWarn
	default:
		return LevelInfo
	}
}

// toSnakeCase converts a human-readable message to a snake_case event identifier.
// Example: "container heartbeat stale" → "container_heartbeat_stale".
func toSnakeCase(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	prevUnderscore := false
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
			prevUnderscore = false
		} else if b.Len() > 0 && !prevUnderscore {
			b.WriteByte('_')
			prevUnderscore = true
		}
	}
	return strings.TrimRight(b.String(), "_")
}

// resolveAttrValue extracts a JSON-safe value from a slog.Value.
// Error values are converted to their string representation because
// encoding/json marshals error interfaces as empty objects ({}).
func resolveAttrValue(v slog.Value) any {
	resolved := v.Resolve()
	val := resolved.Any()
	if err, ok := val.(error); ok {
		return err.Error()
	}
	return val
}

// WithAttrs returns a new handler with the given attributes pre-set.
func (h *SlogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	combined := make([]slog.Attr, 0, len(h.attrs)+len(attrs))
	combined = append(combined, h.attrs...)
	combined = append(combined, attrs...)

	return &SlogHandler{
		delegate: h.delegate.WithAttrs(attrs),
		writer:   h.writer,
		attrs:    combined,
		group:    h.group,
	}
}

// WithGroup returns a new handler with the given group name.
func (h *SlogHandler) WithGroup(name string) slog.Handler {
	group := name
	if h.group != "" {
		group = h.group + "." + name
	}

	return &SlogHandler{
		delegate: h.delegate.WithGroup(name),
		writer:   h.writer,
		attrs:    h.attrs,
		group:    group,
	}
}
