package engine

import (
	"fmt"
	"strings"
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
