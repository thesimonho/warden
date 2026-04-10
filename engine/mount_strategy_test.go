package engine

import (
	"fmt"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/mount"

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

func TestBuildSocketMounts(t *testing.T) {
	t.Run("nil input returns nil", func(t *testing.T) {
		result := buildSocketMounts(nil)
		if result != nil {
			t.Fatalf("expected nil, got %d mounts", len(result))
		}
	})

	t.Run("converts to structured bind mounts", func(t *testing.T) {
		mounts := []api.Mount{
			{HostPath: "/run/host-services/ssh-auth.sock", ContainerPath: "/run/ssh-agent.sock"},
		}
		result := buildSocketMounts(mounts)
		if len(result) != 1 {
			t.Fatalf("expected 1 mount, got %d", len(result))
		}
		if result[0].Type != mount.TypeBind {
			t.Errorf("expected TypeBind, got %s", result[0].Type)
		}
		if result[0].Source != "/run/host-services/ssh-auth.sock" {
			t.Errorf("expected proxy source, got %s", result[0].Source)
		}
		if result[0].Target != "/run/ssh-agent.sock" {
			t.Errorf("expected container target, got %s", result[0].Target)
		}
		if result[0].ReadOnly {
			t.Error("expected read-write (connect() requires write permission on Unix sockets)")
		}
	})
}

func TestFindFailingMount(t *testing.T) {
	t.Parallel()

	mounts := []mount.Mount{
		{Type: mount.TypeBind, Source: "/run/user/1000/gnupg/S.gpg-agent", Target: "/home/warden/.gnupg/S.gpg-agent"},
		{Type: mount.TypeBind, Source: "/run/user/1000/ssh-agent.socket", Target: "/run/ssh-agent.sock"},
	}

	t.Run("matches first mount", func(t *testing.T) {
		t.Parallel()
		err := fmt.Errorf("invalid mount config for type \"bind\": bind source path does not exist: /run/user/1000/gnupg/S.gpg-agent")
		idx := findFailingMount(err, mounts)
		if idx != 0 {
			t.Errorf("expected index 0, got %d", idx)
		}
	})

	t.Run("matches second mount", func(t *testing.T) {
		t.Parallel()
		err := fmt.Errorf("invalid mount config for type \"bind\": bind source path does not exist: /run/user/1000/ssh-agent.socket")
		idx := findFailingMount(err, mounts)
		if idx != 1 {
			t.Errorf("expected index 1, got %d", idx)
		}
	})

	t.Run("no match returns -1", func(t *testing.T) {
		t.Parallel()
		err := fmt.Errorf("some other mount error")
		idx := findFailingMount(err, mounts)
		if idx != -1 {
			t.Errorf("expected -1, got %d", idx)
		}
	})

	t.Run("empty mounts returns -1", func(t *testing.T) {
		t.Parallel()
		err := fmt.Errorf("bind source path does not exist: /run/user/1000/ssh-agent.socket")
		idx := findFailingMount(err, nil)
		if idx != -1 {
			t.Errorf("expected -1, got %d", idx)
		}
	})
}

func TestIsFileSharingError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		msg  string
		want bool
	}{
		{
			name: "Docker Desktop file sharing error",
			msg:  `mounts denied: The path /nix/store/fb27361rphjqimgzb32ac1r4vys91zy5-hm_gitconfig is not shared from the host and is not known to Docker`,
			want: true,
		},
		{
			name: "unrelated mount error",
			msg:  `mount source path does not exist: /nonexistent`,
			want: false,
		},
		{
			name: "empty error",
			msg:  "",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := fmt.Errorf("%s", tt.msg)
			if got := isFileSharingError(err); got != tt.want {
				t.Errorf("isFileSharingError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFileSharingHint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		msg         string
		wantEmpty   bool
		wantContain []string
	}{
		{
			name:        "nix store path",
			msg:         `mounts denied: The path /nix/store/fb27361rphjqimgzb32ac1r4vys91zy5-hm_gitconfig is not shared from the host and is not known to Docker`,
			wantContain: []string{"/nix/store/fb27361", `"/nix"`},
		},
		{
			name:        "other unshared path",
			msg:         `mounts denied: The path /opt/custom/config is not shared from the host and is not known to Docker`,
			wantContain: []string{"/opt/custom/config", `"/opt"`},
		},
		{
			name:      "non-file-sharing error",
			msg:       `mount source path does not exist: /nonexistent`,
			wantEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			hint := fileSharingHint(fmt.Errorf("%s", tt.msg))
			if tt.wantEmpty {
				if hint != "" {
					t.Errorf("expected empty hint, got %q", hint)
				}
				return
			}
			for _, s := range tt.wantContain {
				if !strings.Contains(hint, s) {
					t.Errorf("hint %q should contain %q", hint, s)
				}
			}
		})
	}
}
