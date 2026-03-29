package tui

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/thesimonho/warden/access"
	"github.com/thesimonho/warden/api"
)

// AccessView displays access items with detection status and provides
// management actions (create, edit, delete, reset, test/resolve).
type AccessView struct {
	client  Client
	items   []api.AccessItemResponse
	loading bool
	err     error
	cursor  int
	keys    AccessKeyMap

	// Sub-view states.
	formView *AccessFormView

	// Delete confirmation.
	confirmingDelete bool

	// Resolve/test output shown inline below the list.
	resolveResult []access.ResolvedCredential
	resolveLabel  string
	resolving     bool
}

// NewAccessView creates a new access management view.
func NewAccessView(client Client) *AccessView {
	return &AccessView{
		client:  client,
		loading: true,
		keys:    DefaultAccessKeyMap(),
	}
}

// Init fetches the access items list.
func (v *AccessView) Init() tea.Cmd {
	v.loading = true
	return v.fetchItems()
}

// fetchItems issues the ListAccessItems API call.
func (v *AccessView) fetchItems() tea.Cmd {
	return func() tea.Msg {
		resp, err := v.client.ListAccessItems(context.Background())
		if err != nil {
			return AccessItemsLoadedMsg{Err: err}
		}
		return AccessItemsLoadedMsg{Items: resp.Items}
	}
}

// Update handles messages for the access view.
func (v *AccessView) Update(msg tea.Msg) (View, tea.Cmd) {
	// Delegate to form sub-view when active.
	if v.formView != nil {
		return v.updateFormView(msg)
	}

	switch msg := msg.(type) {
	case AccessItemsLoadedMsg:
		v.loading = false
		if msg.Err != nil {
			v.err = msg.Err
			return v, nil
		}
		v.items = msg.Items
		if v.cursor >= len(v.items) && len(v.items) > 0 {
			v.cursor = len(v.items) - 1
		}
		return v, nil

	case AccessItemResolvedMsg:
		v.resolving = false
		if msg.Err != nil {
			v.err = msg.Err
			return v, nil
		}
		if len(msg.Items) > 0 {
			v.resolveResult = msg.Items[0].Credentials
			v.resolveLabel = msg.Items[0].Label
		}
		return v, nil

	case OperationResultMsg:
		if msg.Err != nil {
			v.err = msg.Err
			return v, nil
		}
		// Refresh list after any mutation.
		return v, v.fetchItems()

	case tea.KeyPressMsg:
		if v.confirmingDelete {
			return v.handleDeleteConfirm(msg)
		}
		if v.err != nil {
			v.err = nil
			return v, v.fetchItems()
		}
		return v.handleKey(msg)
	}
	return v, nil
}

// updateFormView delegates messages to the form sub-view and returns
// to the list when the form navigates back.
func (v *AccessView) updateFormView(msg tea.Msg) (View, tea.Cmd) {
	if _, isBack := msg.(NavigateBackMsg); isBack {
		v.formView = nil
		return v, v.fetchItems()
	}
	var cmd tea.Cmd
	updated, cmd := v.formView.Update(msg)
	if form, ok := updated.(*AccessFormView); ok {
		v.formView = form
	}
	return v, cmd
}

// handleKey processes key presses in the list view.
func (v *AccessView) handleKey(msg tea.KeyPressMsg) (View, tea.Cmd) {
	switch {
	case msg.String() == "up" || msg.String() == "k":
		if v.cursor > 0 {
			v.cursor--
		}
		v.clearResolve()
	case msg.String() == "down" || msg.String() == "j":
		if v.cursor < len(v.items)-1 {
			v.cursor++
		}
		v.clearResolve()
	case key.Matches(msg, v.keys.New):
		v.formView = NewAccessFormView(v.client, nil)
		return v, v.formView.Init()
	case key.Matches(msg, v.keys.Edit):
		return v.editSelected()
	case key.Matches(msg, v.keys.Delete):
		return v.deleteSelected()
	case key.Matches(msg, v.keys.Reset):
		return v.resetSelected()
	case key.Matches(msg, v.keys.Test):
		return v.testSelected()
	case key.Matches(msg, v.keys.Refresh):
		return v, v.fetchItems()
	}
	return v, nil
}

