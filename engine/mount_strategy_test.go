package engine

import (
	"testing"

	"github.com/docker/docker/api/types/mount"

	"github.com/thesimonho/warden/api"
)

func TestBuildMounts(t *testing.T) {
	t.Run("project mount only", func(t *testing.T) {
		binds, structured, err := buildMounts("/home/user/project", "/home/warden/my-project", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(binds) != 1 {
			t.Fatalf("expected 1 bind, got %d", len(binds))
		}
		if binds[0] != "/home/user/project:/home/warden/my-project" {
			t.Errorf("unexpected bind: %s", binds[0])
		}
		if len(structured) != 0 {
			t.Fatalf("expected 0 structured mounts, got %d", len(structured))
		}
	})

	t.Run("project mount with additional mounts", func(t *testing.T) {
		mounts := []api.Mount{
			{HostPath: "/home/user/.claude", ContainerPath: "/home/warden/.claude", ReadOnly: true},
			{HostPath: "/home/user/.ssh", ContainerPath: "/home/warden/.ssh", ReadOnly: true},
		}
		binds, structured, err := buildMounts("/home/user/project", "/home/warden/my-project", mounts)
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
		if len(structured) != 0 {
			t.Fatalf("expected 0 structured mounts, got %d", len(structured))
		}
	})

	t.Run("socket mount uses structured API", func(t *testing.T) {
		mounts := []api.Mount{
			{HostPath: "/home/user/.claude", ContainerPath: "/home/warden/.claude", ReadOnly: true},
			{HostPath: "/tmp/ssh-agent.sock", ContainerPath: "/run/ssh-agent.sock", ReadOnly: true, IsSocket: true},
		}
		binds, structured, err := buildMounts("/home/user/project", "/home/warden/my-project", mounts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(binds) != 2 {
			t.Fatalf("expected 2 binds, got %d", len(binds))
		}
		if len(structured) != 1 {
			t.Fatalf("expected 1 structured mount, got %d", len(structured))
		}
		if structured[0].Type != mount.TypeBind {
			t.Errorf("expected TypeBind, got %s", structured[0].Type)
		}
		if structured[0].Source != "/tmp/ssh-agent.sock" {
			t.Errorf("expected socket source path, got %s", structured[0].Source)
		}
		if structured[0].Target != "/run/ssh-agent.sock" {
			t.Errorf("expected socket target path, got %s", structured[0].Target)
		}
		if !structured[0].ReadOnly {
			t.Error("expected socket mount to be read-only")
		}
	})

	t.Run("relative host path returns error", func(t *testing.T) {
		mounts := []api.Mount{
			{HostPath: "relative/.claude", ContainerPath: "/home/warden/.claude"},
		}
		_, _, err := buildMounts("/home/user/project", "/home/warden/my-project", mounts)
		if err == nil {
			t.Fatal("expected error for relative host path")
		}
	})

	t.Run("relative container path returns error", func(t *testing.T) {
		mounts := []api.Mount{
			{HostPath: "/home/user/.claude", ContainerPath: "relative/.claude"},
		}
		_, _, err := buildMounts("/home/user/project", "/home/warden/my-project", mounts)
		if err == nil {
			t.Fatal("expected error for relative container path")
		}
	})

	t.Run("empty mounts list", func(t *testing.T) {
		binds, structured, err := buildMounts("/home/user/project", "/home/warden/my-project", []api.Mount{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(binds) != 1 {
			t.Fatalf("expected 1 bind, got %d", len(binds))
		}
		if len(structured) != 0 {
			t.Fatalf("expected 0 structured mounts, got %d", len(structured))
		}
	})
}
