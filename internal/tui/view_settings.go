package tui

import (
	"context"
	"fmt"
	"strconv"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/thesimonho/warden/api"
)

// SettingsView displays and allows editing of server settings.
type SettingsView struct {
	client        Client
	settings      *api.SettingsResponse
	loading       bool
	err           error
	cursor        int
	keys          SettingsKeyMap
	capturingKey  bool // true when waiting for a ctrl+key press for disconnect key
	editingBudget bool // true when the budget text input is focused
	budgetInput   textinput.Model
}

// settings menu items.
const (
	settingsItemNotificationsEnabled = iota
	settingsItemAuditLogMode
	settingsItemDisconnectKey
	settingsItemDefaultBudget
	settingsItemBudgetActionWarn
	settingsItemBudgetActionStopWorktrees
	settingsItemBudgetActionStopContainer
	settingsItemBudgetActionPreventStart
	settingsItemCount
)

// NewSettingsView creates a new settings view.
func NewSettingsView(client Client) *SettingsView {
	ti := textinput.New()
	ti.Placeholder = "0 = unlimited"
	ti.CharLimit = 10
	ti.SetWidth(15)
	return &SettingsView{
		client:      client,
		loading:     true,
		keys:        DefaultSettingsKeyMap(),
		budgetInput: ti,
	}
}

// Init fetches settings.
func (v *SettingsView) Init() tea.Cmd {
	v.loading = true
	return loadSettings(v.client)
}

// Update handles messages for the settings view.
func (v *SettingsView) Update(msg tea.Msg) (View, tea.Cmd) {
	switch msg := msg.(type) {
	case SettingsLoadedMsg:
		if msg.Err != nil {
			v.err = msg.Err
			v.loading = false
			return v, nil
		}
		v.settings = msg.Settings
		v.loading = false
		return v, nil

	case OperationResultMsg:
		if msg.Err != nil {
			v.err = msg.Err
		}
		// Refresh settings after mutation.
		return v, loadSettings(v.client)

	case tea.KeyPressMsg:
		if v.err != nil {
			v.err = nil
			return v, loadSettings(v.client)
		}
		return v.handleKey(msg)

	default:
		// Forward non-key messages to text input when editing budget.
		if v.editingBudget {
			var cmd tea.Cmd
			v.budgetInput, cmd = v.budgetInput.Update(msg)
			return v, cmd
		}
	}
	return v, nil
}

func (v *SettingsView) handleKey(msg tea.KeyPressMsg) (View, tea.Cmd) {
	// When capturing a disconnect key, accept ctrl+<key> or esc to cancel.
	if v.capturingKey {
		return v.handleKeyCapture(msg)
	}

	// When editing the budget text input, forward keys to it.
	if v.editingBudget {
		return v.handleBudgetEdit(msg)
	}

	switch msg.String() {
	case "up", "k":
		if v.cursor > 0 {
			v.cursor--
		}
	case "down", "j":
		if v.cursor < settingsItemCount-1 {
			v.cursor++
		}
	case "enter", " ":
		return v.activateItem()
	}
	return v, nil
}

// handleKeyCapture waits for a ctrl+<key> press to set as the disconnect key.
func (v *SettingsView) handleKeyCapture(msg tea.KeyPressMsg) (View, tea.Cmd) {
	keyStr := msg.String()

	// Cancel on escape.
	if keyStr == "esc" {
		v.capturingKey = false
		return v, nil
	}

	// Accept ctrl+<letter> combinations.
	if len(keyStr) > 5 && keyStr[:5] == "ctrl+" {
		newKey := keyStr
		v.capturingKey = false
		return v, func() tea.Msg {
			_, err := v.client.UpdateSettings(context.Background(), api.UpdateSettingsRequest{
				DisconnectKey: &newKey,
			})
			return OperationResultMsg{Operation: "change_disconnectkey", Err: err}
		}
	}

	// Ignore non-ctrl keys while capturing.
	return v, nil
}

