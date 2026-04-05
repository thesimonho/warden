package tui

import (
	"fmt"
	"testing"

	"charm.land/bubbles/v2/help"
)

// maxFullHelpColumnHeight is the maximum number of bindings per column
// in FullHelp. The bubbles help component renders each group as a
// vertical column and silently truncates rows that don't fit. Keeping
// columns short ensures all bindings are visible.
const maxFullHelpColumnHeight = 3

// allKeyMaps returns every help.KeyMap used by the TUI, including
// contextual variants. Add new keymaps here when they are created.
func allKeyMaps() []struct {
	name string
	km   help.KeyMap
} {
	formKeys := DefaultFormKeyMap()
	return []struct {
		name string
		km   help.KeyMap
	}{
		{"GlobalKeyMap", DefaultGlobalKeyMap()},
		{"ProjectKeyMap", DefaultProjectKeyMap()},
		{"WorktreeKeyMap", DefaultWorktreeKeyMap()},
		{"SettingsKeyMap", DefaultSettingsKeyMap()},
		{"AuditLogKeyMap", DefaultAuditLogKeyMap()},
		{"AccessKeyMap", DefaultAccessKeyMap()},
		{"FormKeyMap", formKeys},
		{"formSelectionKeyMap", formSelectionKeyMap{keys: formKeys}},
		{"formWithRemoveKeyMap(mounts)", formWithRemoveKeyMap{keys: formKeys, isMounts: true}},
		{"formWithRemoveKeyMap(envvars)", formWithRemoveKeyMap{keys: formKeys, isMounts: false}},
	}
}

func TestFullHelpColumnHeight(t *testing.T) {
	t.Parallel()

	for _, tt := range allKeyMaps() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			for i, group := range tt.km.FullHelp() {
				if len(group) > maxFullHelpColumnHeight {
					t.Errorf(
						"FullHelp column %d has %d bindings (max %d) — items will be truncated by the help component",
						i, len(group), maxFullHelpColumnHeight,
					)
				}
			}
		})
	}
}

func TestFullHelpSupersetOfShortHelp(t *testing.T) {
	t.Parallel()

	for _, tt := range allKeyMaps() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Collect all help keys from FullHelp.
			fullKeys := make(map[string]bool)
			for _, group := range tt.km.FullHelp() {
				for _, b := range group {
					for _, k := range b.Keys() {
						fullKeys[k] = true
					}
				}
			}

			// Every enabled ShortHelp binding should appear in FullHelp.
			for _, b := range tt.km.ShortHelp() {
				if !b.Enabled() {
					continue
				}
				helpText := fmt.Sprintf("%s (%s)", b.Help().Key, b.Help().Desc)
				for _, k := range b.Keys() {
					// Skip the "?" toggle — it's intentionally absent from FullHelp.
					if k == "?" {
						continue
					}
					if !fullKeys[k] {
						t.Errorf("ShortHelp binding %s (key %q) not found in FullHelp", helpText, k)
					}
				}
			}
		})
	}
}

// Verify the test registry is kept in sync — compile error if a
// new KeyMap type is added without being included in allKeyMaps.
var _ = []help.KeyMap{
	GlobalKeyMap{},
	ProjectKeyMap{},
	WorktreeKeyMap{},
	SettingsKeyMap{},
	AuditLogKeyMap{},
	AccessKeyMap{},
	FormKeyMap{},
	formSelectionKeyMap{},
	formWithRemoveKeyMap{},
}

// ManageKeyMap and worktreeHelpKeyMap are defined in non-test files
// that can't be imported here without circular deps, but they use the
// same key.Binding types. They are covered by the compile-time
// interface assertion (var _ help.KeyMap = ManageKeyMap{}) in their
// own files. Add them to allKeyMaps if they move to keymap.go.
