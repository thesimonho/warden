package tui

import (
	"testing"
)

func TestServiceAdapterImplementsClient(t *testing.T) {
	// Compile-time check is already in adapter.go via:
	//   var _ Client = (*ServiceAdapter)(nil)
	// This test documents the intent explicitly.
	var _ Client = &ServiceAdapter{}
}

func TestNewServiceAdapter(t *testing.T) {
	// NewServiceAdapter should accept a nil app without panicking.
	// Operations will fail at call time, but construction is safe.
	adapter := NewServiceAdapter(nil)
	if adapter == nil {
		t.Fatal("NewServiceAdapter should return non-nil")
	}
}
