package components

import (
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/thesimonho/warden/api"
)

var (
	dirSelectedStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorAccent)
	dirNormalStyle   = lipgloss.NewStyle()
	dirMutedStyle    = lipgloss.NewStyle().Foreground(ColorGray)
	dirErrorStyle    = lipgloss.NewStyle().Foreground(ColorError)
)

// DirectoryBrowserMsg carries the result of a directory listing.
type DirectoryBrowserMsg struct {
	Path    string
	Entries []api.DirEntry
	Err     error
}

// DirectorySelectedMsg is sent when the user confirms a directory.
type DirectorySelectedMsg struct {
	Path string
}

// DirectoryBrowser is a navigable filesystem tree for selecting a directory.
// Supports scrolling when the entry list exceeds the visible height.
type DirectoryBrowser struct {
	currentPath string
	entries     []api.DirEntry
	cursor      int
	loading     bool
	err         error
	height      int // visible rows for entries (0 = unlimited)
	offset      int // scroll offset into entries list
	// loadFn performs the async directory listing.
	loadFn func(path string) tea.Cmd
}

// NewDirectoryBrowser creates a browser starting at the given path.
// loadFn should call client.ListDirectories and return a DirectoryBrowserMsg.
func NewDirectoryBrowser(startPath string, loadFn func(string) tea.Cmd) *DirectoryBrowser {
	return &DirectoryBrowser{
		currentPath: startPath,
		loading:     true,
		loadFn:      loadFn,
	}
}

// SetHeight sets the maximum number of visible entry rows.
// The header and footer take 4 lines, so the caller should subtract
// those from the available height before calling this.
func (b *DirectoryBrowser) SetHeight(h int) {
	if h < 3 {
		h = 3
	}
	b.height = h
}

// Init loads the initial directory listing.
func (b *DirectoryBrowser) Init() tea.Cmd {
	return b.loadFn(b.currentPath)
}

// Path returns the currently selected path.
func (b *DirectoryBrowser) Path() string {
	return b.currentPath
}

// Update handles messages for the directory browser.
func (b *DirectoryBrowser) Update(msg tea.Msg) (*DirectoryBrowser, tea.Cmd) {
	switch msg := msg.(type) {
	case DirectoryBrowserMsg:
		b.loading = false
		if msg.Err != nil {
			b.err = msg.Err
			return b, nil
		}
		b.entries = msg.Entries
		b.cursor = 0
		b.offset = 0
		b.currentPath = msg.Path
		return b, nil

	case tea.KeyPressMsg:
		return b.handleKey(msg)
	}
	return b, nil
}

// View renders the directory browser with scrolling support.
func (b *DirectoryBrowser) View() string {
	var s strings.Builder

	s.WriteString(dirMutedStyle.Render("Directory: "))
	s.WriteString(dirSelectedStyle.Render(b.currentPath))
	s.WriteString("\n\n")

	if b.loading {
		s.WriteString("Loading...")
		return s.String()
	}
	if b.err != nil {
		s.WriteString(dirErrorStyle.Render("Error: " + b.err.Error()))
		return s.String()
	}

	// Build all items: parent (..) + entries.
	totalItems := 1 + len(b.entries)
	visibleHeight := totalItems
	if b.height > 0 && visibleHeight > b.height {
		visibleHeight = b.height
	}

	// Render visible window of items.
	for i := b.offset; i < b.offset+visibleHeight && i < totalItems; i++ {
		isSelected := i == b.cursor
		cursor := "  "
		if isSelected {
			cursor = "> "
		}

		if i == 0 {
			// Parent directory.
			s.WriteString(cursor)
			s.WriteString(dirMutedStyle.Render("../ (parent)"))
		} else {
			entry := b.entries[i-1]
			style := dirNormalStyle
			if isSelected {
				style = dirSelectedStyle
			}
			s.WriteString(cursor)
			s.WriteString(style.Render(entry.Name + "/"))
		}
		s.WriteString("\n")
	}

	// Scroll indicators.
	if b.offset > 0 {
		s.WriteString(dirMutedStyle.Render("  ↑ more"))
		s.WriteString("\n")
	}
	if b.height > 0 && b.offset+visibleHeight < totalItems {
		s.WriteString(dirMutedStyle.Render("  ↓ more"))
		s.WriteString("\n")
	}

	return s.String()
}

// ensureCursorVisible adjusts the scroll offset so the cursor is visible.
func (b *DirectoryBrowser) ensureCursorVisible() {
	if b.height <= 0 {
		return
	}
	if b.cursor < b.offset {
		b.offset = b.cursor
	}
	if b.cursor >= b.offset+b.height {
		b.offset = b.cursor - b.height + 1
	}
}

func (b *DirectoryBrowser) handleKey(msg tea.KeyPressMsg) (*DirectoryBrowser, tea.Cmd) {
	maxIndex := len(b.entries) // parent dir is index 0

	switch msg.String() {
	case "up", "k":
		if b.cursor > 0 {
			b.cursor--
			b.ensureCursorVisible()
		}
	case "down", "j":
		if b.cursor < maxIndex {
			b.cursor++
			b.ensureCursorVisible()
		}
	case "enter":
		if b.cursor == 0 {
			parent := filepath.Dir(b.currentPath)
			if parent != b.currentPath {
				b.loading = true
				return b, b.loadFn(parent)
			}
		} else {
			idx := b.cursor - 1
			if idx < len(b.entries) {
				b.loading = true
				return b, b.loadFn(b.entries[idx].Path)
			}
		}
	case "backspace":
		parent := filepath.Dir(b.currentPath)
		if parent != b.currentPath {
			b.loading = true
			return b, b.loadFn(parent)
		}
	}
	return b, nil
}
