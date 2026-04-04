package tui

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/thesimonho/warden/api"
)

// Filter values for cycling through options. Empty string means "all".
// Category order: session → agent → prompt → config → budget → system → debug.
var (
	categoryFilters = []string{
		"",
		string(api.AuditCategorySession), string(api.AuditCategoryAgent),
		string(api.AuditCategoryPrompt), string(api.AuditCategoryConfig),
		string(api.AuditCategoryBudget), string(api.AuditCategorySystem),
		string(api.AuditCategoryDebug),
	}
	levelFilters = []string{
		"", string(api.AuditLevelInfo), string(api.AuditLevelWarn), string(api.AuditLevelError),
	}
	sourceFilters = []string{
		"", string(api.AuditSourceAgent), string(api.AuditSourceBackend),
		string(api.AuditSourceFrontend), string(api.AuditSourceContainer),
	}
)

// timeRangePresets defines the available time range filter options.
// Empty duration means "all time".
var timeRangePresets = []struct {
	Label    string
	Duration time.Duration
}{
	{Label: "", Duration: 0},
	{Label: "1h", Duration: 1 * time.Hour},
	{Label: "6h", Duration: 6 * time.Hour},
	{Label: "24h", Duration: 24 * time.Hour},
	{Label: "7d", Duration: 7 * 24 * time.Hour},
	{Label: "30d", Duration: 30 * 24 * time.Hour},
}

// autoRefreshIntervals defines the available auto-refresh intervals.
// Zero duration means "off".
var autoRefreshIntervals = []struct {
	Label    string
	Duration time.Duration
}{
	{Label: "", Duration: 0},
	{Label: "10s", Duration: 10 * time.Second},
	{Label: "30s", Duration: 30 * time.Second},
	{Label: "1m", Duration: 1 * time.Minute},
	{Label: "5m", Duration: 5 * time.Minute},
}

// auditEventLabel converts a snake_case event name to a human-readable label.
func auditEventLabel(event string) string {
	return strings.Title(strings.ReplaceAll(event, "_", " ")) //nolint:staticcheck // strings.Title is fine for ASCII event names
}

// deleteConfirmWord is the text the user must type to confirm audit deletion.
const deleteConfirmWord = "delete"

// AuditLogView displays filtered audit log entries with summary and filters.
type AuditLogView struct {
	client  Client
	entries []api.AuditEntry
	summary *api.AuditSummary
	loading bool
	err     error
	offset  int
	keys    AuditLogKeyMap

	// Filter indices — the string value is derived from the corresponding
	// slice on read (categoryFilters, levelFilters, sourceFilters).
	categoryIdx int
	levelIdx    int
	sourceIdx   int

	// Project filter cycles through names fetched from the API.
	// Index 0 = "all", 1..N = projectNames[idx-1].
	projectNames []string
	projectIdx   int

	timeRangeIdx   int
	autoRefreshIdx int

	// Incremented on interval change so stale ticks are ignored.
	autoRefreshGen int

	// Set before issuing a fetch to preserve scroll on auto-refresh.
	preserveScroll bool

	// Delete confirmation state.
	confirmingDelete bool
	confirmInput     textinput.Model
}

// NewAuditLogView creates a new audit log view.
func NewAuditLogView(client Client) *AuditLogView {
	return &AuditLogView{
		client:  client,
		loading: true,
		keys:    DefaultAuditLogKeyMap(),
	}
}

// Init fetches audit log entries, summary, and available project names.
func (v *AuditLogView) Init() tea.Cmd {
	v.loading = true
	return tea.Batch(v.fetchData(), v.fetchProjects())
}

func (v *AuditLogView) buildFilters() api.AuditFilters {
	filters := api.AuditFilters{
		Category:  api.AuditCategory(categoryFilters[v.categoryIdx]),
		Level:     levelFilters[v.levelIdx],
		Source:    sourceFilters[v.sourceIdx],
		ProjectID: v.activeProjectFilter(),
	}
	if preset := timeRangePresets[v.timeRangeIdx]; preset.Duration > 0 {
		filters.Since = time.Now().Add(-preset.Duration).Format(time.RFC3339)
	}
	return filters
}

