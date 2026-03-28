package seccomp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestProfileJSON_ReturnsNonEmpty(t *testing.T) {
	got := ProfileJSON()
	if got == "" {
		t.Fatal("ProfileJSON() returned empty string")
	}
}

func TestProfileJSON_IsValidJSON(t *testing.T) {
	var raw json.RawMessage
	if err := json.Unmarshal([]byte(ProfileJSON()), &raw); err != nil {
		t.Fatalf("ProfileJSON() is not valid JSON: %v", err)
	}
}

func TestValidate_Succeeds(t *testing.T) {
	if err := Validate(); err != nil {
		t.Fatalf("Validate() failed: %v", err)
	}
}

func TestProfile_HasExpectedDefaultAction(t *testing.T) {
	var profile seccompProfile
	if err := json.Unmarshal(profileBytes, &profile); err != nil {
		t.Fatalf("failed to parse profile: %v", err)
	}
	if profile.DefaultAction != "SCMP_ACT_ALLOW" {
		t.Errorf("defaultAction = %q, want SCMP_ACT_ALLOW", profile.DefaultAction)
	}
}

func TestProfile_HasBothArchitectures(t *testing.T) {
	var profile seccompProfile
	if err := json.Unmarshal(profileBytes, &profile); err != nil {
		t.Fatalf("failed to parse profile: %v", err)
	}

	archSet := make(map[string]bool)
	for _, arch := range profile.Architectures {
		archSet[arch] = true
	}

	for _, want := range []string{"SCMP_ARCH_X86_64", "SCMP_ARCH_AARCH64"} {
		if !archSet[want] {
			t.Errorf("profile missing architecture %q", want)
		}
	}
}

func TestWriteProfileFile_CreatesValidFile(t *testing.T) {
	dir := t.TempDir()
	path, err := WriteProfileFile(dir)
	if err != nil {
		t.Fatalf("WriteProfileFile() error: %v", err)
	}

	wantPath := filepath.Join(dir, "seccomp.json")
	if path != wantPath {
		t.Errorf("path = %q, want %q", path, wantPath)
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading written file: %v", err)
	}

	if string(contents) != string(profileBytes) {
		t.Error("written file contents do not match embedded profile")
	}
}

func TestProfile_BlocksDangerousSyscalls(t *testing.T) {
	var profile seccompProfile
	if err := json.Unmarshal(profileBytes, &profile); err != nil {
		t.Fatalf("failed to parse profile: %v", err)
	}

	// Collect all denied syscalls across all rules.
	denied := make(map[string]bool)
	for _, rule := range profile.Syscalls {
		if rule.Action == "SCMP_ACT_ERRNO" {
			for _, name := range rule.Names {
				denied[name] = true
			}
		}
	}

	mustBlock := []string{
		"kexec_load",
		"kexec_file_load",
		"reboot",
		"init_module",
		"finit_module",
		"delete_module",
		"mount",
		"umount2",
		"pivot_root",
		"bpf",
		"userfaultfd",
		"open_by_handle_at",
	}

	for _, syscall := range mustBlock {
		if !denied[syscall] {
			t.Errorf("dangerous syscall %q is not blocked", syscall)
		}
	}
}
