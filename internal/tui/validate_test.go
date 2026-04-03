package tui

import "testing"

func TestSanitizeWorktreeName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"simple valid", "feature-auth", "feature-auth"},
		{"spaces to hyphens", "my branch", "my-branch"},
		{"tabs to hyphens", "my\tbranch", "my-branch"},
		{"leading hyphen stripped", "-feature", "feature"},
		{"leading dot stripped", ".hidden", "hidden"},
		{"consecutive dots collapsed", "my..branch", "my.branch"},
		{"tilde replaced", "my~branch", "my-branch"},
		{"colon replaced", "my:branch", "my-branch"},
		{"multiple invalid collapsed", "my~~^branch", "my-branch"},
		{"brackets replaced", "my[branch]", "my-branch-"},
		{"backslash replaced", "my\\branch", "my-branch"},
		{"leading dots and hyphens stripped", ".--.feature", "feature"},
		{"all invalid", "~^:?*", ""},
		{"already clean", "fix-login-bug", "fix-login-bug"},
		{"underscores preserved", "my_branch", "my_branch"},
		{"dots preserved", "fix.login", "fix.login"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := SanitizeWorktreeName(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeWorktreeName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidateWorktreeName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
		errMsg  string
	}{
		{"valid simple", "feature-auth", false, ""},
		{"valid with dots", "fix.login", false, ""},
		{"valid with underscores", "my_branch", false, ""},
		{"empty", "", true, "worktree name is required"},
		{"ends with .lock", "branch.lock", true, "cannot end with .lock"},
		{"ends with dot", "branch.", true, "cannot end with .lock or a dot"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateWorktreeName(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateWorktreeName(%q) = nil, want error containing %q", tt.input, tt.errMsg)
				} else if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateWorktreeName(%q) = %q, want error containing %q", tt.input, err.Error(), tt.errMsg)
				}
			} else if err != nil {
				t.Errorf("ValidateWorktreeName(%q) = %q, want nil", tt.input, err.Error())
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
