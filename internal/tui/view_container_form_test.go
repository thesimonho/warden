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
			setup:   func(v *ContainerFormView) { v.step = stepNetwork; v.network = 1 },
			field:   netDomains,
			visible: true,
		},
		{
			name:    "domains hidden when full",
			setup:   func(v *ContainerFormView) { v.step = stepNetwork; v.network = 0 },
			field:   netDomains,
			visible: false,
		},
		{
			name:    "domains hidden when none",
			setup:   func(v *ContainerFormView) { v.step = stepNetwork; v.network = 2 },
			field:   netDomains,
			visible: false,
		},
		{
			name:    "name always visible on general step",
			setup:   func(v *ContainerFormView) { v.step = stepGeneral },
			field:   genName,
			visible: true,
		},
		{
			name:    "network always visible on network step",
			setup:   func(v *ContainerFormView) { v.step = stepNetwork },
			field:   netNetwork,
			visible: true,
		},
		{
			name:    "image always visible on advanced step",
			setup:   func(v *ContainerFormView) { v.step = stepAdvanced },
			field:   advImage,
			visible: true,
		},
		{
			name:    "mounts always visible on advanced step",
			setup:   func(v *ContainerFormView) { v.step = stepAdvanced },
			field:   advMounts,
			visible: true,
		},
		{
			name:    "envvars always visible on advanced step",
			setup:   func(v *ContainerFormView) { v.step = stepAdvanced },
			field:   advEnvVars,
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
	v.step = stepNetwork
	v.fieldCursor = netNetwork

	// Moving down from Network should skip Domains (hidden when mode=full)
	// and land on Ports.
	v.moveCursor(1)
	if v.fieldCursor != netPorts {
		t.Errorf("after moving down from Network: fieldCursor=%d, want %d (Ports)", v.fieldCursor, netPorts)
	}
}

func TestMoveCursorStaysInBounds(t *testing.T) {
	t.Parallel()

	v := &ContainerFormView{}
	v.initFields("", "", "", "full", "", false)

	// At first field of General, moving up should stay.
	v.step = stepGeneral
	v.fieldCursor = genAgentType
	v.moveCursor(-1)
	if v.fieldCursor != genAgentType {
		t.Errorf("at top, after move up: fieldCursor=%d, want %d", v.fieldCursor, genAgentType)
	}

	// At last field of Advanced, moving down should stay.
	v.step = stepAdvanced
	v.fieldCursor = advSubmit
	v.moveCursor(1)
	if v.fieldCursor != advSubmit {
		t.Errorf("at bottom, after move down: fieldCursor=%d, want %d", v.fieldCursor, advSubmit)
	}
}

