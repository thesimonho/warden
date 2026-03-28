package engine

import (
	"testing"
)

const testSeccompValue = "/tmp/test-seccomp.json"

func TestBuildSecurityConfig_FullMode(t *testing.T) {
	capDrop, capAdd, secOpts := buildSecurityConfig(NetworkModeFull, testSeccompValue)

	assertContains(t, capDrop, "ALL", "capDrop")
	assertNotContains(t, capAdd, "NET_ADMIN", "capAdd")
	assertHasBaseCapabilities(t, capAdd)
	assertHasSecurityOpts(t, secOpts)
}

func TestBuildSecurityConfig_RestrictedMode(t *testing.T) {
	capDrop, capAdd, secOpts := buildSecurityConfig(NetworkModeRestricted, testSeccompValue)

	assertContains(t, capDrop, "ALL", "capDrop")
	assertContains(t, capAdd, "NET_ADMIN", "capAdd")
	assertHasBaseCapabilities(t, capAdd)
	assertHasSecurityOpts(t, secOpts)
}

func TestBuildSecurityConfig_NoneMode(t *testing.T) {
	capDrop, capAdd, secOpts := buildSecurityConfig(NetworkModeNone, testSeccompValue)

	assertContains(t, capDrop, "ALL", "capDrop")
	assertContains(t, capAdd, "NET_ADMIN", "capAdd")
	assertHasBaseCapabilities(t, capAdd)
	assertHasSecurityOpts(t, secOpts)
}

func TestBuildSecurityConfig_DropsUnnecessaryCaps(t *testing.T) {
	_, capAdd, _ := buildSecurityConfig(NetworkModeFull, testSeccompValue)

	// These capabilities from Docker's default set should NOT be re-added.
	// AUDIT_WRITE was removed when the entrypoint switched from su (PAM) to gosu.
	shouldNotHave := []string{"SETPCAP", "MKNOD", "SETFCAP", "AUDIT_WRITE"}
	for _, cap := range shouldNotHave {
		assertNotContains(t, capAdd, cap, "capAdd")
	}
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

// assertHasSecurityOpts verifies both no-new-privileges and seccomp are present.
func assertHasSecurityOpts(t *testing.T, secOpts []string) {
	t.Helper()
	assertContains(t, secOpts, "no-new-privileges", "securityOpts")

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