// editSelected opens the form view for the selected item.
func (v *AccessView) editSelected() (View, tea.Cmd) {
	if len(v.items) == 0 {
		return v, nil
	}
	item := v.items[v.cursor]
	v.formView = NewAccessFormView(v.client, &item)
	return v, v.formView.Init()
}

// deleteSelected initiates delete confirmation for user-created items.
func (v *AccessView) deleteSelected() (View, tea.Cmd) {
	if len(v.items) == 0 {
		return v, nil
	}
	item := v.items[v.cursor]
	if item.BuiltIn {
		v.err = fmt.Errorf("built-in items cannot be deleted (use 'r' to reset)")
		return v, nil
	}
	v.confirmingDelete = true
	return v, nil
}

// handleDeleteConfirm processes keys during delete confirmation.
func (v *AccessView) handleDeleteConfirm(msg tea.KeyPressMsg) (View, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		v.confirmingDelete = false
		item := v.items[v.cursor]
		return v, func() tea.Msg {
			err := v.client.DeleteAccessItem(context.Background(), item.ID)
			return OperationResultMsg{Operation: "delete_access", Err: err}
		}
	case "n", "N", "esc":
		v.confirmingDelete = false
	}
	return v, nil
}

// resetSelected resets a built-in item to its default configuration.
func (v *AccessView) resetSelected() (View, tea.Cmd) {
	if len(v.items) == 0 {
		return v, nil
	}
	item := v.items[v.cursor]
	if !item.BuiltIn {
		v.err = fmt.Errorf("only built-in items can be reset")
		return v, nil
	}
	return v, func() tea.Msg {
		_, err := v.client.ResetAccessItem(context.Background(), item.ID)
		return OperationResultMsg{Operation: "reset_access", Err: err}
	}
}

// testSelected resolves the selected item to preview injections.
func (v *AccessView) testSelected() (View, tea.Cmd) {
	if len(v.items) == 0 {
		return v, nil
	}
	item := v.items[v.cursor]
	v.resolving = true
	v.resolveResult = nil
	return v, func() tea.Msg {
		resp, err := v.client.ResolveAccessItems(context.Background(), api.ResolveAccessItemsRequest{
			Items: []access.Item{item.Item},
		})
		if err != nil {
			return AccessItemResolvedMsg{Err: err}
		}
		return AccessItemResolvedMsg{Items: resp.Items}
	}
}

// clearResolve dismisses the resolve output when navigating.
func (v *AccessView) clearResolve() {
	v.resolveResult = nil
	v.resolveLabel = ""
}

// Render renders the access view.
func (v *AccessView) Render(width, height int) string {
	// Form sub-view takes over rendering when active.
	if v.formView != nil {
		return v.formView.Render(width, height)
	}

	var s strings.Builder

	// Header.
	s.WriteString(v.renderHeader())

	if v.loading {
		return s.String() + "Loading access items..."
	}
	if v.err != nil {
		return s.String() + Styles.Error.Render("Error: "+v.err.Error()) + "\n" +
			Styles.Muted.Render("Press any key to dismiss")
	}

	// Delete confirmation overlay.
	if v.confirmingDelete && v.cursor < len(v.items) {
		s.WriteString("\n")
		s.WriteString(Styles.Error.Render(fmt.Sprintf(
			"Delete \"%s\"? This cannot be undone.", v.items[v.cursor].Label,
		)))
		s.WriteString("\n")
		s.WriteString(Styles.Muted.Render("y to confirm · n/esc to cancel"))
		return s.String()
	}

	if len(v.items) == 0 {
		s.WriteString(Styles.Muted.Render("No access items. Press 'n' to create one."))
		return s.String()
	}

	// Item list.
	headerLines := 3
	resolveLines := v.resolveLineCount()
	maxVisible := height - headerLines - resolveLines
	if maxVisible < 3 {
		maxVisible = 3
	}

	s.WriteString(v.renderItems(width, maxVisible))

	// Resolve output.
	if v.resolving {
		s.WriteString("\n" + Styles.Muted.Render("Resolving..."))
	} else if v.resolveResult != nil {
		s.WriteString(v.renderResolveResult())
	}

	return s.String()
}

