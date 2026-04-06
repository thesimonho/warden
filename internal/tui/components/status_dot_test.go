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
		{"stopped", engine.WorktreeStateStopped, "○"},
		{"unknown", engine.WorktreeState("unknown"), "○"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := WorktreeStateDot(tt.state)
			if got == "" {
				t.Errorf("WorktreeStateDot(%q) returned empty string", tt.state)
			}
			if !containsRune(got, []rune(tt.want)[0]) {
				t.Errorf("WorktreeStateDot(%q) = %q, want to contain %q", tt.state, got, tt.want)
			}
			// Verify active states use filled dot, stopped/unknown use hollow.
			if tt.state == engine.WorktreeStateStopped || tt.state == engine.WorktreeState("unknown") {
				if containsRune(got, '●') {
					t.Errorf("WorktreeStateDot(%q) should use hollow ○, not filled ●", tt.state)
				}
			} else {
				if containsRune(got, '○') {
					t.Errorf("WorktreeStateDot(%q) should use filled ●, not hollow ○", tt.state)
				}
			}
		})
	}
}

func TestWorktreeStateDot_UniqueStylesPerState(t *testing.T) {
	t.Parallel()

	// Each active state should produce a visually distinct output.
	connected := WorktreeStateDot(engine.WorktreeStateConnected)
	shell := WorktreeStateDot(engine.WorktreeStateShell)
	background := WorktreeStateDot(engine.WorktreeStateBackground)
	stopped := WorktreeStateDot(engine.WorktreeStateStopped)

	if connected == shell {
		t.Error("connected and shell should have different styling")
	}
	if connected == background {
		t.Error("connected and background should have different styling")
	}
	if shell == background {
		t.Error("shell and background should have different styling")
	}
	if connected == stopped {
		t.Error("connected and stopped should have different styling")
	}
}

func TestAttentionDot(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		notificationType engine.NotificationType
		wantEmpty        bool
	}{
		{"permission", engine.NotificationPermissionPrompt, false},
		{"answer", engine.NotificationElicitationDialog, false},
		{"input", engine.NotificationIdlePrompt, false},
		{"empty", engine.NotificationType(""), true},
		{"unknown", engine.NotificationType("unknown"), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := AttentionDot(tt.notificationType)
			if tt.wantEmpty && got != "" {
				t.Errorf("AttentionDot(%q) = %q, want empty", tt.notificationType, got)
			}
			if !tt.wantEmpty {
				if got == "" {
					t.Errorf("AttentionDot(%q) returned empty, want non-empty", tt.notificationType)
				}
				if !containsRune(got, '◉') {
					t.Errorf("AttentionDot(%q) = %q, want to contain ◉", tt.notificationType, got)
				}
			}
		})
	}
}

func TestAttentionDot_UniqueStylesPerType(t *testing.T) {
	t.Parallel()

	permission := AttentionDot(engine.NotificationPermissionPrompt)
	elicitation := AttentionDot(engine.NotificationElicitationDialog)
	idle := AttentionDot(engine.NotificationIdlePrompt)

	if permission == elicitation {
		t.Error("permission and elicitation should have different styling")
	}
	if permission == idle {
		t.Error("permission and idle should have different styling")
	}
	if elicitation == idle {
		t.Error("elicitation and idle should have different styling")
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
			if got == "" {
				t.Errorf("ContainerStateDot(%q) returned empty string", tt.state)
			}
			if !containsRune(got, []rune(tt.want)[0]) {
				t.Errorf("ContainerStateDot(%q) = %q, want to contain %q", tt.state, got, tt.want)
			}
		})
	}
}

func TestContainerStateDot_RunningDistinctFromExited(t *testing.T) {
	t.Parallel()

	running := ContainerStateDot("running")
	exited := ContainerStateDot("exited")
	notFound := ContainerStateDot("not-found")

	if running == exited {
		t.Error("running and exited should have different styling")
	}
	if running == notFound {
		t.Error("running and not-found should have different styling")
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
