package tui

import "testing"

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
		{"has space", "my branch", true, "cannot contain spaces"},
		{"has tab", "my\tbranch", true, "cannot contain spaces"},
		{"starts with hyphen", "-feature", true, "cannot start with a hyphen"},
		{"starts with dot", ".hidden", true, "cannot start with a dot"},
		{"consecutive dots", "my..branch", true, "cannot contain consecutive dots"},
		{"tilde", "my~branch", true, "contains invalid characters"},
		{"caret", "my^branch", true, "contains invalid characters"},
		{"colon", "my:branch", true, "contains invalid characters"},
		{"question mark", "my?branch", true, "contains invalid characters"},
		{"asterisk", "my*branch", true, "contains invalid characters"},
		{"bracket", "my[branch", true, "contains invalid characters"},
		{"backslash", "my\\branch", true, "contains invalid characters"},
		{"at brace", "my@{branch", true, "contains invalid characters"},
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
