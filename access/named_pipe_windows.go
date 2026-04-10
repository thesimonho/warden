//go:build windows

package access

import "github.com/Microsoft/go-winio"

// probeNamedPipe attempts to connect to a Windows named pipe to verify
// it has an active listener. Stale or non-existent pipes fail quickly.
func probeNamedPipe(path string) bool {
	timeout := ProbeTimeout
	conn, err := winio.DialPipe(path, &timeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
