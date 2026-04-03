package engine

import (
	"os"
	"path/filepath"
	"testing"
)

// setupSymlinkTree creates a temporary directory tree with various symlink
// configurations for testing. Returns the root temp dir (cleaned up by t.Cleanup).
func setupSymlinkTree(t *testing.T) (mountDir, externalDir string) {
	t.Helper()

	// The "mount root" — simulates ~/.claude as mounted into the container.
	mountDir = t.TempDir()

	// An "external" directory — simulates /nix/store or ~/dotfiles,
	// i.e. paths that exist on the host but not inside the container.
	externalDir = t.TempDir()

	return mountDir, externalDir
}

// writeFile is a test helper that creates a file with the given content,
// creating parent directories as needed.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("creating parent dirs for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

// --- Regular files (no symlinks) ---

func TestResolveSymlinks_RegularFilesPassThrough(t *testing.T) {
	mountDir, _ := setupSymlinkTree(t)
	writeFile(t, filepath.Join(mountDir, "settings.json"), `{"key":"value"}`)
	writeFile(t, filepath.Join(mountDir, "subdir", "config.toml"), "x = 1")

	mounts := []Mount{{HostPath: mountDir, ContainerPath: "/home/warden/.claude"}}

	resolved, err := resolveSymlinksForMounts(mounts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// No extra mounts should be added — everything is a regular file.
	if len(resolved) != 1 {
		t.Errorf("expected 1 mount (original only), got %d", len(resolved))
	}
}

// --- Symlinks to files inside the mount tree ---

func TestResolveSymlinks_InternalFileSymlinkIgnored(t *testing.T) {
	mountDir, _ := setupSymlinkTree(t)
	writeFile(t, filepath.Join(mountDir, "real-settings.json"), `{"hooks":{}}`)
	if err := os.Symlink(
		filepath.Join(mountDir, "real-settings.json"),
		filepath.Join(mountDir, "settings.json"),
	); err != nil {
		t.Fatal(err)
	}

	mounts := []Mount{{HostPath: mountDir, ContainerPath: "/home/warden/.claude"}}

	resolved, err := resolveSymlinksForMounts(mounts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Internal symlink — target is inside the mount, no extra mount needed.
	if len(resolved) != 1 {
		t.Errorf("expected 1 mount, got %d", len(resolved))
	}
}

// --- Symlinks to files outside the mount tree ---

func TestResolveSymlinks_ExternalFileSymlinkResolved(t *testing.T) {
	mountDir, externalDir := setupSymlinkTree(t)
	writeFile(t, filepath.Join(externalDir, "settings.json"), `{"hooks":{"Stop":[]}}`)
	if err := os.Symlink(
		filepath.Join(externalDir, "settings.json"),
		filepath.Join(mountDir, "settings.json"),
	); err != nil {
		t.Fatal(err)
	}

	mounts := []Mount{{HostPath: mountDir, ContainerPath: "/home/warden/.claude"}}

	resolved, err := resolveSymlinksForMounts(mounts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should add an extra mount for the resolved file.
	hasExtraMount := false
	for _, m := range resolved {
		if m.HostPath == filepath.Join(externalDir, "settings.json") {
			hasExtraMount = true
			if m.ContainerPath != "/home/warden/.claude/settings.json" {
				t.Errorf("unexpected container path: %s", m.ContainerPath)
			}
		}
	}
	if !hasExtraMount {
		t.Errorf("expected extra mount for external symlink target, got mounts: %+v", resolved)
	}
}

// --- Symlinks to directories inside the mount tree ---

func TestResolveSymlinks_InternalDirSymlinkIgnored(t *testing.T) {
	mountDir, _ := setupSymlinkTree(t)
	realDir := filepath.Join(mountDir, "real-hooks")
	writeFile(t, filepath.Join(realDir, "hook.sh"), "#!/bin/bash")
	if err := os.Symlink(realDir, filepath.Join(mountDir, "hooks")); err != nil {
		t.Fatal(err)
	}

	mounts := []Mount{{HostPath: mountDir, ContainerPath: "/home/warden/.claude"}}

	resolved, err := resolveSymlinksForMounts(mounts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resolved) != 1 {
		t.Errorf("expected 1 mount, got %d", len(resolved))
	}
}

// --- Symlinks to directories outside the mount tree ---

func TestResolveSymlinks_ExternalDirSymlinkResolved(t *testing.T) {
	mountDir, externalDir := setupSymlinkTree(t)
	hooksDir := filepath.Join(externalDir, "hooks")
	writeFile(t, filepath.Join(hooksDir, "PreToolUse.sh"), "#!/bin/bash")

	if err := os.Symlink(hooksDir, filepath.Join(mountDir, "hooks")); err != nil {
		t.Fatal(err)
	}

	mounts := []Mount{{HostPath: mountDir, ContainerPath: "/home/warden/.claude"}}

	resolved, err := resolveSymlinksForMounts(mounts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hasExtraMount := false
	for _, m := range resolved {
		if m.HostPath == hooksDir {
			hasExtraMount = true
			if m.ContainerPath != "/home/warden/.claude/hooks" {
				t.Errorf("unexpected container path: %s", m.ContainerPath)
			}
		}
	}
	if !hasExtraMount {
		t.Errorf("expected extra mount for external dir symlink, got mounts: %+v", resolved)
	}
}

// --- Nested symlinks (symlink → symlink → real file) ---

func TestResolveSymlinks_NestedSymlinksFullyResolved(t *testing.T) {
	mountDir, externalDir := setupSymlinkTree(t)
	intermediateDir := t.TempDir()

	// external/settings.json is the real file
	writeFile(t, filepath.Join(externalDir, "settings.json"), `{"real":true}`)
	// intermediate/settings.json → external/settings.json
	if err := os.Symlink(
		filepath.Join(externalDir, "settings.json"),
		filepath.Join(intermediateDir, "settings.json"),
	); err != nil {
		t.Fatal(err)
	}
	// mount/settings.json → intermediate/settings.json
	if err := os.Symlink(
		filepath.Join(intermediateDir, "settings.json"),
		filepath.Join(mountDir, "settings.json"),
	); err != nil {
		t.Fatal(err)
	}

	mounts := []Mount{{HostPath: mountDir, ContainerPath: "/home/warden/.claude"}}

	resolved, err := resolveSymlinksForMounts(mounts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should resolve through the entire chain to the real file.
	hasExtraMount := false
	for _, m := range resolved {
		if m.HostPath == filepath.Join(externalDir, "settings.json") {
			hasExtraMount = true
		}
	}
	if !hasExtraMount {
		t.Errorf("expected mount pointing to final resolved target, got: %+v", resolved)
	}
}

// --- Broken symlink (target doesn't exist on host either) ---

func TestResolveSymlinks_BrokenSymlinkSkipped(t *testing.T) {
	mountDir, _ := setupSymlinkTree(t)
	if err := os.Symlink("/nonexistent/path/settings.json", filepath.Join(mountDir, "settings.json")); err != nil {
		t.Fatal(err)
	}

	mounts := []Mount{{HostPath: mountDir, ContainerPath: "/home/warden/.claude"}}

	resolved, err := resolveSymlinksForMounts(mounts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Broken symlink — target doesn't exist even on host. Skip gracefully.
	if len(resolved) != 1 {
		t.Errorf("expected 1 mount (original only), got %d: %+v", len(resolved), resolved)
	}
}

// --- Circular symlinks ---

func TestResolveSymlinks_CircularSymlinkSkipped(t *testing.T) {
	mountDir, _ := setupSymlinkTree(t)
	if err := os.Symlink(
		filepath.Join(mountDir, "b"),
		filepath.Join(mountDir, "a"),
	); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(
		filepath.Join(mountDir, "a"),
		filepath.Join(mountDir, "b"),
	); err != nil {
		t.Fatal(err)
	}

	mounts := []Mount{{HostPath: mountDir, ContainerPath: "/home/warden/.claude"}}

	resolved, err := resolveSymlinksForMounts(mounts)
	// Should not hang or crash.
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Circular symlinks can't be resolved — skip them.
	if len(resolved) != 1 {
		t.Errorf("expected 1 mount (original only), got %d", len(resolved))
	}
}

// --- Permission errors ---

func TestResolveSymlinks_UnreadableSymlinkSkipped(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("test requires non-root user")
	}

	mountDir, externalDir := setupSymlinkTree(t)
	secretFile := filepath.Join(externalDir, "secret.json")
	writeFile(t, secretFile, `{"token":"abc"}`)
	if err := os.Chmod(secretFile, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(secretFile, 0o644) }) //nolint:errcheck

	if err := os.Symlink(secretFile, filepath.Join(mountDir, "secret.json")); err != nil {
		t.Fatal(err)
	}

	mounts := []Mount{{HostPath: mountDir, ContainerPath: "/home/warden/.claude"}}

	resolved, err := resolveSymlinksForMounts(mounts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Symlink target exists but we can't read it — still add the mount
	// because Docker might run as a different user who can read it.
	// The key thing is we don't crash.
	_ = resolved
}

// --- Mount path itself is a symlink ---

func TestResolveSymlinks_MountRootIsSymlink(t *testing.T) {
	realDir := t.TempDir()
	writeFile(t, filepath.Join(realDir, "settings.json"), `{"root":true}`)

	symlinkDir := filepath.Join(t.TempDir(), "dot-claude")
	if err := os.Symlink(realDir, symlinkDir); err != nil {
		t.Fatal(err)
	}

	mounts := []Mount{{HostPath: symlinkDir, ContainerPath: "/home/warden/.claude"}}

	resolved, err := resolveSymlinksForMounts(mounts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The mount root itself being a symlink is fine — Docker resolves it
	// on the host side. No extra mounts needed since the contents are real files.
	if len(resolved) != 1 {
		t.Errorf("expected 1 mount, got %d", len(resolved))
	}
}

// --- Mount root is a symlink to a dir containing external symlinks ---

func TestResolveSymlinks_MountRootSymlinkWithExternalSymlinksInside(t *testing.T) {
	realDir := t.TempDir()
	externalDir := t.TempDir()

	writeFile(t, filepath.Join(realDir, ".credentials.json"), `{"token":"x"}`)
	writeFile(t, filepath.Join(externalDir, "settings.json"), `{"hooks":{}}`)
	if err := os.Symlink(
		filepath.Join(externalDir, "settings.json"),
		filepath.Join(realDir, "settings.json"),
	); err != nil {
		t.Fatal(err)
	}

	symlinkDir := filepath.Join(t.TempDir(), "dot-claude")
	if err := os.Symlink(realDir, symlinkDir); err != nil {
		t.Fatal(err)
	}

	mounts := []Mount{{HostPath: symlinkDir, ContainerPath: "/home/warden/.claude"}}

	resolved, err := resolveSymlinksForMounts(mounts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have original mount (resolved to realDir) + extra mount for settings.json.
	if len(resolved) != 2 {
		t.Errorf("expected 2 mounts, got %d: %+v", len(resolved), resolved)
	}

	// The original mount's host path should be the resolved real directory.
	if resolved[0].HostPath != realDir {
		t.Errorf("expected resolved host path %s, got %s", realDir, resolved[0].HostPath)
	}
}

// --- Mixed: some files are symlinks, some aren't ---

func TestResolveSymlinks_MixedSymlinksAndRegularFiles(t *testing.T) {
	mountDir, externalDir := setupSymlinkTree(t)

	// Regular files.
	writeFile(t, filepath.Join(mountDir, ".credentials.json"), `{"token":"x"}`)
	writeFile(t, filepath.Join(mountDir, ".claude.json"), `{"usage":{}}`)

	// External symlink — file.
	writeFile(t, filepath.Join(externalDir, "settings.json"), `{"hooks":{}}`)
	if err := os.Symlink(
		filepath.Join(externalDir, "settings.json"),
		filepath.Join(mountDir, "settings.json"),
	); err != nil {
		t.Fatal(err)
	}

	// External symlink — directory.
	hooksDir := filepath.Join(externalDir, "hooks")
	writeFile(t, filepath.Join(hooksDir, "stop.sh"), "#!/bin/bash")
	if err := os.Symlink(hooksDir, filepath.Join(mountDir, "hooks")); err != nil {
		t.Fatal(err)
	}

	mounts := []Mount{{HostPath: mountDir, ContainerPath: "/home/warden/.claude"}}

	resolved, err := resolveSymlinksForMounts(mounts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Original mount + 2 extra mounts (settings.json + hooks/).
	if len(resolved) != 3 {
		t.Errorf("expected 3 mounts, got %d: %+v", len(resolved), resolved)
	}
}

// --- Deeply nested directory with symlinks inside ---

func TestResolveSymlinks_SymlinksInSubdirectories(t *testing.T) {
	mountDir, externalDir := setupSymlinkTree(t)

	// Create a real subdirectory with a symlink inside it.
	subDir := filepath.Join(mountDir, "plugins", "cache")
	writeFile(t, filepath.Join(subDir, "local-plugin.json"), `{}`)

	// Symlink inside the subdirectory pointing outside.
	writeFile(t, filepath.Join(externalDir, "remote-plugin.json"), `{"remote":true}`)
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(
		filepath.Join(externalDir, "remote-plugin.json"),
		filepath.Join(subDir, "remote-plugin.json"),
	); err != nil {
		t.Fatal(err)
	}

	mounts := []Mount{{HostPath: mountDir, ContainerPath: "/home/warden/.claude"}}

	resolved, err := resolveSymlinksForMounts(mounts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should find the symlink inside the nested subdirectory.
	hasNestedMount := false
	for _, m := range resolved {
		if m.HostPath == filepath.Join(externalDir, "remote-plugin.json") {
			hasNestedMount = true
			expected := "/home/warden/.claude/plugins/cache/remote-plugin.json"
			if m.ContainerPath != expected {
				t.Errorf("expected container path %s, got %s", expected, m.ContainerPath)
			}
		}
	}
	if !hasNestedMount {
		t.Errorf("expected mount for nested symlink, got: %+v", resolved)
	}
}

// --- ReadOnly propagation ---

func TestResolveSymlinks_ReadOnlyPropagated(t *testing.T) {
	mountDir, externalDir := setupSymlinkTree(t)
	writeFile(t, filepath.Join(externalDir, "settings.json"), `{}`)
	if err := os.Symlink(
		filepath.Join(externalDir, "settings.json"),
		filepath.Join(mountDir, "settings.json"),
	); err != nil {
		t.Fatal(err)
	}

	mounts := []Mount{{
		HostPath:      mountDir,
		ContainerPath: "/home/warden/.claude",
		ReadOnly:      true,
	}}

	resolved, err := resolveSymlinksForMounts(mounts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, m := range resolved {
		if m.HostPath == filepath.Join(externalDir, "settings.json") {
			if !m.ReadOnly {
				t.Error("extra mount should inherit ReadOnly from parent mount")
			}
		}
	}
}

// --- Multiple mounts with independent symlinks ---

func TestResolveSymlinks_MultipleMountsIndependent(t *testing.T) {
	claudeDir, externalDir := setupSymlinkTree(t)
	sshDir := t.TempDir()

	// ~/.claude has an external symlink.
	writeFile(t, filepath.Join(externalDir, "settings.json"), `{}`)
	if err := os.Symlink(
		filepath.Join(externalDir, "settings.json"),
		filepath.Join(claudeDir, "settings.json"),
	); err != nil {
		t.Fatal(err)
	}

	// ~/.ssh has an external symlink.
	writeFile(t, filepath.Join(externalDir, "ssh-config"), "Host *")
	if err := os.Symlink(
		filepath.Join(externalDir, "ssh-config"),
		filepath.Join(sshDir, "config"),
	); err != nil {
		t.Fatal(err)
	}

	mounts := []Mount{
		{HostPath: claudeDir, ContainerPath: "/home/warden/.claude"},
		{HostPath: sshDir, ContainerPath: "/home/warden/.ssh", ReadOnly: true},
	}

	resolved, err := resolveSymlinksForMounts(mounts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 2 original mounts + 2 extra mounts.
	if len(resolved) != 4 {
		t.Errorf("expected 4 mounts, got %d: %+v", len(resolved), resolved)
	}
}

// --- Non-directory mount (single file mount) ---

func TestResolveSymlinks_SingleFileMountNotWalked(t *testing.T) {
	externalDir := t.TempDir()
	writeFile(t, filepath.Join(externalDir, "gitconfig"), "[user]\nname = Test")

	mounts := []Mount{{
		HostPath:      filepath.Join(externalDir, "gitconfig"),
		ContainerPath: "/home/warden/.gitconfig",
	}}

	resolved, err := resolveSymlinksForMounts(mounts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Single file mount — nothing to walk, pass through unchanged.
	if len(resolved) != 1 {
		t.Errorf("expected 1 mount, got %d", len(resolved))
	}
}

// --- Single file mount that IS a symlink ---

func TestResolveSymlinks_SingleFileSymlinkMountResolved(t *testing.T) {
	externalDir := t.TempDir()
	linkDir := t.TempDir()

	writeFile(t, filepath.Join(externalDir, "gitconfig"), "[user]\nname = Test")
	if err := os.Symlink(
		filepath.Join(externalDir, "gitconfig"),
		filepath.Join(linkDir, "gitconfig"),
	); err != nil {
		t.Fatal(err)
	}

	mounts := []Mount{{
		HostPath:      filepath.Join(linkDir, "gitconfig"),
		ContainerPath: "/home/warden/.gitconfig",
	}}

	resolved, err := resolveSymlinksForMounts(mounts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The host path should be resolved to the real file.
	if resolved[0].HostPath != filepath.Join(externalDir, "gitconfig") {
		t.Errorf("expected resolved host path %s, got %s",
			filepath.Join(externalDir, "gitconfig"), resolved[0].HostPath)
	}
}

// --- Symlinked directory should NOT be recursively walked ---

func TestResolveSymlinks_ExternalDirNotRecursivelyWalked(t *testing.T) {
	mountDir, externalDir := setupSymlinkTree(t)

	// External dir has nested content, including its own symlinks.
	deepExternal := t.TempDir()
	writeFile(t, filepath.Join(deepExternal, "deep.json"), `{}`)
	hooksDir := filepath.Join(externalDir, "hooks")
	writeFile(t, filepath.Join(hooksDir, "hook.sh"), "#!/bin/bash")
	if err := os.Symlink(
		filepath.Join(deepExternal, "deep.json"),
		filepath.Join(hooksDir, "deep-link.json"),
	); err != nil {
		t.Fatal(err)
	}

	// Mount dir symlinks to the hooks dir.
	if err := os.Symlink(hooksDir, filepath.Join(mountDir, "hooks")); err != nil {
		t.Fatal(err)
	}

	mounts := []Mount{{HostPath: mountDir, ContainerPath: "/home/warden/.claude"}}

	resolved, err := resolveSymlinksForMounts(mounts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// We should mount hooksDir at /home/warden/.claude/hooks, but NOT
	// recurse into it to resolve its internal symlinks — that would
	// be an unbounded walk of external filesystem trees.
	// The directory mount makes hooksDir's contents visible, including
	// any symlinks within it (which may or may not work depending on
	// whether THEIR targets are also mounted — that's the user's problem).
	extraMountCount := len(resolved) - 1
	if extraMountCount != 1 {
		t.Errorf("expected exactly 1 extra mount (the hooks dir), got %d: %+v",
			extraMountCount, resolved)
	}
}

// --- Empty directory that's a symlink ---

func TestResolveSymlinks_EmptyExternalDirSymlink(t *testing.T) {
	mountDir, externalDir := setupSymlinkTree(t)
	emptyDir := filepath.Join(externalDir, "empty")
	if err := os.MkdirAll(emptyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(emptyDir, filepath.Join(mountDir, "empty")); err != nil {
		t.Fatal(err)
	}

	mounts := []Mount{{HostPath: mountDir, ContainerPath: "/home/warden/.claude"}}

	resolved, err := resolveSymlinksForMounts(mounts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Empty external dir symlink should still get its own mount.
	hasEmptyMount := false
	for _, m := range resolved {
		if m.HostPath == emptyDir {
			hasEmptyMount = true
		}
	}
	if !hasEmptyMount {
		t.Errorf("expected mount for empty external dir, got: %+v", resolved)
	}
}

// --- Credentials file is a regular file and remains writable ---

func TestResolveSymlinks_CredentialsFileUntouched(t *testing.T) {
	mountDir, externalDir := setupSymlinkTree(t)
	writeFile(t, filepath.Join(mountDir, ".credentials.json"), `{"token":"x"}`)

	// Also add an external symlink to verify it doesn't interfere.
	writeFile(t, filepath.Join(externalDir, "settings.json"), `{}`)
	if err := os.Symlink(
		filepath.Join(externalDir, "settings.json"),
		filepath.Join(mountDir, "settings.json"),
	); err != nil {
		t.Fatal(err)
	}

	mounts := []Mount{{HostPath: mountDir, ContainerPath: "/home/warden/.claude"}}

	resolved, err := resolveSymlinksForMounts(mounts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Original mount should still be present (for credentials and other real files).
	if resolved[0].HostPath != mountDir {
		t.Errorf("first mount should be the original, got: %s", resolved[0].HostPath)
	}
	if resolved[0].ContainerPath != "/home/warden/.claude" {
		t.Errorf("first mount container path should be unchanged, got: %s", resolved[0].ContainerPath)
	}
}

// --- Nix Home Manager: triple symlink chain (config → nix store → nix store → dotfiles) ---

func TestResolveSymlinks_NixTripleSymlinkChain(t *testing.T) {
	mountDir := t.TempDir()  // ~/.codex
	nixStore1 := t.TempDir() // /nix/store/...-home-manager-files/.codex/
	nixStore2 := t.TempDir() // /nix/store/...-hm_config.toml
	dotfiles := t.TempDir()  // ~/dotfiles/AI/settings/codex/

	// The real writable file in dotfiles.
	writeFile(t, filepath.Join(dotfiles, "config.toml"), `model = "gpt-4"`)

	// Nix store file 2 → dotfiles (symlink to the real file).
	nixFile2 := filepath.Join(nixStore2, "hm_config.toml")
	if err := os.Symlink(filepath.Join(dotfiles, "config.toml"), nixFile2); err != nil {
		t.Fatal(err)
	}

	// Nix store file 1 → nix store file 2 (another symlink hop).
	nixConfigDir := filepath.Join(nixStore1, ".codex")
	if err := os.MkdirAll(nixConfigDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(nixFile2, filepath.Join(nixConfigDir, "config.toml")); err != nil {
		t.Fatal(err)
	}

	// Mount dir → nix store file 1 (first symlink in the chain).
	if err := os.Symlink(filepath.Join(nixConfigDir, "config.toml"), filepath.Join(mountDir, "config.toml")); err != nil {
		t.Fatal(err)
	}

	mounts := []Mount{{HostPath: mountDir, ContainerPath: "/home/warden/.codex"}}

	resolved, err := resolveSymlinksForMounts(mounts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should resolve through ALL hops to the real dotfiles path.
	hasMount := false
	for _, m := range resolved {
		if m.HostPath == filepath.Join(dotfiles, "config.toml") {
			hasMount = true
			if m.ContainerPath != "/home/warden/.codex/config.toml" {
				t.Errorf("expected container path /home/warden/.codex/config.toml, got %s", m.ContainerPath)
			}
		}
	}
	if !hasMount {
		t.Errorf("expected mount resolving through triple symlink chain to dotfiles, got: %+v", resolved)
	}
}

// --- Nix Home Manager: multiple config files + AGENTS.md split into subdirs ---

func TestResolveSymlinks_NixMultipleConfigFilesAndDirs(t *testing.T) {
	mountDir := t.TempDir()  // ~/.codex
	dotfiles := t.TempDir()  // ~/dotfiles/AI/

	// Real files in dotfiles.
	writeFile(t, filepath.Join(dotfiles, "config.toml"), `model = "gpt-4"`)
	writeFile(t, filepath.Join(dotfiles, "AGENTS.md"), "# Agent instructions")
	writeFile(t, filepath.Join(dotfiles, "skills", "tdd", "skill.md"), "# TDD Skill")
	writeFile(t, filepath.Join(dotfiles, "skills", "review", "skill.md"), "# Review Skill")

	// Symlinks from mount dir to dotfiles.
	if err := os.Symlink(filepath.Join(dotfiles, "config.toml"), filepath.Join(mountDir, "config.toml")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dotfiles, "AGENTS.md"), filepath.Join(mountDir, "AGENTS.md")); err != nil {
		t.Fatal(err)
	}
	// Directory symlink for skills.
	if err := os.Symlink(filepath.Join(dotfiles, "skills"), filepath.Join(mountDir, "skills")); err != nil {
		t.Fatal(err)
	}

	// Also some real (non-symlinked) content.
	writeFile(t, filepath.Join(mountDir, "sessions", "2026", "04", "01", "rollout.jsonl"), "{}")

	mounts := []Mount{{HostPath: mountDir, ContainerPath: "/home/warden/.codex"}}

	resolved, err := resolveSymlinksForMounts(mounts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Original + config.toml + AGENTS.md + skills/ = 4 mounts.
	if len(resolved) != 4 {
		t.Errorf("expected 4 mounts, got %d: %+v", len(resolved), resolved)
	}

	// Verify each external symlink is resolved.
	paths := make(map[string]string)
	for _, m := range resolved[1:] {
		paths[m.ContainerPath] = m.HostPath
	}

	if paths["/home/warden/.codex/config.toml"] != filepath.Join(dotfiles, "config.toml") {
		t.Errorf("config.toml not resolved correctly: %+v", paths)
	}
	if paths["/home/warden/.codex/AGENTS.md"] != filepath.Join(dotfiles, "AGENTS.md") {
		t.Errorf("AGENTS.md not resolved correctly: %+v", paths)
	}
	if paths["/home/warden/.codex/skills"] != filepath.Join(dotfiles, "skills") {
		t.Errorf("skills/ not resolved correctly: %+v", paths)
	}
}

// --- GNU Stow: symlinks one level deep (no nix store intermediary) ---

func TestResolveSymlinks_GNUStowPattern(t *testing.T) {
	mountDir := t.TempDir()  // ~/.claude
	dotfiles := t.TempDir()  // ~/dotfiles/.claude/

	// Stow creates direct symlinks from ~ to dotfiles.
	writeFile(t, filepath.Join(dotfiles, "settings.json"), `{"theme":"dark"}`)
	writeFile(t, filepath.Join(dotfiles, "hooks", "Notification.sh"), "#!/bin/bash")

	if err := os.Symlink(filepath.Join(dotfiles, "settings.json"), filepath.Join(mountDir, "settings.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dotfiles, "hooks"), filepath.Join(mountDir, "hooks")); err != nil {
		t.Fatal(err)
	}

	// Non-symlinked mutable data.
	writeFile(t, filepath.Join(mountDir, "projects", "proj1", "session.jsonl"), "{}")

	mounts := []Mount{{HostPath: mountDir, ContainerPath: "/home/warden/.claude"}}

	resolved, err := resolveSymlinksForMounts(mounts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Original + settings.json + hooks/ = 3.
	if len(resolved) != 3 {
		t.Errorf("expected 3 mounts, got %d: %+v", len(resolved), resolved)
	}
}

// --- Top-level tmp/ and log/ directories are skipped ---

func TestResolveSymlinks_TmpAndLogDirsSkipped(t *testing.T) {
	mountDir := t.TempDir()
	externalDir := t.TempDir()

	// Create tmp/ with external symlinks (simulates Codex sandbox binaries).
	tmpDir := filepath.Join(mountDir, "tmp", "path", "codex-arg0")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(externalDir, "codex-binary"), "binary")
	if err := os.Symlink(filepath.Join(externalDir, "codex-binary"), filepath.Join(tmpDir, "apply_patch")); err != nil {
		t.Fatal(err)
	}

	// Create log/ with some content.
	logDir := filepath.Join(mountDir, "log")
	writeFile(t, filepath.Join(logDir, "debug.log"), "log content")

	// Create a real config symlink that SHOULD be resolved.
	writeFile(t, filepath.Join(externalDir, "config.toml"), `model = "gpt-4"`)
	if err := os.Symlink(filepath.Join(externalDir, "config.toml"), filepath.Join(mountDir, "config.toml")); err != nil {
		t.Fatal(err)
	}

	mounts := []Mount{{HostPath: mountDir, ContainerPath: "/home/warden/.codex"}}

	resolved, err := resolveSymlinksForMounts(mounts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have original + config.toml only. tmp/ and its symlinks are skipped.
	if len(resolved) != 2 {
		t.Errorf("expected 2 mounts (original + config.toml), got %d: %+v", len(resolved), resolved)
	}

	// Verify the tmp symlink was NOT resolved.
	for _, m := range resolved {
		if m.HostPath == filepath.Join(externalDir, "codex-binary") {
			t.Error("tmp/ directory symlinks should be skipped, but apply_patch was resolved")
		}
	}
}

// --- Ordering: original mount comes first, then extras ---

func TestResolveSymlinks_OriginalMountFirstThenExtras(t *testing.T) {
	mountDir, externalDir := setupSymlinkTree(t)
	writeFile(t, filepath.Join(externalDir, "a.json"), `{}`)
	writeFile(t, filepath.Join(externalDir, "z.json"), `{}`)
	if err := os.Symlink(filepath.Join(externalDir, "z.json"), filepath.Join(mountDir, "z.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(externalDir, "a.json"), filepath.Join(mountDir, "a.json")); err != nil {
		t.Fatal(err)
	}

	mounts := []Mount{{HostPath: mountDir, ContainerPath: "/home/warden/.claude"}}

	resolved, err := resolveSymlinksForMounts(mounts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// First mount must be the original directory mount.
	if resolved[0].HostPath != mountDir {
		t.Errorf("first mount should be the original directory, got: %s", resolved[0].HostPath)
	}

	// Extra mounts come after. Docker processes mounts in order, so the
	// overlay mounts (which need to overlay files inside the directory mount)
	// must come after the directory mount.
	if len(resolved) != 3 {
		t.Fatalf("expected 3 mounts, got %d", len(resolved))
	}
}