// activeProjectFilter returns the current project filter value.
func (v *AuditLogView) activeProjectFilter() string {
	if v.projectIdx == 0 || len(v.projectNames) == 0 {
		return ""
	}
	return v.projectNames[v.projectIdx-1]
}

func (v *AuditLogView) fetchData() tea.Cmd {
	filters := v.buildFilters()
	return func() tea.Msg {
		entries, summary := v.fetchEntriesAndSummary(filters)
		if entries.err != nil {
			return AuditLogLoadedMsg{Err: entries.err}
		}
		if summary.err != nil {
			return AuditLogLoadedMsg{Entries: entries.data, Err: summary.err}
		}
		return AuditLogLoadedMsg{Entries: entries.data, Summary: summary.data}
	}
}

// fetchEntriesAndSummary issues both API calls concurrently.
func (v *AuditLogView) fetchEntriesAndSummary(filters api.AuditFilters) (
	entryResult struct {
		data []api.AuditEntry
		err  error
	},
	summaryResult struct {
		data *api.AuditSummary
		err  error
	},
) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		entryResult.data, entryResult.err = v.client.GetAuditLog(context.Background(), filters)
	}()
	go func() {
		defer wg.Done()
		summaryResult.data, summaryResult.err = v.client.GetAuditSummary(context.Background(), filters)
	}()
	wg.Wait()
	return entryResult, summaryResult
}

func (v *AuditLogView) fetchProjects() tea.Cmd {
	return func() tea.Msg {
		names, err := v.client.GetAuditProjects(context.Background())
		if err != nil {
			return AuditProjectsLoadedMsg{Err: err}
		}
		return AuditProjectsLoadedMsg{Names: names}
	}
}

// autoRefreshTickMsg is sent by the auto-refresh ticker.
// The generation field allows stale ticks from a previous interval to be
// discarded when the user switches intervals before the old tick fires.
type autoRefreshTickMsg struct {
	generation int
}

func (v *AuditLogView) scheduleAutoRefresh() tea.Cmd {
	interval := autoRefreshIntervals[v.autoRefreshIdx]
	if interval.Duration == 0 {
		return nil
	}
	gen := v.autoRefreshGen
	return tea.Tick(interval.Duration, func(time.Time) tea.Msg {
		return autoRefreshTickMsg{generation: gen}
	})
}

// Update handles messages for the audit log view.
func (v *AuditLogView) Update(msg tea.Msg) (View, tea.Cmd) {
	switch msg := msg.(type) {
	case AuditLogLoadedMsg:
		v.loading = false
		if msg.Err != nil {
			v.err = msg.Err
			return v, nil
		}
		v.entries = msg.Entries
		v.summary = msg.Summary
		if !v.preserveScroll {
			v.offset = 0
		}
		v.preserveScroll = false
		// Clamp offset if entries shrunk.
		if v.offset >= len(v.entries) && len(v.entries) > 0 {
			v.offset = len(v.entries) - 1
		}
		return v, nil

	case AuditProjectsLoadedMsg:
		if msg.Err == nil {
			v.projectNames = msg.Names
		}
		return v, nil

	case OperationResultMsg:
		if msg.Err != nil {
			v.err = msg.Err
		}
		return v, tea.Batch(v.fetchData(), v.fetchProjects())

	case autoRefreshTickMsg:
		// Discard stale ticks from a previous interval.
		if msg.generation != v.autoRefreshGen {
			return v, nil
		}
		if autoRefreshIntervals[v.autoRefreshIdx].Duration == 0 {
			return v, nil
		}
		v.preserveScroll = true
		return v, tea.Batch(v.fetchData(), v.scheduleAutoRefresh())

	case tea.KeyPressMsg:
		if v.confirmingDelete {
			return v.updateDeleteConfirm(msg)
		}
		if v.err != nil {
			v.err = nil
			return v, v.fetchData()
		}
		return v.handleKey(msg)
	}
	return v, nil
}

