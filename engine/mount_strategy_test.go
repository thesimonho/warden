package engine

import (
	"testing"

	"github.com/thesimonho/warden/api"
)

func TestBuildBindMounts(t *testing.T) {
	t.Run("project mount only", func(t *testing.T) {
		binds, err := buildBindMounts("/home/user/project", "/home/warden/my-project", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(binds) != 1 {
			t.Fatalf("expected 1 bind, got %d", len(binds))
		}
		if binds[0] != "/home/user/project:/home/warden/my-project" {
			t.Errorf("unexpected bind: %s", binds[0])
		}
	})

	t.Run("project mount with additional mounts", func(t *testing.T) {
		mounts := []api.Mount{
			{HostPath: "/home/user/.claude", ContainerPath: "/home/warden/.claude", ReadOnly: true},
			{HostPath: "/home/user/.ssh", ContainerPath: "/home/warden/.ssh", ReadOnly: true},
		}
		binds, err := buildBindMounts("/home/user/project", "/home/warden/my-project", mounts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(binds) != 3 {
			t.Fatalf("expected 3 binds, got %d", len(binds))
		}
		if binds[1] != "/home/user/.claude:/home/warden/.claude:ro" {
			t.Errorf("expected ro mount, got %q", binds[1])
		}
		if binds[2] != "/home/user/.ssh:/home/warden/.ssh:ro" {
			t.Errorf("expected ro mount, got %q", binds[2])
		}
	})

	t.Run("relative host path returns error", func(t *testing.T) {
		mounts := []api.Mount{
			{HostPath: "relative/.claude", ContainerPath: "/home/warden/.claude"},
		}
		_, err := buildBindMounts("/home/user/project", "/home/warden/my-project", mounts)
		if err == nil {
			t.Fatal("expected error for relative host path")
		}
	})

	t.Run("relative container path returns error", func(t *testing.T) {
		mounts := []api.Mount{
			{HostPath: "/home/user/.claude", ContainerPath: "relative/.claude"},
		}
		_, err := buildBindMounts("/home/user/project", "/home/warden/my-project", mounts)
		if err == nil {
			t.Fatal("expected error for relative container path")
		}
	})

	t.Run("empty mounts list", func(t *testing.T) {
		binds, err := buildBindMounts("/home/user/project", "/home/warden/my-project", []api.Mount{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(binds) != 1 {
			t.Fatalf("expected 1 bind, got %d", len(binds))
		}
	})
}
