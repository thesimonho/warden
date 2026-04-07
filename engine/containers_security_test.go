package engine

import (
	"testing"
)

const testSeccompValue = "/tmp/test-seccomp.json"

func TestBuildSecurityConfig_Capabilities(t *testing.T) {
	capDrop, capAdd, _ := buildSecurityConfig(testSeccompValue)

	assertContains(t, capDrop, "ALL", "capDrop")
	assertHasBaseCapabilities(t, capAdd)

	// These capabilities must NOT be present. NET_ADMIN is applied
	// externally via privileged docker exec. Others are unnecessary
	// for a coding agent container.
	shouldNotHave := []string{"SETPCAP", "MKNOD", "SETFCAP", "AUDIT_WRITE", "NET_ADMIN"}
	for _, cap := range shouldNotHave {
		assertNotContains(t, capAdd, cap, "capAdd")
	}
}

func TestBuildSecurityConfig_SecurityOpts(t *testing.T) {
	_, _, secOpts := buildSecurityConfig(testSeccompValue)

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