func (v *AuditLogView) handleKey(msg tea.KeyPressMsg) (View, tea.Cmd) {
	switch {
	case msg.String() == "up" || msg.String() == "k":
		if v.offset > 0 {
			v.offset--
		}
	case msg.String() == "down" || msg.String() == "j":
		if v.offset < len(v.entries)-1 {
			v.offset++
		}
	case key.Matches(msg, v.keys.Refresh):
		return v, v.fetchData()
	case key.Matches(msg, v.keys.CategoryFilter):
		v.categoryIdx = (v.categoryIdx + 1) % len(categoryFilters)
		return v, v.fetchData()
	case key.Matches(msg, v.keys.LevelFilter):
		v.levelIdx = (v.levelIdx + 1) % len(levelFilters)
		return v, v.fetchData()
	case key.Matches(msg, v.keys.SourceFilter):
		v.sourceIdx = (v.sourceIdx + 1) % len(sourceFilters)
		return v, v.fetchData()
	case key.Matches(msg, v.keys.ProjectFilter):
		v.cycleProjectFilter()
		return v, v.fetchData()
	case key.Matches(msg, v.keys.TimeRange):
		v.timeRangeIdx = (v.timeRangeIdx + 1) % len(timeRangePresets)
		return v, v.fetchData()
	case key.Matches(msg, v.keys.AutoRefresh):
		v.autoRefreshIdx = (v.autoRefreshIdx + 1) % len(autoRefreshIntervals)
		v.autoRefreshGen++
		return v, v.scheduleAutoRefresh()
	case key.Matches(msg, v.keys.Clear):
		ti := textinput.New()
		ti.Placeholder = deleteConfirmWord
		ti.Prompt = "> "
		ti.CharLimit = 10
		v.confirmInput = ti
		v.confirmingDelete = true
		return v, ti.Focus()
	}
	return v, nil
}

// updateDeleteConfirm handles key presses during the delete confirmation input.
func (v *AuditLogView) updateDeleteConfirm(msg tea.KeyPressMsg) (View, tea.Cmd) {
	switch msg.String() {
	case "esc":
		v.confirmingDelete = false
		return v, nil
	case "enter":
		if v.confirmInput.Value() == deleteConfirmWord {
			v.confirmingDelete = false
			filters := v.buildFilters()
			return v, func() tea.Msg {
				err := v.client.DeleteAuditEvents(context.Background(), filters)
				return OperationResultMsg{Operation: "clear", Err: err}
			}
		}
		return v, nil
	}

	var cmd tea.Cmd
	v.confirmInput, cmd = v.confirmInput.Update(msg)
	return v, cmd
}

// cycleProjectFilter advances through: all → project1 → project2 → ... → all.
func (v *AuditLogView) cycleProjectFilter() {
	if len(v.projectNames) == 0 {
		v.projectIdx = 0
		return
	}
	v.projectIdx = (v.projectIdx + 1) % (len(v.projectNames) + 1)
}

// formatFilterLabel renders a filter name:value pair with appropriate styling.
func formatFilterLabel(name, value string) string {
	if value != "" {
		return Styles.Bold.Render(name+":") + value
	}
	return Styles.Muted.Render(name + ":all")
}

// Render renders the audit log view.
func (v *AuditLogView) Render(width, height int) string {
	// Show delete confirmation overlay.
	if v.confirmingDelete {
		var s strings.Builder
		s.WriteString(Styles.Bold.Render("Delete Audit Events") + "\n\n")
		s.WriteString(Styles.Error.Render("Type '"+deleteConfirmWord+"' to confirm deletion:") + "\n")
		s.WriteString(v.confirmInput.View() + "\n\n")
		s.WriteString(Styles.Muted.Render("enter to confirm · esc to cancel"))
		return s.String()
	}

	var s strings.Builder

	s.WriteString(v.renderHeader())
	s.WriteString(v.renderFilters())

	if v.loading {
		return s.String() + "Loading audit log..."
	}
	if v.err != nil {
		return s.String() + Styles.Error.Render("Error: "+v.err.Error())
	}
	if len(v.entries) == 0 {
		return s.String() + Styles.Muted.Render("No audit entries. Enable audit logging in settings.")
	}

	// Title + filters + blank = 4 lines, +1 for summary when present.
	headerLines := 5
	if v.summary != nil {
		headerLines = 6
	}
	maxVisible := height - headerLines
	if maxVisible < 5 {
		maxVisible = 5
	}

	s.WriteString(v.renderEntries(width, maxVisible))
	return s.String()
}

