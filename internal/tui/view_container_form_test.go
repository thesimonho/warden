package tui

import (
	"strings"
	"testing"

	"github.com/thesimonho/warden/api"
)

func TestFieldVisibility(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setup   func(v *ContainerFormView)
		field   int
		visible bool
	}{
		{
			name:    "domains visible when restricted",
			setup:   func(v *ContainerFormView) { v.network = 1 },
			field:   fieldDomains,
			visible: true,
		},
		{
			name:    "domains hidden when full",
			setup:   func(v *ContainerFormView) { v.network = 0 },
			field:   fieldDomains,
			visible: false,
		},
		{
			name:    "domains hidden when none",
			setup:   func(v *ContainerFormView) { v.network = 2 },
			field:   fieldDomains,
			visible: false,
		},
		{
			name:    "image hidden when advanced closed",
			setup:   func(v *ContainerFormView) { v.advancedOpen = false },
			field:   fieldImage,
			visible: false,
		},
		{
			name:    "image visible when advanced open",
			setup:   func(v *ContainerFormView) { v.advancedOpen = true },
			field:   fieldImage,
			visible: true,
		},
		{
			name:    "mounts hidden when advanced closed",
			setup:   func(v *ContainerFormView) { v.advancedOpen = false },
			field:   fieldMounts,
			visible: false,
		},
		{
			name:    "mounts visible when advanced open",
			setup:   func(v *ContainerFormView) { v.advancedOpen = true },
			field:   fieldMounts,
			visible: true,
		},
		{
			name:    "envvars hidden when advanced closed",
			setup:   func(v *ContainerFormView) { v.advancedOpen = false },
			field:   fieldEnvVars,
			visible: false,
		},
		{
			name:    "envvars visible when advanced open",
			setup:   func(v *ContainerFormView) { v.advancedOpen = true },
			field:   fieldEnvVars,
			visible: true,
		},
		{
			name:    "name always visible",
			setup:   func(v *ContainerFormView) {},
			field:   fieldName,
			visible: true,
		},
		{
			name:    "network always visible",
			setup:   func(v *ContainerFormView) {},
			field:   fieldNetwork,
			visible: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			v := &ContainerFormView{}
			v.initFields("", "", "", "full", "", false)
			tt.setup(v)
			got := v.isFieldVisible(tt.field)
			if got != tt.visible {
				t.Errorf("isFieldVisible(%d) = %v, want %v", tt.field, got, tt.visible)
			}
		})
	}
}

func TestMoveCursorSkipsHiddenFields(t *testing.T) {
	t.Parallel()

	v := &ContainerFormView{}
	v.initFields("", "", "", "full", "", false)
	v.cursor = fieldNetwork

	// Moving down from Network should skip Domains and land on Runtimes.
	v.moveCursor(1)
	if v.cursor != fieldRuntimes {
		t.Errorf("after moving down from Network: cursor=%d, want %d (Runtimes)", v.cursor, fieldRuntimes)
	}
}

func TestMoveCursorStaysInBounds(t *testing.T) {
	t.Parallel()

	v := &ContainerFormView{}
	v.initFields("", "", "", "full", "", false)

	// At first field, moving up should stay.
	v.cursor = fieldAgentType
	v.moveCursor(-1)
	if v.cursor != fieldAgentType {
		t.Errorf("at top, after move up: cursor=%d, want %d", v.cursor, fieldAgentType)
	}

	// At submit, moving down should stay.
	v.cursor = fieldSubmit
	v.moveCursor(1)
	if v.cursor != fieldSubmit {
		t.Errorf("at bottom, after move down: cursor=%d, want %d", v.cursor, fieldSubmit)
	}
}

func TestMountSubCursorNavigation(t *testing.T) {
	t.Parallel()

	v := &ContainerFormView{}
	v.initFields("", "", "", "full", "", false)
	v.advancedOpen = true
	v.mounts = []api.Mount{
		{HostPath: "/a", ContainerPath: "/b"},
		{HostPath: "/c", ContainerPath: "/d"},
	}
	v.cursor = fieldMounts
	v.mountCursor = -1

	// Move down into first mount item.
	v.moveCursor(1)
	if v.cursor != fieldMounts || v.mountCursor != 0 {
		t.Errorf("expected fieldMounts/0; got cursor=%d, mountCursor=%d", v.cursor, v.mountCursor)
	}

	// Move down to second mount.
	v.moveCursor(1)
	if v.mountCursor != 1 {
		t.Errorf("expected mountCursor=1, got %d", v.mountCursor)
	}

	// Move up back to first.
	v.moveCursor(-1)
	if v.mountCursor != 0 {
		t.Errorf("expected mountCursor=0, got %d", v.mountCursor)
	}

	// Move up to header (-1).
	v.moveCursor(-1)
	if v.mountCursor != -1 {
		t.Errorf("expected mountCursor=-1 (header), got %d", v.mountCursor)
	}
}

func TestRemoveMount(t *testing.T) {
	t.Parallel()

	v := &ContainerFormView{}
	v.initFields("", "", "", "full", "", false)
	v.advancedOpen = true
	v.mounts = []api.Mount{
		{HostPath: "/a", ContainerPath: "/b"},
		{HostPath: "/c", ContainerPath: "/d"},
	}
	v.cursor = fieldMounts
	v.mountCursor = 0

	v.removeCurrentItem()

	if len(v.mounts) != 1 {
		t.Errorf("expected 1 mount, got %d", len(v.mounts))
	}
	if v.mounts[0].HostPath != "/c" {
		t.Errorf("wrong mount remaining: %q", v.mounts[0].HostPath)
	}
}

