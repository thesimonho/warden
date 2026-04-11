package constants

import "testing"

func TestTmuxSessionName(t *testing.T) {
	tests := []struct {
		worktreeID string
		want       string
	}{
		{"main", "warden-main"},
		{"feat-foo", "warden-feat-foo"},
		{"abc.123", "warden-abc.123"},
	}
	for _, tc := range tests {
		if got := TmuxSessionName(tc.worktreeID); got != tc.want {
			t.Errorf("TmuxSessionName(%q) = %q, want %q", tc.worktreeID, got, tc.want)
		}
	}
}

func TestTmuxShellSessionName(t *testing.T) {
	tests := []struct {
		worktreeID string
		want       string
	}{
		{"main", "warden-shell-main"},
		{"feat-foo", "warden-shell-feat-foo"},
	}
	for _, tc := range tests {
		if got := TmuxShellSessionName(tc.worktreeID); got != tc.want {
			t.Errorf("TmuxShellSessionName(%q) = %q, want %q", tc.worktreeID, got, tc.want)
		}
	}
}

// TestShellSessionNameDiffers asserts the shell and agent session names never
// collide for the same worktree — they must coexist in the same container.
func TestShellSessionNameDiffers(t *testing.T) {
	const wid = "main"
	if TmuxSessionName(wid) == TmuxShellSessionName(wid) {
		t.Fatalf("agent and shell session names collide for %q", wid)
	}
}
