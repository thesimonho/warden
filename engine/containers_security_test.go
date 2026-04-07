package engine

import (
	"testing"

	"github.com/thesimonho/warden/api"
)

const testSeccompValue = "/tmp/test-seccomp.json"

func TestBuildSecurityConfig_FullMode(t *testing.T) {
	capDrop, capAdd, secOpts := buildSecurityConfig(api.NetworkModeFull, testSeccompValue)

	assertContains(t, capDrop, "ALL", "capDrop")
	assertNotContains(t, capAdd, "NET_ADMIN", "capAdd")
	assertHasBaseCapabilities(t, capAdd)
	assertHasSecurityOpts(t, secOpts)
}

func TestBuildSecurityConfig_RestrictedMode(t *testing.T) {
	capDrop, capAdd, secOpts := buildSecurityConfig(api.NetworkModeRestricted, testSeccompValue)

	assertContains(t, capDrop, "ALL", "capDrop")
	// NET_ADMIN is intentionally excluded — network isolation is applied
	// via privileged docker exec from the Go server.
	assertNotContains(t, capAdd, "NET_ADMIN", "capAdd")
	assertHasBaseCapabilities(t, capAdd)
	assertHasSecurityOpts(t, secOpts)
}

func TestBuildSecurityConfig_NoneMode(t *testing.T) {
	capDrop, capAdd, secOpts := buildSecurityConfig(api.NetworkModeNone, testSeccompValue)

	assertContains(t, capDrop, "ALL", "capDrop")
	// NET_ADMIN is intentionally excluded — network isolation is applied
	// via privileged docker exec from the Go server.
	assertNotContains(t, capAdd, "NET_ADMIN", "capAdd")
	assertHasBaseCapabilities(t, capAdd)
	assertHasSecurityOpts(t, secOpts)
}

func TestBuildSecurityConfig_DropsUnnecessaryCaps(t *testing.T) {
	_, capAdd, _ := buildSecurityConfig(api.NetworkModeFull, testSeccompValue)

	// These capabilities from Docker's default set should NOT be re-added.
	// AUDIT_WRITE was removed when the entrypoint switched from su (PAM) to gosu.
	// NET_ADMIN is applied externally via privileged docker exec.
	shouldNotHave := []string{"SETPCAP", "MKNOD", "SETFCAP", "AUDIT_WRITE", "NET_ADMIN"}
	for _, cap := range shouldNotHave {
		assertNotContains(t, capAdd, cap, "capAdd")
	}
}

func TestBuildSecurityConfig_NoNewPrivilegesRemoved(t *testing.T) {
	// no-new-privileges is intentionally absent so sudo (SUID binary) can
	// elevate to root for package installation. This is safe because the
	// tight bounding set (no NET_ADMIN, no SYS_ADMIN) limits what root can do.
	_, _, secOpts := buildSecurityConfig(api.NetworkModeFull, testSeccompValue)
	assertNotContains(t, secOpts, "no-new-privileges", "securityOpts")
}

func TestBuildSecurityConfig_AllModesIdentical(t *testing.T) {
	// All network modes produce the same security config — network isolation
	// is handled externally, not via container capabilities.
	fullDrop, fullAdd, fullOpts := buildSecurityConfig(api.NetworkModeFull, testSeccompValue)
	restrictedDrop, restrictedAdd, restrictedOpts := buildSecurityConfig(api.NetworkModeRestricted, testSeccompValue)
	noneDrop, noneAdd, noneOpts := buildSecurityConfig(api.NetworkModeNone, testSeccompValue)

	assertSlicesEqual(t, fullDrop, restrictedDrop, "capDrop full vs restricted")
	assertSlicesEqual(t, fullDrop, noneDrop, "capDrop full vs none")
	assertSlicesEqual(t, fullAdd, restrictedAdd, "capAdd full vs restricted")
	assertSlicesEqual(t, fullAdd, noneAdd, "capAdd full vs none")
	assertSlicesEqual(t, fullOpts, restrictedOpts, "secOpts full vs restricted")
	assertSlicesEqual(t, fullOpts, noneOpts, "secOpts full vs none")
}

// assertContains checks that the slice contains the expected value.
func assertContains(t *testing.T, slice []string, want, label string) {
	t.Helper()
	for _, v := range slice {
		if v == want {
			return
		}
	}
	t.Errorf("%s missing %q, got %v", label, want, slice)
}

// assertNotContains checks that the slice does NOT contain the value.
func assertNotContains(t *testing.T, slice []string, unwanted, label string) {
	t.Helper()
	for _, v := range slice {
		if v == unwanted {
			t.Errorf("%s should not contain %q", label, unwanted)
			return
		}
	}
}

// assertHasBaseCapabilities verifies all base capabilities are present.
func assertHasBaseCapabilities(t *testing.T, capAdd []string) {
	t.Helper()
	for _, cap := range baseCapabilities {
		assertContains(t, capAdd, cap, "capAdd")
	}
}

// assertHasSecurityOpts verifies the seccomp profile is present and
// no-new-privileges is absent.
func assertHasSecurityOpts(t *testing.T, secOpts []string) {
	t.Helper()
	// no-new-privileges must NOT be present (removed to enable sudo).
	assertNotContains(t, secOpts, "no-new-privileges", "securityOpts")

	want := "seccomp=" + testSeccompValue
	hasSeccomp := false
	for _, opt := range secOpts {
		if opt == want {
			hasSeccomp = true
			break
		}
	}
	if !hasSeccomp {
		t.Errorf("securityOpts missing seccomp value, got %v", secOpts)
	}
}

// assertSlicesEqual checks that two string slices have the same elements.
func assertSlicesEqual(t *testing.T, a, b []string, label string) {
	t.Helper()
	if len(a) != len(b) {
		t.Errorf("%s: length mismatch %d vs %d", label, len(a), len(b))
		return
	}
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("%s: element %d differs: %q vs %q", label, i, a[i], b[i])
		}
	}
}