// renderHeader renders the title and summary lines.
func (v *AuditLogView) renderHeader() string {
	var s strings.Builder
	s.WriteString(Styles.Subtitle.Render("Audit Log"))
	if len(v.entries) > 0 {
		s.WriteString(Styles.Muted.Render(fmt.Sprintf(" (%d entries)", len(v.entries))))
	}
	s.WriteString("\n")

	if v.summary != nil {
		fmt.Fprintf(&s,
			"Sessions: %d  Tools: %d  Prompts: %d  Cost: $%.2f  Projects: %d  Worktrees: %d",
			v.summary.TotalSessions,
			v.summary.TotalToolUses,
			v.summary.TotalPrompts,
			v.summary.TotalCostUSD,
			v.summary.UniqueProjects,
			v.summary.UniqueWorktrees,
		)
		s.WriteString("\n")
	}
	return s.String()
}

// renderFilters renders the active filter bar and auto-refresh indicator.
func (v *AuditLogView) renderFilters() string {
	var s strings.Builder
	s.WriteString("Filters: ")
	s.WriteString(formatFilterLabel("category", categoryFilters[v.categoryIdx]) + " ")
	s.WriteString(formatFilterLabel("level", levelFilters[v.levelIdx]) + " ")
	s.WriteString(formatFilterLabel("source", sourceFilters[v.sourceIdx]) + " ")
	s.WriteString(formatFilterLabel("project", v.activeProjectFilter()) + " ")
	s.WriteString(formatFilterLabel("range", timeRangePresets[v.timeRangeIdx].Label))
	if interval := autoRefreshIntervals[v.autoRefreshIdx]; interval.Duration > 0 {
		s.WriteString("  " + Styles.Success.Render("⟳ "+interval.Label))
	}
	s.WriteString("\n\n")
	return s.String()
}

// renderEntries renders visible audit log entries in reverse chronological order.
func (v *AuditLogView) renderEntries(width, maxVisible int) string {
	var s strings.Builder

	start := len(v.entries) - 1 - v.offset
	if start < 0 {
		start = 0
	}
	end := start - maxVisible
	if end < 0 {
		end = -1
	}

	for i := start; i > end; i-- {
		s.WriteString(renderEntry(v.entries[i], width))
	}
	return s.String()
}

// renderEntry renders a single audit log entry line.
func renderEntry(e api.AuditEntry, width int) string {
	ts := e.Timestamp.Format("01/02 15:04:05")

	levelStyle := Styles.Muted
	switch e.Level {
	case api.AuditLevelWarn:
		levelStyle = Styles.Warning
	case api.AuditLevelError:
		levelStyle = Styles.Error
	}

	eventLabel := auditEventLabel(e.Event)

	projectCol := e.DisplayProject()
	prefix := ts + " " + padRight(string(e.Level), 6) + " " + padRight(string(e.Source), 12)
	prefix += " " + padRight(projectCol, 15)
	wtCol := ""
	if e.Worktree != "" {
		wtCol = "[" + e.Worktree + "]"
	}
	prefix += " " + padRight(wtCol, 15)
	prefix += " " + eventLabel

	msg := ""
	if e.Message != "" {
		remaining := width - len(prefix) - 1
		if remaining > 0 {
			msg = " " + truncate(e.Message, remaining)
		}
	}

	line := Styles.Muted.Render(ts) + " " +
		levelStyle.Render(padRight(string(e.Level), 6)) + " " +
		padRight(string(e.Source), 12)
	line += " " + padRight(projectCol, 15)
	worktreeCol := ""
	if e.Worktree != "" {
		worktreeCol = "[" + e.Worktree + "]"
	}
	line += " " + Styles.Muted.Render(padRight(worktreeCol, 15))
	line += " " + Styles.Bold.Render(eventLabel) + msg
	return line + "\n"
}

// HelpKeyMap returns the audit log view's key bindings for the help bar.
func (v *AuditLogView) HelpKeyMap() help.KeyMap {
	return v.keys
}