func (v *SettingsView) activateItem() (View, tea.Cmd) {
	if v.settings == nil {
		return v, nil
	}

	switch v.cursor {
	case settingsItemNotificationsEnabled:
		newVal := !v.settings.NotificationsEnabled
		return v, func() tea.Msg {
			_, err := v.client.UpdateSettings(context.Background(), api.UpdateSettingsRequest{
				NotificationsEnabled: &newVal,
			})
			return OperationResultMsg{Operation: "toggle_notifications", Err: err}
		}

	case settingsItemAuditLogMode:
		// Cycle audit log mode: off → standard → detailed → off.
		modes := []api.AuditLogMode{api.AuditLogOff, api.AuditLogStandard, api.AuditLogDetailed}
		current := v.settings.AuditLogMode
		nextIdx := 0
		for i, m := range modes {
			if m == current {
				nextIdx = (i + 1) % len(modes)
				break
			}
		}
		newMode := modes[nextIdx]
		return v, func() tea.Msg {
			_, err := v.client.UpdateSettings(context.Background(), api.UpdateSettingsRequest{
				AuditLogMode: &newMode,
			})
			return OperationResultMsg{Operation: "change_auditlogmode", Err: err}
		}

	case settingsItemDisconnectKey:
		// Enter capture mode — next ctrl+key press sets the disconnect key.
		v.capturingKey = true
		return v, nil

	case settingsItemDefaultBudget:
		// Enter budget edit mode.
		v.editingBudget = true
		if v.settings.DefaultProjectBudget > 0 {
			v.budgetInput.SetValue(strconv.FormatFloat(v.settings.DefaultProjectBudget, 'f', 2, 64))
		} else {
			v.budgetInput.SetValue("")
		}
		return v, v.budgetInput.Focus()

	case settingsItemBudgetActionWarn:
		newVal := !v.settings.BudgetActionWarn
		return v, func() tea.Msg {
			_, err := v.client.UpdateSettings(context.Background(), api.UpdateSettingsRequest{
				BudgetActionWarn: &newVal,
			})
			return OperationResultMsg{Operation: "toggle_budget_warn", Err: err}
		}

	case settingsItemBudgetActionStopWorktrees:
		newVal := !v.settings.BudgetActionStopWorktrees
		return v, func() tea.Msg {
			_, err := v.client.UpdateSettings(context.Background(), api.UpdateSettingsRequest{
				BudgetActionStopWorktrees: &newVal,
			})
			return OperationResultMsg{Operation: "toggle_budget_stop_worktrees", Err: err}
		}

	case settingsItemBudgetActionStopContainer:
		newVal := !v.settings.BudgetActionStopContainer
		return v, func() tea.Msg {
			_, err := v.client.UpdateSettings(context.Background(), api.UpdateSettingsRequest{
				BudgetActionStopContainer: &newVal,
			})
			return OperationResultMsg{Operation: "toggle_budget_stop_container", Err: err}
		}

	case settingsItemBudgetActionPreventStart:
		newVal := !v.settings.BudgetActionPreventStart
		return v, func() tea.Msg {
			_, err := v.client.UpdateSettings(context.Background(), api.UpdateSettingsRequest{
				BudgetActionPreventStart: &newVal,
			})
			return OperationResultMsg{Operation: "toggle_budget_prevent_start", Err: err}
		}
	}
	return v, nil
}

// handleBudgetEdit processes key events while the budget text input is focused.
func (v *SettingsView) handleBudgetEdit(msg tea.KeyPressMsg) (View, tea.Cmd) {
	switch msg.String() {
	case "esc":
		v.editingBudget = false
		v.budgetInput.Blur()
		return v, nil
	case "enter":
		v.editingBudget = false
		v.budgetInput.Blur()
		value, _ := strconv.ParseFloat(v.budgetInput.Value(), 64)
		if value < 0 {
			value = 0
		}
		return v, func() tea.Msg {
			_, err := v.client.UpdateSettings(context.Background(), api.UpdateSettingsRequest{
				DefaultProjectBudget: &value,
			})
			return OperationResultMsg{Operation: "change_budget", Err: err}
		}
	}

	// Forward to text input.
	var cmd tea.Cmd
	v.budgetInput, cmd = v.budgetInput.Update(msg)
	return v, cmd
}