func TestMountSubCursorNavigation(t *testing.T) {
	t.Parallel()

	v := &ContainerFormView{}
	v.initFields("", "", "", "full", "", false)
	v.step = stepAdvanced
	v.mounts = []api.Mount{
		{HostPath: "/a", ContainerPath: "/b"},
		{HostPath: "/c", ContainerPath: "/d"},
	}
	v.fieldCursor = advMounts
	v.mountCursor = -1

	// Move down into first mount item.
	v.moveCursor(1)
	if v.fieldCursor != advMounts || v.mountCursor != 0 {
		t.Errorf("expected advMounts/0; got fieldCursor=%d, mountCursor=%d", v.fieldCursor, v.mountCursor)
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
	v.step = stepAdvanced
	v.mounts = []api.Mount{
		{HostPath: "/a", ContainerPath: "/b"},
		{HostPath: "/c", ContainerPath: "/d"},
	}
	v.fieldCursor = advMounts
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
	v.step = stepAdvanced
	v.mounts = []api.Mount{
		{HostPath: "/a", ContainerPath: "/b"},
	}
	v.fieldCursor = advMounts
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
	v.step = stepAdvanced
	v.envVars = []envVarEntry{
		{key: "FOO", value: "bar"},
		{key: "BAZ", value: "qux"},
	}
	v.fieldCursor = advEnvVars
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
	v.step = stepGeneral

	view := v.fieldViewGeneral(genAgentType)
	if !strings.Contains(view, "[Claude Code]") {
		t.Errorf("default agent type: view=%q, want to contain [Claude Code]", view)
	}

	v.agentType = 1
	view = v.fieldViewGeneral(genAgentType)
	if !strings.Contains(view, "[OpenAI Codex]") {
		t.Errorf("codex agent type: view=%q, want to contain [OpenAI Codex]", view)
	}
}

func TestFieldViewSkipPerms(t *testing.T) {
	t.Parallel()

	v := &ContainerFormView{}
	v.initFields("", "", "", "full", "", false)
	v.step = stepGeneral

	view := v.fieldViewGeneral(genSkipPerms)
	if !strings.Contains(view, "[no]") {
		t.Errorf("skip perms off: view=%q, want to contain [no]", view)
	}

	v.skipPerm = true
	view = v.fieldViewGeneral(genSkipPerms)
	if !strings.Contains(view, "[yes]") {
		t.Errorf("skip perms on: view=%q, want to contain [yes]", view)
	}
}

func TestFieldViewNetwork(t *testing.T) {
	t.Parallel()

	v := &ContainerFormView{}
	v.initFields("", "", "", "full", "", false)
	v.step = stepNetwork

	view := v.fieldViewNetwork(netNetwork)
	if !strings.Contains(view, "[full]") {
		t.Errorf("network=full: view=%q, want to contain [full]", view)
	}

	v.network = 1
	view = v.fieldViewNetwork(netNetwork)
	if !strings.Contains(view, "[restricted]") {
		t.Errorf("network=restricted: view=%q, want to contain [restricted]", view)
	}

	v.network = 2
	view = v.fieldViewNetwork(netNetwork)
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

func TestRenderShowsStepBar(t *testing.T) {
	t.Parallel()

	v := &ContainerFormView{}
	v.initFields("test", "/tmp", "img", "full", "", false)

	view := v.Render(80, 40)
	if !strings.Contains(view, "General") {
		t.Error("should show General tab in step bar")
	}
	if !strings.Contains(view, "Environment") {
		t.Error("should show Environment tab in step bar")
	}
	if !strings.Contains(view, "Network") {
		t.Error("should show Network tab in step bar")
	}
	if !strings.Contains(view, "Advanced") {
		t.Error("should show Advanced tab in step bar")
	}
}

func TestRemoveDoesNothingOnWrongField(t *testing.T) {
	t.Parallel()

	v := &ContainerFormView{}
	v.initFields("", "", "", "full", "", false)
	v.step = stepGeneral
	v.fieldCursor = genName

	// Should not panic or modify anything.
	v.removeCurrentItem()
}

func TestStepSwitching(t *testing.T) {
	t.Parallel()

	v := &ContainerFormView{}
	v.initFields("", "", "", "full", "", false)
	v.step = stepGeneral
	v.fieldCursor = genBudget

	// Switch to next step — resets field cursor.
	v.switchStep(1)
	if v.step != stepEnvironment {
		t.Errorf("expected stepEnvironment, got %d", v.step)
	}
	if v.fieldCursor != 0 {
		t.Errorf("expected fieldCursor=0 after step switch, got %d", v.fieldCursor)
	}

	// Switch back.
	v.switchStep(-1)
	if v.step != stepGeneral {
		t.Errorf("expected stepGeneral, got %d", v.step)
	}

	// Can't go before first step.
	v.switchStep(-1)
	if v.step != stepGeneral {
		t.Errorf("expected to stay on stepGeneral, got %d", v.step)
	}

	// Jump to last step.
	v.step = stepAdvanced
	v.switchStep(1)
	if v.step != stepAdvanced {
		t.Errorf("expected to stay on stepAdvanced, got %d", v.step)
	}
}

func TestStepBadge(t *testing.T) {
	t.Parallel()

	v := &ContainerFormView{}
	v.initFields("", "", "", "full", "", false)

	// General: required when name is empty.
	badge := v.stepBadge(stepGeneral)
	if badge != "*" {
		t.Errorf("general badge with empty name: got %q, want '*'", badge)
	}

	// General: configured when name + path set.
	v.inputs[0].SetValue("my-project")
	v.inputs[1].SetValue("/tmp/proj")
	badge = v.stepBadge(stepGeneral)
	if badge != "✓" {
		t.Errorf("general badge with name+path: got %q, want '✓'", badge)
	}

	// Network: always configured.
	badge = v.stepBadge(stepNetwork)
	if badge != "✓" {
		t.Errorf("network badge: got %q, want '✓'", badge)
	}

	// Environment: empty when no runtimes or access items.
	badge = v.stepBadge(stepEnvironment)
	if badge != "" {
		t.Errorf("environment badge with no items: got %q, want ''", badge)
	}

	// Environment: configured when runtimes loaded.
	v.runtimeDefaults = []api.RuntimeDefault{{ID: "node", Label: "Node.js"}}
	badge = v.stepBadge(stepEnvironment)
	if badge != "✓" {
		t.Errorf("environment badge with runtimes: got %q, want '✓'", badge)
	}
}

func TestStepSummary(t *testing.T) {
	t.Parallel()

	v := &ContainerFormView{}
	v.initFields("", "", "", "full", "", false)
	v.runtimeToggles = make(map[string]bool)

	// General: setup required when empty.
	summary := v.stepSummary(stepGeneral)
	if summary != "Setup required" {
		t.Errorf("general summary empty: got %q, want 'Setup required'", summary)
	}

	// General: shows agent + name when filled.
	v.inputs[0].SetValue("my-project")
	v.inputs[1].SetValue("/tmp/proj")
	summary = v.stepSummary(stepGeneral)
	if !strings.Contains(summary, "my-project") {
		t.Errorf("general summary with name: got %q, want to contain 'my-project'", summary)
	}

	// Network: reflects mode.
	summary = v.stepSummary(stepNetwork)
	if summary != "Full access" {
		t.Errorf("network summary: got %q, want 'Full access'", summary)
	}
	v.network = 1
	summary = v.stepSummary(stepNetwork)
	if summary != "Restricted" {
		t.Errorf("network summary restricted: got %q, want 'Restricted'", summary)
	}

	// Advanced: defaults applied when nothing customized.
	v.inputs[2].SetValue("ghcr.io/thesimonho/warden:latest")
	summary = v.stepSummary(stepAdvanced)
	if summary != "Defaults applied" {
		t.Errorf("advanced summary default: got %q, want 'Defaults applied'", summary)
	}
}

func TestSubmitFromAnyStep(t *testing.T) {
	t.Parallel()

	// Submit reads all state fields regardless of current step.
	v := &ContainerFormView{}
	v.initFields("test", "/tmp/proj", "img", "full", "", false)
	v.runtimeToggles = make(map[string]bool)
	v.accessToggles = make(map[string]bool)

	// Set step to Environment but submit should still validate name+path from General.
	v.step = stepEnvironment
	cmd := v.submit()

	// Should proceed (non-nil cmd) because name and path are set.
	if cmd == nil {
		t.Errorf("expected submit to produce a command from non-General step, got nil (err: %v)", v.err)
	}
}