func TestRemoveLastMount(t *testing.T) {
	t.Parallel()

	v := &ContainerFormView{}
	v.initFields("", "", "", "full", "", false)
	v.advancedOpen = true
	v.mounts = []api.Mount{
		{HostPath: "/a", ContainerPath: "/b"},
	}
	v.cursor = fieldMounts
	v.mountCursor = 0

	v.removeCurrentItem()

	if len(v.mounts) != 0 {
		t.Errorf("expected 0 mounts, got %d", len(v.mounts))
	}
	if v.mountCursor != -1 {
		t.Errorf("mountCursor should be -1 when empty, got %d", v.mountCursor)
	}
}

func TestRemoveEnvVar(t *testing.T) {
	t.Parallel()

	v := &ContainerFormView{}
	v.initFields("", "", "", "full", "", false)
	v.advancedOpen = true
	v.envVars = []envVarEntry{
		{key: "FOO", value: "bar"},
		{key: "BAZ", value: "qux"},
	}
	v.cursor = fieldEnvVars
	v.envCursor = 1

	v.removeCurrentItem()

	if len(v.envVars) != 1 {
		t.Errorf("expected 1 env var, got %d", len(v.envVars))
	}
	if v.envVars[0].key != "FOO" {
		t.Errorf("wrong env var remaining: %q", v.envVars[0].key)
	}
}

func TestSubmitValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setName string
		setPath string
		wantErr string
	}{
		{
			name:    "missing name",
			setName: "",
			setPath: "/some/path",
			wantErr: "container name is required",
		},
		{
			name:    "missing path",
			setName: "test",
			setPath: "",
			wantErr: "project path is required",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			v := &ContainerFormView{}
			v.initFields(tt.setName, tt.setPath, "img", "full", "", false)

			cmd := v.submit()
			if cmd != nil {
				t.Error("expected nil cmd for validation failure")
			}
			if v.err == nil {
				t.Fatal("expected validation error")
			}
			if v.err.Error() != tt.wantErr {
				t.Errorf("error = %q, want %q", v.err.Error(), tt.wantErr)
			}
		})
	}
}

func TestIsSensitiveKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		key  string
		want bool
	}{
		{"API_KEY", true},
		{"api_key", true},
		{"SECRET", true},
		{"my_secret_value", true},
		{"TOKEN", true},
		{"auth_token", true},
		{"PASSWORD", true},
		{"db_password", true},
		{"HOME", false},
		{"PATH", false},
		{"DEBUG", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			t.Parallel()
			got := isSensitiveKey(tt.key)
			if got != tt.want {
				t.Errorf("isSensitiveKey(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func TestFieldViewAgentType(t *testing.T) {
	t.Parallel()

	v := &ContainerFormView{}
	v.initFields("", "", "", "full", "", false)

	view := v.fieldView(fieldAgentType)
	if !strings.Contains(view, "[Claude Code]") {
		t.Errorf("default agent type: view=%q, want to contain [Claude Code]", view)
	}

	v.agentType = 1
	view = v.fieldView(fieldAgentType)
	if !strings.Contains(view, "[OpenAI Codex]") {
		t.Errorf("codex agent type: view=%q, want to contain [OpenAI Codex]", view)
	}
}

func TestFieldViewSkipPerms(t *testing.T) {
	t.Parallel()

	v := &ContainerFormView{}
	v.initFields("", "", "", "full", "", false)

	view := v.fieldView(fieldSkipPerms)
	if !strings.Contains(view, "[no]") {
		t.Errorf("skip perms off: view=%q, want to contain [no]", view)
	}

	v.skipPerm = true
	view = v.fieldView(fieldSkipPerms)
	if !strings.Contains(view, "[yes]") {
		t.Errorf("skip perms on: view=%q, want to contain [yes]", view)
	}
}

func TestFieldViewNetwork(t *testing.T) {
	t.Parallel()

	v := &ContainerFormView{}
	v.initFields("", "", "", "full", "", false)

	view := v.fieldView(fieldNetwork)
	if !strings.Contains(view, "[full]") {
		t.Errorf("network=full: view=%q, want to contain [full]", view)
	}

	v.network = 1
	view = v.fieldView(fieldNetwork)
	if !strings.Contains(view, "[restricted]") {
		t.Errorf("network=restricted: view=%q, want to contain [restricted]", view)
	}

	v.network = 2
	view = v.fieldView(fieldNetwork)
	if !strings.Contains(view, "[none]") {
		t.Errorf("network=none: view=%q, want to contain [none]", view)
	}
}

func TestRenderShowsLoading(t *testing.T) {
	t.Parallel()

	v := &ContainerFormView{loading: true}
	v.initFields("", "", "", "full", "", false)

	view := v.Render(80, 40)
	if !strings.Contains(view, "Loading") {
		t.Error("should show loading state")
	}
}

func TestRenderShowsFormTitle(t *testing.T) {
	t.Parallel()

	v := &ContainerFormView{}
	v.initFields("test", "/tmp", "img", "full", "", false)

	view := v.Render(80, 40)
	if !strings.Contains(view, "Create Project") {
		t.Error("create form should show Create Project title")
	}

	v.editID = "abc123"
	view = v.Render(80, 40)
	if !strings.Contains(view, "Edit Project") {
		t.Error("edit form should show Edit Project title")
	}
}

func TestRemoveDoesNothingOnWrongField(t *testing.T) {
	t.Parallel()

	v := &ContainerFormView{}
	v.initFields("", "", "", "full", "", false)
	v.cursor = fieldName

	// Should not panic or modify anything.
	v.removeCurrentItem()
}
