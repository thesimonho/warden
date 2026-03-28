package components

import (
	"testing"

	"github.com/thesimonho/warden/engine"
)

func TestWorktreeStateDot(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		state engine.WorktreeState
		want  string // the raw dot character (without ANSI)
	}{
		{"connected", engine.WorktreeStateConnected, "●"},
		{"shell", engine.WorktreeStateShell, "●"},
		{"background", engine.WorktreeStateBackground, "●"},
		{"disconnected", engine.WorktreeStateDisconnected, "○"},
		{"unknown", engine.WorktreeState("unknown"), "○"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := WorktreeStateDot(tt.state)
			// The result contains ANSI escape codes for color.
			// Verify it contains the expected dot character.
			if !containsRune(got, []rune(tt.want)[0]) {
				t.Errorf("WorktreeStateDot(%q) = %q, want to contain %q", tt.state, got, tt.want)
			}
		})
	}
}

func TestAttentionDot(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		notificationType engine.NotificationType
		wantEmpty        bool
		wantDot          string
	}{
		{"permission", engine.NotificationPermissionPrompt, false, "◉"},
		{"answer", engine.NotificationElicitationDialog, false, "◉"},
		{"input", engine.NotificationIdlePrompt, false, "◉"},
		{"empty", engine.NotificationType(""), true, ""},
		{"unknown", engine.NotificationType("unknown"), true, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := AttentionDot(tt.notificationType)
			if tt.wantEmpty && got != "" {
				t.Errorf("AttentionDot(%q) = %q, want empty", tt.notificationType, got)
			}
			if !tt.wantEmpty && !containsRune(got, []rune(tt.wantDot)[0]) {
				t.Errorf("AttentionDot(%q) = %q, want to contain %q", tt.notificationType, got, tt.wantDot)
			}
		})
	}
}

func TestContainerStateDot(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		state string
		want  string
	}{
		{"running", "running", "●"},
		{"exited", "exited", "●"},
		{"stopped", "stopped", "●"},
		{"not-found", "not-found", "○"},
		{"unknown", "", "○"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ContainerStateDot(tt.state)
			if !containsRune(got, []rune(tt.want)[0]) {
				t.Errorf("ContainerStateDot(%q) = %q, want to contain %q", tt.state, got, tt.want)
			}
		})
	}
}

// containsRune checks if the string contains a specific rune.
func containsRune(s string, r rune) bool {
	for _, c := range s {
		if c == r {
			return true
		}
	}
	return false
}
