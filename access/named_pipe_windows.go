//go:build windows

package access

import (
	"time"

	"github.com/Microsoft/go-winio"
)

// namedPipeProbeTimeout is the maximum time to wait when verifying a
// Windows named pipe has a listener.
const namedPipeProbeTimeout = 500 * time.Millisecond

// probeNamedPipe attempts to connect to a Windows named pipe to verify
// it has an active listener. Stale or non-existent pipes fail quickly.
func probeNamedPipe(path string) bool {
	conn, err := winio.DialPipe(path, &namedPipeProbeTimeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
