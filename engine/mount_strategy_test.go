package engine

import (
	"testing"
)

func TestBuildBindMounts(t *testing.T) {
	t.Run("project mount only", func(t *testing.T) {
		binds, err := buildBindMounts("/home/user/project", "/home/dev/my-project", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(binds) != 1 {
			t.Fatalf("expected 1 bind, got %d", len(binds))
		}
		if binds[0] != "/home/user/project:/home/dev/my-project" {
			t.Errorf("unexpected bind: %s", binds[0])
		}
	})

	t.Run("project mount with additional mounts", func(t *testing.T) {
		mounts := []Mount{
			{HostPath: "/home/user/.claude", ContainerPath: "/home/dev/.claude", ReadOnly: true},
			{HostPath: "/home/user/.ssh", ContainerPath: "/home/dev/.ssh", ReadOnly: true},
		}
		binds, err := buildBindMounts("/home/user/project", "/home/dev/my-project", mounts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(binds) != 3 {
			t.Fatalf("expected 3 binds, got %d", len(binds))
		}
		if binds[1] != "/home/user/.claude:/home/dev/.claude:ro" {
			t.Errorf("expected ro mount, got %q", binds[1])
		}
		if binds[2] != "/home/user/.ssh:/home/dev/.ssh:ro" {
			t.Errorf("expected ro mount, got %q", binds[2])
		}
	})

	t.Run("relative host path returns error", func(t *testing.T) {
		mounts := []Mount{
			{HostPath: "relative/.claude", ContainerPath: "/home/dev/.claude"},
		}
		_, err := buildBindMounts("/home/user/project", "/home/dev/my-project", mounts)
		if err == nil {
			t.Fatal("expected error for relative host path")
		}
	})

	t.Run("relative container path returns error", func(t *testing.T) {
		mounts := []Mount{
			{HostPath: "/home/user/.claude", ContainerPath: "relative/.claude"},
		}
		_, err := buildBindMounts("/home/user/project", "/home/dev/my-project", mounts)
		if err == nil {
			t.Fatal("expected error for relative container path")
		}
	})

	t.Run("empty mounts list", func(t *testing.T) {
		binds, err := buildBindMounts("/home/user/project", "/home/dev/my-project", []Mount{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(binds) != 1 {
			t.Fatalf("expected 1 bind, got %d", len(binds))
		}
	})
}
