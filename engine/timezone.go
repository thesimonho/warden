//go:build !windows

package engine

import (
	"os"
	"strings"
)

// hostTimezone returns the host's IANA timezone name (e.g. "America/New_York")
// so the container can be configured to match the user's local time.
//
// Resolution order on Unix-like hosts:
//  1. The TZ environment variable if set.
//  2. /etc/timezone (Debian/Ubuntu, some other Linux distros).
//  3. The target of the /etc/localtime symlink (macOS, most Linux distros),
//     from which the IANA name is extracted by stripping the ".../zoneinfo/"
//     prefix.
//
// Returns an empty string when no timezone can be determined. Callers
// should omit the TZ env var in that case and let the container fall
// back to UTC. See timezone_windows.go for the Windows implementation.
func hostTimezone() string {
	if tz := strings.TrimSpace(os.Getenv("TZ")); tz != "" {
		return tz
	}

	if data, err := os.ReadFile("/etc/timezone"); err == nil {
		if tz := strings.TrimSpace(string(data)); tz != "" {
			return tz
		}
	}

	if target, err := os.Readlink("/etc/localtime"); err == nil {
		const marker = "zoneinfo/"
		if idx := strings.Index(target, marker); idx >= 0 {
			return target[idx+len(marker):]
		}
	}

	return ""
}
