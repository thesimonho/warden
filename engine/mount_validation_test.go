package engine

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thesimonho/warden/api"
)

// --- DetectStaleMounts ---

func TestDetectStaleMounts_NoChanges(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "settings.json"), `{"hooks":{}}`)

	original := []api.Mount{{HostPath: dir, ContainerPath: "/home/warden/.claude"}}

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

	original := []api.Mount{{HostPath: mountDir, ContainerPath: "/home/warden/.claude"}}

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
	// The resolved symlink target is deleted entirely (e.g. nix store
	// garbage collected). The container still has the old mount pointing
	// to the deleted path, but the symlink on the host is also broken.
	// Since the broken state matches between host and container, there's
	// no benefit to blocking the restart — the fresh resolution simply
	// skips the broken symlink.

	externalDir := t.TempDir()
	writeFile(t, filepath.Join(externalDir, "settings.json"), `{}`)

	mountDir := t.TempDir()
	if err := os.Symlink(
		filepath.Join(externalDir, "settings.json"),
		filepath.Join(mountDir, "settings.json"),
	); err != nil {
		t.Fatal(err)
	}

	original := []api.Mount{{HostPath: mountDir, ContainerPath: "/home/warden/.claude"}}
	creationResolved, err := resolveSymlinksForMounts(original)
	if err != nil {
		t.Fatal(err)
	}

	// Delete the external target (simulates garbage collection).
	if err := os.RemoveAll(externalDir); err != nil {
		t.Fatal(err)
	}

	// The broken symlink is skipped during fresh resolution. The container
	// has an orphaned mount pointing to the deleted path, but this doesn't
	// block restart — the host has the same broken symlink.
	stale := DetectStaleMounts(original, creationResolved)
	if len(stale) != 0 {
		t.Errorf("expected no stale mounts when target is deleted (broken state matches host), got: %v", stale)
	}
}

func TestDetectStaleMounts_NewSymlinkAppeared(t *testing.T) {
	// At creation time, a file was a regular file. Later, it becomes a
	// symlink to an external target (e.g. user starts managing it with
	// a dotfile manager). The container doesn't have the extra mount.

	mountDir := t.TempDir()
	writeFile(t, filepath.Join(mountDir, "settings.json"), `{"local":true}`)

	original := []api.Mount{{HostPath: mountDir, ContainerPath: "/home/warden/.claude"}}
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

	original := []api.Mount{
		{HostPath: claudeDir, ContainerPath: "/home/warden/.claude"},
		{HostPath: sshDir, ContainerPath: "/home/warden/.ssh", ReadOnly: true},
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
	if stale[0] != "/home/warden/.claude/a.json" {
		t.Errorf("expected stale mount for /home/warden/.claude/a.json, got %s", stale[0])
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

	original := []api.Mount{{HostPath: mountDir, ContainerPath: "/home/warden/.claude"}}
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
	stalePath := "/home/warden/.claude/settings.json"
	err := &StaleMountsError{StalePaths: []string{stalePath}}
	msg := err.Error()
	if msg == "" {
		t.Fatal("expected non-empty error message")
	}
	// The error message should mention the stale path so users can diagnose the issue.
	if !strings.Contains(msg, stalePath) {
		t.Errorf("expected error to contain stale path %q, got %q", stalePath, msg)
	}
}

func TestIsStaleMountsError(t *testing.T) {
	stalePaths := []string{"/path/a", "/path/b"}
	err := &StaleMountsError{StalePaths: stalePaths}
	if !IsStaleMountsError(err) {
		t.Error("expected IsStaleMountsError to return true")
	}
	if IsStaleMountsError(fmt.Errorf("other error")) {
		t.Error("expected IsStaleMountsError to return false for other errors")
	}
	// Verify nil error doesn't panic.
	if IsStaleMountsError(nil) {
		t.Error("expected IsStaleMountsError to return false for nil")
	}

	// Verify the stale paths are preserved and accessible.
	var staleErr *StaleMountsError
	if errors.As(err, &staleErr) {
		if len(staleErr.StalePaths) != 2 {
			t.Errorf("expected 2 stale paths, got %d", len(staleErr.StalePaths))
		}
	} else {
		t.Error("expected errors.As to extract StaleMountsError")
	}
}

func TestDetectStaleMounts_ExtraMountsInCurrentIgnored(t *testing.T) {
	// Extra mounts in the container that aren't derived from original_mounts
	// (e.g. socket mounts, cache volumes) should NOT be flagged as stale.
	// This is a regression test for a bug where the SSH agent socket mount
	// was always flagged because it's tracked separately from original_mounts.

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "settings.json"), `{"hooks":{}}`)

	original := []api.Mount{{HostPath: dir, ContainerPath: "/home/warden/.claude"}}

	// Resolve once (simulates container creation).
	current, err := resolveSymlinksForMounts(original)
	if err != nil {
		t.Fatal(err)
	}

	// Append extra mounts that exist in the container but not in original_mounts.
	// These simulate socket mounts and other non-tracked bind mounts.
	current = append(current,
		api.Mount{HostPath: "/run/user/1000/ssh-agent.socket", ContainerPath: "/run/ssh-agent.sock"},
		api.Mount{HostPath: "/some/cache", ContainerPath: "/cache"},
	)

	stale := DetectStaleMounts(original, current)
	if len(stale) != 0 {
		t.Errorf("expected no stale mounts when extra mounts exist in container, got: %v", stale)
	}
}

// --- Encode/decode round-trip ---
