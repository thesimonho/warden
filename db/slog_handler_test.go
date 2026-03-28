package db

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"testing"
)

func TestResolveAttrValue_ErrorToString(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf("something went wrong: %s", "details")
	val := resolveAttrValue(slog.AnyValue(err))

	str, ok := val.(string)
	if !ok {
		t.Fatalf("expected string, got %T", val)
	}
	if str != "something went wrong: details" {
		t.Errorf("expected error message, got %q", str)
	}
}

func TestResolveAttrValue_ErrorIsJSONSerializable(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf("container failed: %s", "timeout")
	val := resolveAttrValue(slog.AnyValue(err))

	data, marshalErr := json.Marshal(val)
	if marshalErr != nil {
		t.Fatalf("json.Marshal failed: %v", marshalErr)
	}

	// Should be a JSON string, not an empty object.
	if string(data) == "{}" {
		t.Error("error serialized as empty object — resolveAttrValue did not convert to string")
	}

	var result string
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("expected JSON string, got: %s", string(data))
	}
	if result != "container failed: timeout" {
		t.Errorf("unexpected value: %q", result)
	}
}

func TestResolveAttrValue_NonErrorPassthrough(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		val  slog.Value
	}{
		{"string", slog.StringValue("hello")},
		{"int", slog.IntValue(42)},
		{"bool", slog.BoolValue(true)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolveAttrValue(tt.val)
			// Non-error values should pass through unchanged.
			if _, ok := result.(error); ok {
				t.Error("non-error value should not be converted")
			}
		})
	}
}

func TestSlogHandler_OffModeDropsEvents(t *testing.T) {
	t.Parallel()

	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer store.Close() //nolint:errcheck

	writer := NewAuditWriter(store, AuditOff, nil)
	handler := NewSlogHandler(slog.NewTextHandler(io.Discard, nil), writer)
	log := slog.New(handler)

	log.Error("this should be dropped", "key", "value")

	entries, err := store.Read()
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries in off mode, got %d", len(entries))
	}
}

func TestSlogHandler_ErrorAttrInAuditLog(t *testing.T) {
	t.Parallel()

	logger, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer logger.Close() //nolint:errcheck

	writer := NewAuditWriter(logger, AuditDetailed, nil)
	handler := NewSlogHandler(slog.NewTextHandler(io.Discard, nil), writer)
	log := slog.New(handler)

	log.Error("restart failed", "id", "abc123", "err", fmt.Errorf("container not found"))

	entries, err := logger.Read()
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	errVal, ok := entries[0].Attrs["err"]
	if !ok {
		t.Fatal("expected 'err' attr in entry")
	}

	errStr, ok := errVal.(string)
	if !ok {
		t.Fatalf("expected string attr, got %T", errVal)
	}
	if errStr != "container not found" {
		t.Errorf("expected 'container not found', got %q", errStr)
	}
}
