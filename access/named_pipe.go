//go:build !windows

package access

// probeNamedPipe is a no-op on non-Windows platforms. Windows named
// pipes don't exist outside Windows, so detection always fails.
func probeNamedPipe(_ string) bool {
	return false
}