// Render renders the settings view.
func (v *SettingsView) Render(_, _ int) string {
	if v.loading {
		return "Loading settings..."
	}
	if v.err != nil {
		return Styles.Error.Render("Error: " + v.err.Error())
	}

	var s string
	s += Styles.Subtitle.Render("Settings") + "\n\n"

	if v.settings != nil {
		// Desktop notifications toggle.
		cursor := "  "
		if v.cursor == settingsItemNotificationsEnabled {
			cursor = "> "
		}
		notifCheckbox := "[ ]"
		if v.settings.NotificationsEnabled {
			notifCheckbox = "[x]"
		}
		line := cursor + notifCheckbox + " " + Styles.Bold.Render("Desktop Notifications")
		if v.cursor == settingsItemNotificationsEnabled {
			s += Styles.Bold.Render(line)
		} else {
			s += line
		}
		s += "\n"
		s += "    " + Styles.Muted.Render("System tray notifications when agents need input") + "\n\n"

		// Audit log mode.
		cursor = "  "
		if v.cursor == settingsItemAuditLogMode {
			cursor = "> "
		}
		modeDisplay := Styles.Error.Render("off")
		switch v.settings.AuditLogMode {
		case api.AuditLogStandard:
			modeDisplay = Styles.Success.Render("standard")
		case api.AuditLogDetailed:
			modeDisplay = Styles.Success.Render("detailed")
		}
		line = cursor + Styles.Bold.Render("Audit Log: ") + modeDisplay
		if v.cursor == settingsItemAuditLogMode {
			s += Styles.Bold.Render(line)
		} else {
			s += line
		}
		s += "\n"
		s += "    " + Styles.Muted.Render("off = disabled, standard = sessions/lifecycle, detailed = all events") + "\n"

		// Disconnect key selector.
		cursor = "  "
		if v.cursor == settingsItemDisconnectKey {
			cursor = "> "
		}
		disconnectValue := v.settings.DisconnectKey
		if v.capturingKey {
			disconnectValue = Styles.Warning.Render("press ctrl+<key>...")
		}
		line = cursor + Styles.Bold.Render("Disconnect Key: ") + disconnectValue
		if v.cursor == settingsItemDisconnectKey {
			s += Styles.Bold.Render(line)
		} else {
			s += line
		}
		s += "\n"
		s += "    " + Styles.Muted.Render("Key to return from terminal to Warden") + "\n"

		// Default budget.
		cursor = "  "
		if v.cursor == settingsItemDefaultBudget {
			cursor = "> "
		}
		if v.editingBudget {
			line = cursor + Styles.Bold.Render("Default Budget: $") + v.budgetInput.View()
		} else {
			budgetStr := Styles.Muted.Render("unlimited")
			if v.settings.DefaultProjectBudget > 0 {
				budgetStr = fmt.Sprintf("$%.2f", v.settings.DefaultProjectBudget)
			}
			line = cursor + Styles.Bold.Render("Default Budget: ") + budgetStr
		}
		if v.cursor == settingsItemDefaultBudget {
			s += Styles.Bold.Render(line)
		} else {
			s += line
		}
		s += "\n"
		s += "    " + Styles.Muted.Render("Per-project cost limit in USD (0 or empty = unlimited)") + "\n"

		// Budget enforcement actions.
		s += "\n" + Styles.Muted.Render("  Budget enforcement") + "\n"
		for _, item := range []struct {
			index int
			label string
			value bool
		}{
			{settingsItemBudgetActionWarn, "Show a warning", v.settings.BudgetActionWarn},
			{settingsItemBudgetActionStopWorktrees, "Stop worktrees", v.settings.BudgetActionStopWorktrees},
			{settingsItemBudgetActionStopContainer, "Stop container", v.settings.BudgetActionStopContainer},
			{settingsItemBudgetActionPreventStart, "Prevent restart", v.settings.BudgetActionPreventStart},
		} {
			cursor = "  "
			if v.cursor == item.index {
				cursor = "> "
			}
			checkbox := "[ ]"
			if item.value {
				checkbox = "[x]"
			}
			line = cursor + checkbox + " " + item.label
			if v.cursor == item.index {
				s += Styles.Bold.Render(line)
			} else {
				s += line
			}
			s += "\n"
		}
	}

	return s
}

// HelpKeyMap returns the settings view's key bindings for the help bar.
func (v *SettingsView) HelpKeyMap() help.KeyMap {
	return v.keys
}

// --- Commands ---

func loadSettings(client Client) tea.Cmd {
	return func() tea.Msg {
		settings, err := client.GetSettings(context.Background())
		return SettingsLoadedMsg{Settings: settings, Err: err}
	}
}
