//go:build !windows

package engine

import (
	"testing"
)

// TestHostTimezoneRespectsTZEnv verifies the TZ env var short-circuits all
// other resolution so the helper is easy to control in tests and from
// process launchers that override the zone explicitly.
func TestHostTimezoneRespectsTZEnv(t *testing.T) {
	t.Setenv("TZ", "Europe/Berlin")
	if got := hostTimezone(); got != "Europe/Berlin" {
		t.Fatalf("hostTimezone() = %q, want %q", got, "Europe/Berlin")
	}
}

// TestHostTimezoneTrimsWhitespace ensures surrounding whitespace from TZ (or
// a /etc/timezone file) does not leak into the generated env var, which
// would produce an invalid IANA name inside the container.
func TestHostTimezoneTrimsWhitespace(t *testing.T) {
	t.Setenv("TZ", "  Asia/Tokyo\n")
	if got := hostTimezone(); got != "Asia/Tokyo" {
		t.Fatalf("hostTimezone() = %q, want %q", got, "Asia/Tokyo")
	}
}