// renderHeader renders the title with detection summary.
func (v *AccessView) renderHeader() string {
	var s strings.Builder
	s.WriteString(Styles.Subtitle.Render("Access"))
	if len(v.items) > 0 {
		available := 0
		for _, item := range v.items {
			if item.Detection.Available {
				available++
			}
		}
		s.WriteString(Styles.Muted.Render(fmt.Sprintf(
			" (%d of %d detected)", available, len(v.items),
		)))
	}
	s.WriteString("\n\n")
	return s.String()
}

// renderItems renders the scrollable list of access items.
func (v *AccessView) renderItems(width, maxVisible int) string {
	var s strings.Builder

	start := 0
	if v.cursor >= maxVisible {
		start = v.cursor - maxVisible + 1
	}
	end := start + maxVisible
	if end > len(v.items) {
		end = len(v.items)
	}

	for i := start; i < end; i++ {
		s.WriteString(v.renderItem(v.items[i], i == v.cursor, width))
	}

	return s.String()
}

// renderItem renders a single access item line.
func (v *AccessView) renderItem(item api.AccessItemResponse, isSelected bool, _ int) string {
	cursor := "  "
	if isSelected {
		cursor = cursorMarker
	}

	dot := statusDot(item.Detection.Available)

	// Label and badges.
	label := item.Label
	if isSelected {
		label = Styles.Bold.Render(label)
	}

	badges := ""
	if item.BuiltIn {
		badges += " " + Styles.Muted.Render("[built-in]")
	}

	// Credential summary.
	credParts := v.renderCredentialSummary(item)

	line := cursor + dot + " " + label + badges
	if credParts != "" {
		line += "  " + credParts
	}
	return line + "\n"
}

// renderCredentialSummary renders a compact per-credential detection summary.
func (v *AccessView) renderCredentialSummary(item api.AccessItemResponse) string {
	if len(item.Detection.Credentials) == 0 {
		return ""
	}
	var parts []string
	for _, cred := range item.Detection.Credentials {
		parts = append(parts, statusDot(cred.Available)+" "+Styles.Muted.Render(cred.Label))
	}
	return strings.Join(parts, "  ")
}

// resolveLineCount estimates how many lines the resolve output will use.
func (v *AccessView) resolveLineCount() int {
	if v.resolving {
		return 2
	}
	if v.resolveResult == nil {
		return 0
	}
	// Title + one line per credential + blank separator.
	count := 3
	for _, cred := range v.resolveResult {
		count++ // credential label line
		count += len(cred.Injections)
	}
	return count
}

// renderResolveResult renders the test/resolve output inline.
func (v *AccessView) renderResolveResult() string {
	var s strings.Builder
	s.WriteString("\n" + Styles.Bold.Render("Test: "+v.resolveLabel) + "\n")

	for _, cred := range v.resolveResult {
		dot := statusDot(cred.Resolved)
		label := cred.Label
		if cred.SourceMatched != "" {
			label += " " + Styles.Muted.Render("("+cred.SourceMatched+")")
		}
		s.WriteString("  " + dot + " " + label + "\n")

		if cred.Error != "" {
			s.WriteString("    " + Styles.Error.Render(cred.Error) + "\n")
		}
		for _, inj := range cred.Injections {
			ro := ""
			if inj.ReadOnly {
				ro = Styles.Muted.Render(" [RO]")
			}
			fmt.Fprintf(&s,
				"    %s %s = %s%s\n",
				Styles.Muted.Render(string(inj.Type)),
				Styles.Bold.Render(inj.Key),
				truncate(inj.Value, 50),
				ro,
			)
		}
		if !cred.Resolved && cred.Error == "" {
			s.WriteString("    " + Styles.Muted.Render("Not detected on host") + "\n")
		}
	}
	return s.String()
}

// statusDot returns a styled filled or empty circle based on available.
func statusDot(available bool) string {
	if available {
		return Styles.Success.Render("●")
	}
	return Styles.Muted.Render("○")
}

// HelpKeyMap returns the access view's key bindings for the help bar.
func (v *AccessView) HelpKeyMap() help.KeyMap {
	if v.formView != nil {
		return v.formView.HelpKeyMap()
	}
	return v.keys
}
