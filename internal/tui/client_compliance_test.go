package tui_test

import (
	"testing"

	"github.com/thesimonho/warden/client"
	"github.com/thesimonho/warden/internal/tui"
)

// TestClientPackageSatisfiesInterface verifies that client.Client from
// the HTTP package satisfies the tui.Client interface. This is the key
// architectural check — both ServiceAdapter and client.Client must be
// interchangeable.
func TestClientPackageSatisfiesInterface(t *testing.T) {
	var _ tui.Client = (*client.Client)(nil)
}
