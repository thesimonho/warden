package components

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/thesimonho/warden/api"
)

// noopLoad returns a load function that does nothing (for tests that
// don't need async directory listing).
func noopLoad(path string) tea.Cmd { return nil }

func TestDirectoryBrowser_SetHeight(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		setHeight  int
		wantHeight int
	}{
		{"normal", 20, 20},
		{"minimum clamped", 1, 3},
		{"zero clamped", 0, 3},
		{"negative clamped", -5, 3},
		{"exact minimum", 3, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			b := NewDirectoryBrowser("/tmp", noopLoad)
			b.SetHeight(tt.setHeight)
			if b.height != tt.wantHeight {
				t.Errorf("SetHeight(%d): got height=%d, want %d", tt.setHeight, b.height, tt.wantHeight)
			}
		})
	}
}

func TestDirectoryBrowser_Path(t *testing.T) {
	t.Parallel()

	b := NewDirectoryBrowser("/home/user", noopLoad)
	if got := b.Path(); got != "/home/user" {
		t.Errorf("Path() = %q, want %q", got, "/home/user")
	}
}

func TestDirectoryBrowser_UpdateWithEntries(t *testing.T) {
	t.Parallel()

	b := NewDirectoryBrowser("/home", noopLoad)
	entries := []api.DirEntry{
		{Name: "alice", Path: "/home/alice"},
		{Name: "bob", Path: "/home/bob"},
	}

	b, _ = b.Update(DirectoryBrowserMsg{
		Path:    "/home",
		Entries: entries,
	})

	if b.loading {
		t.Error("expected loading=false after DirectoryBrowserMsg")
	}
	if len(b.entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(b.entries))
	}
	if b.cursor != 0 {
		t.Errorf("cursor should reset to 0, got %d", b.cursor)
	}
	if b.offset != 0 {
		t.Errorf("offset should reset to 0, got %d", b.offset)
	}
}

func TestDirectoryBrowser_UpdateWithError(t *testing.T) {
	t.Parallel()

	b := NewDirectoryBrowser("/home", noopLoad)
	b, _ = b.Update(DirectoryBrowserMsg{
		Err: errTestDir,
	})

	if b.loading {
		t.Error("expected loading=false after error")
	}
	if b.err == nil {
		t.Error("expected err to be set")
	}
}

var errTestDir = &testError{"directory not found"}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }

func TestDirectoryBrowser_KeyNavigation(t *testing.T) {
	t.Parallel()

	b := NewDirectoryBrowser("/home", noopLoad)
	b, _ = b.Update(DirectoryBrowserMsg{
		Path: "/home",
		Entries: []api.DirEntry{
			{Name: "a", Path: "/home/a"},
			{Name: "b", Path: "/home/b"},
			{Name: "c", Path: "/home/c"},
		},
	})

	// Cursor starts at 0 (parent dir).
	if b.cursor != 0 {
		t.Fatalf("initial cursor=%d, want 0", b.cursor)
	}

	// Move down.
	b, _ = b.Update(tea.KeyPressMsg{Code: 'j'})
	if b.cursor != 1 {
		t.Errorf("after j: cursor=%d, want 1", b.cursor)
	}

	// Move down again.
	b, _ = b.Update(tea.KeyPressMsg{Code: 'j'})
	if b.cursor != 2 {
		t.Errorf("after j: cursor=%d, want 2", b.cursor)
	}

	// Move up.
	b, _ = b.Update(tea.KeyPressMsg{Code: 'k'})
	if b.cursor != 1 {
		t.Errorf("after k: cursor=%d, want 1", b.cursor)
	}

	// Can't go below max.
	b.cursor = 3 // last entry
	b, _ = b.Update(tea.KeyPressMsg{Code: 'j'})
	if b.cursor != 3 {
		t.Errorf("at max, after j: cursor=%d, want 3", b.cursor)
	}

	// Can't go above 0.
	b.cursor = 0
	b, _ = b.Update(tea.KeyPressMsg{Code: 'k'})
	if b.cursor != 0 {
		t.Errorf("at 0, after k: cursor=%d, want 0", b.cursor)
	}
}

func TestDirectoryBrowser_ScrollingEnsureVisible(t *testing.T) {
	t.Parallel()

	b := NewDirectoryBrowser("/home", noopLoad)
	b.SetHeight(3) // only 3 visible rows

	entries := make([]api.DirEntry, 10)
	for i := range entries {
		entries[i] = api.DirEntry{Name: string(rune('a' + i)), Path: "/home/" + string(rune('a'+i))}
	}
	b, _ = b.Update(DirectoryBrowserMsg{Path: "/home", Entries: entries})

	// Move cursor past visible window.
	for i := 0; i < 5; i++ {
		b, _ = b.Update(tea.KeyPressMsg{Code: 'j'})
	}

	// Cursor should be visible: offset <= cursor < offset + height.
	if b.cursor < b.offset || b.cursor >= b.offset+b.height {
		t.Errorf("cursor=%d not visible in window [%d, %d)", b.cursor, b.offset, b.offset+b.height)
	}
}

func TestDirectoryBrowser_ViewShowsScrollIndicators(t *testing.T) {
	t.Parallel()

	b := NewDirectoryBrowser("/home", noopLoad)
	b.SetHeight(3)

	entries := make([]api.DirEntry, 10)
	for i := range entries {
		entries[i] = api.DirEntry{Name: string(rune('a' + i)), Path: "/home/" + string(rune('a'+i))}
	}
	b, _ = b.Update(DirectoryBrowserMsg{Path: "/home", Entries: entries})

	// At top: should show ↓ more but not ↑ more.
	view := b.View()
	if strings.Contains(view, "↑ more") {
		t.Error("at top, should not show ↑ more")
	}
	if !strings.Contains(view, "↓ more") {
		t.Error("at top with overflow, should show ↓ more")
	}

	// Move to middle.
	for i := 0; i < 5; i++ {
		b, _ = b.Update(tea.KeyPressMsg{Code: 'j'})
	}
	view = b.View()
	if !strings.Contains(view, "↑ more") {
		t.Error("in middle, should show ↑ more")
	}
	if !strings.Contains(view, "↓ more") {
		t.Error("in middle, should show ↓ more")
	}
}

func TestDirectoryBrowser_ViewShowsLoading(t *testing.T) {
	t.Parallel()

	b := NewDirectoryBrowser("/tmp", noopLoad)
	view := b.View()
	if !strings.Contains(view, "Loading") {
		t.Error("new browser should show Loading")
	}
}

func TestDirectoryBrowser_ViewShowsError(t *testing.T) {
	t.Parallel()

	b := NewDirectoryBrowser("/tmp", noopLoad)
	b, _ = b.Update(DirectoryBrowserMsg{Err: errTestDir})

	view := b.View()
	if !strings.Contains(view, "directory not found") {
		t.Error("should show error message")
	}
}

func TestDirectoryBrowser_NoScrollWhenFitsInHeight(t *testing.T) {
	t.Parallel()

	b := NewDirectoryBrowser("/home", noopLoad)
	b.SetHeight(20)

	entries := []api.DirEntry{
		{Name: "a", Path: "/home/a"},
		{Name: "b", Path: "/home/b"},
	}
	b, _ = b.Update(DirectoryBrowserMsg{Path: "/home", Entries: entries})

	view := b.View()
	if strings.Contains(view, "↑ more") || strings.Contains(view, "↓ more") {
		t.Error("should not show scroll indicators when all entries fit")
	}
}
