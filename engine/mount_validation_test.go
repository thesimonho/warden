package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// --- DetectStaleMounts ---

func TestDetectStaleMounts_NoChanges(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "settings.json"), `{"hooks":{}}`)

	original := []Mount{{HostPath: dir, ContainerPath: "/home/dev/.claude"}}

	// Resolve once (simulates container creation).
	current, err := resolveSymlinksForMounts(original)
	if err != nil {
		t.Fatal(err)
	}

	// Nothing changed — mounts should not be stale.
	stale := DetectStaleMounts(original, current)
	if len(stale) != 0 {
		t.Errorf("expected no stale mounts, got: %v", stale)
	}
}

func TestDetectStaleMounts_SymlinkTargetChanged(t *testing.T) {
	// A symlink inside a mounted directory points to an external target.
	// After creation, the symlink is updated to point somewhere else.
	// The old target still exists — only the symlink changed.

	oldTarget := t.TempDir()
	writeFile(t, filepath.Join(oldTarget, "config.json"), `{"version":1}`)

	newTarget := t.TempDir()
	writeFile(t, filepath.Join(newTarget, "config.json"), `{"version":2}`)

	mountDir := t.TempDir()
	if err := os.Symlink(
		filepath.Join(oldTarget, "config.json"),
		filepath.Join(mountDir, "config.json"),
	); err != nil {
		t.Fatal(err)
	}

	original := []Mount{{HostPath: mountDir, ContainerPath: "/home/dev/.claude"}}

	// Resolve at "creation time" — points to oldTarget.
	creationResolved, err := resolveSymlinksForMounts(original)
	if err != nil {
		t.Fatal(err)
	}

	// Switch the symlink to the new target (simulates dotfile manager switch).
	if err := os.Remove(filepath.Join(mountDir, "config.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(
		filepath.Join(newTarget, "config.json"),
		filepath.Join(mountDir, "config.json"),
	); err != nil {
		t.Fatal(err)
	}

	// The old target still exists, but the symlink changed.
	// Mounts should be detected as stale.
	stale := DetectStaleMounts(original, creationResolved)
	if len(stale) == 0 {
		t.Fatal("expected stale mounts after symlink target change, got none")
	}
}

func TestDetectStaleMounts_SymlinkTargetDeleted(t *testing.T) {
	// The resolved symlink target is deleted entirely (e.g. garbage collected).

	externalDir := t.TempDir()
	writeFile(t, filepath.Join(externalDir, "settings.json"), `{}`)

	mountDir := t.TempDir()
	if err := os.Symlink(
		filepath.Join(externalDir, "settings.json"),
		filepath.Join(mountDir, "settings.json"),
	); err != nil {
		t.Fatal(err)
	}

	original := []Mount{{HostPath: mountDir, ContainerPath: "/home/dev/.claude"}}
	creationResolved, err := resolveSymlinksForMounts(original)
	if err != nil {
		t.Fatal(err)
	}

	// Delete the external target (simulates garbage collection).
	if err := os.RemoveAll(externalDir); err != nil {
		t.Fatal(err)
	}

	stale := DetectStaleMounts(original, creationResolved)
	if len(stale) == 0 {
		t.Fatal("expected stale mounts after target deletion, got none")
	}
}

func TestDetectStaleMounts_NewSymlinkAppeared(t *testing.T) {
	// At creation time, a file was a regular file. Later, it becomes a
	// symlink to an external target (e.g. user starts managing it with
	// a dotfile manager). The container doesn't have the extra mount.

	mountDir := t.TempDir()
	writeFile(t, filepath.Join(mountDir, "settings.json"), `{"local":true}`)

	original := []Mount{{HostPath: mountDir, ContainerPath: "/home/dev/.claude"}}
	creationResolved, err := resolveSymlinksForMounts(original)
	if err != nil {
		t.Fatal(err)
	}

	// Replace the regular file with a symlink to an external target.
	externalDir := t.TempDir()
	writeFile(t, filepath.Join(externalDir, "settings.json"), `{"managed":true}`)
	if err := os.Remove(filepath.Join(mountDir, "settings.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(
		filepath.Join(externalDir, "settings.json"),
		filepath.Join(mountDir, "settings.json"),
	); err != nil {
		t.Fatal(err)
	}

	// Fresh resolution would produce an extra mount that doesn't exist
	// in the container's current binds.
	stale := DetectStaleMounts(original, creationResolved)
	if len(stale) == 0 {
		t.Fatal("expected stale mounts after new symlink appeared, got none")
	}
}

func TestDetectStaleMounts_MultipleMountsPartialStale(t *testing.T) {
	// Multiple mounts, only one has a changed symlink.

	externalA := t.TempDir()
	writeFile(t, filepath.Join(externalA, "a.json"), `{}`)
	externalB := t.TempDir()
	writeFile(t, filepath.Join(externalB, "b.json"), `{}`)
	newExternalA := t.TempDir()
	writeFile(t, filepath.Join(newExternalA, "a.json"), `{"new":true}`)

	claudeDir := t.TempDir()
	if err := os.Symlink(filepath.Join(externalA, "a.json"), filepath.Join(claudeDir, "a.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(externalB, "b.json"), filepath.Join(claudeDir, "b.json")); err != nil {
		t.Fatal(err)
	}

	sshDir := t.TempDir()
	writeFile(t, filepath.Join(sshDir, "config"), "Host *")

	original := []Mount{
		{HostPath: claudeDir, ContainerPath: "/home/dev/.claude"},
		{HostPath: sshDir, ContainerPath: "/home/dev/.ssh", ReadOnly: true},
	}
	creationResolved, err := resolveSymlinksForMounts(original)
	if err != nil {
		t.Fatal(err)
	}

	// Change only symlink A.
	if err := os.Remove(filepath.Join(claudeDir, "a.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(newExternalA, "a.json"), filepath.Join(claudeDir, "a.json")); err != nil {
		t.Fatal(err)
	}

	stale := DetectStaleMounts(original, creationResolved)
	if len(stale) != 1 {
		t.Fatalf("expected 1 stale mount, got %d: %v", len(stale), stale)
	}
	if stale[0] != "/home/dev/.claude/a.json" {
		t.Errorf("expected stale mount for /home/dev/.claude/a.json, got %s", stale[0])
	}
}

func TestDetectStaleMounts_DoubleSymlinkChain(t *testing.T) {
	// Dotfile managers often create chains: link → generation → actual file.
	// The resolver must resolve the full chain, and changes at any level
	// should be detected.

	actualFile := t.TempDir()
	writeFile(t, filepath.Join(actualFile, "settings.json"), `{"hooks":{}}`)

	genDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(genDir, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(
		filepath.Join(actualFile, "settings.json"),
		filepath.Join(genDir, "config", "settings.json"),
	); err != nil {
		t.Fatal(err)
	}

	mountDir := t.TempDir()
	if err := os.Symlink(
		filepath.Join(genDir, "config", "settings.json"),
		filepath.Join(mountDir, "settings.json"),
	); err != nil {
		t.Fatal(err)
	}

	original := []Mount{{HostPath: mountDir, ContainerPath: "/home/dev/.claude"}}
	creationResolved, err := resolveSymlinksForMounts(original)
	if err != nil {
		t.Fatal(err)
	}

	// Verify creation resolved through the full chain.
	hasActualFile := false
	for _, m := range creationResolved {
		if m.HostPath == filepath.Join(actualFile, "settings.json") {
			hasActualFile = true
		}
	}
	if !hasActualFile {
		t.Fatalf("expected resolution through full chain, got: %+v", creationResolved)
	}

	// Change the intermediate link (simulates generation switch).
	newActualFile := t.TempDir()
	writeFile(t, filepath.Join(newActualFile, "settings.json"), `{"hooks":{"Stop":[]}}`)

	newGenDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(newGenDir, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(
		filepath.Join(newActualFile, "settings.json"),
		filepath.Join(newGenDir, "config", "settings.json"),
	); err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(filepath.Join(mountDir, "settings.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(
		filepath.Join(newGenDir, "config", "settings.json"),
		filepath.Join(mountDir, "settings.json"),
	); err != nil {
		t.Fatal(err)
	}

	stale := DetectStaleMounts(original, creationResolved)
	if len(stale) == 0 {
		t.Fatal("expected stale mounts after chain change, got none")
	}
}

// --- StaleMountsError ---

func TestStaleMountsError_ImplementsError(t *testing.T) {
	err := &StaleMountsError{StalePaths: []string{"/home/dev/.claude/settings.json"}}
	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
}

func TestIsStaleMountsError(t *testing.T) {
	err := &StaleMountsError{StalePaths: []string{"/path"}}
	if !IsStaleMountsError(err) {
		t.Error("expected IsStaleMountsError to return true")
	}
	if IsStaleMountsError(fmt.Errorf("other error")) {
		t.Error("expected IsStaleMountsError to return false for other errors")
	}
}

// --- Encode/decode round-trip ---
