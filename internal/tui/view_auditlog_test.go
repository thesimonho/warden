package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/thesimonho/warden/api"
)

func TestCategoryFiltersIncludeAllAPICategories(t *testing.T) {
	t.Parallel()

	expected := []string{"session", "agent", "prompt", "config", "budget", "system", "debug"}
	for _, cat := range expected {
		found := false
		for _, f := range categoryFilters {
			if f == cat {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("categoryFilters missing %q", cat)
		}
	}
}

func TestCycleProjectFilter(t *testing.T) {
	t.Parallel()

	t.Run("empty project list stays on all", func(t *testing.T) {
		t.Parallel()
		v := &AuditLogView{}
		v.cycleProjectFilter()
		if got := v.activeProjectFilter(); got != "" {
			t.Errorf("expected empty filter, got %q", got)
		}
	})

	t.Run("cycles through projects and back to all", func(t *testing.T) {
		t.Parallel()
		v := &AuditLogView{
			projectNames: []string{"alpha", "beta"},
		}

		v.cycleProjectFilter()
		if got := v.activeProjectFilter(); got != "alpha" {
			t.Errorf("first cycle: expected %q, got %q", "alpha", got)
		}

		v.cycleProjectFilter()
		if got := v.activeProjectFilter(); got != "beta" {
			t.Errorf("second cycle: expected %q, got %q", "beta", got)
		}

		v.cycleProjectFilter()
		if got := v.activeProjectFilter(); got != "" {
			t.Errorf("third cycle: expected empty (all), got %q", got)
		}
	})
}

func TestBuildFilters(t *testing.T) {
	t.Parallel()

	t.Run("default filters are empty", func(t *testing.T) {
		t.Parallel()
		v := &AuditLogView{}
		f := v.buildFilters()
		if f.Category != "" || f.Level != "" || f.Source != "" || f.ProjectID != "" || f.Since != "" {
			t.Errorf("default filters should all be empty, got %+v", f)
		}
	})

	t.Run("category filter is passed through", func(t *testing.T) {
		t.Parallel()
		// Index 5 = "budget" in categoryFilters.
		v := &AuditLogView{categoryIdx: 5}
		f := v.buildFilters()
		if f.Category != api.AuditCategoryBudget {
			t.Errorf("expected category %q, got %q", api.AuditCategoryBudget, f.Category)
		}
	})

	t.Run("project filter maps to ProjectID", func(t *testing.T) {
		t.Parallel()
		v := &AuditLogView{
			projectNames: []string{"my-project"},
			projectIdx:   1,
		}
		f := v.buildFilters()
		if f.ProjectID != "my-project" {
			t.Errorf("expected projectID %q, got %q", "my-project", f.ProjectID)
		}
	})

	t.Run("time range preset sets Since", func(t *testing.T) {
		t.Parallel()
		v := &AuditLogView{timeRangeIdx: 1} // 1h preset
		before := time.Now().Add(-1 * time.Hour)
		f := v.buildFilters()

		if f.Since == "" {
			t.Fatal("expected Since to be set for 1h preset")
		}
		parsed, err := time.Parse(time.RFC3339, f.Since)
		if err != nil {
			t.Fatalf("Since is not valid RFC3339: %v", err)
		}
		// Allow 5 seconds of tolerance for test execution time.
		if parsed.Before(before.Add(-5 * time.Second)) {
			t.Errorf("Since %v is too far in the past (expected around %v)", parsed, before)
		}
	})

	t.Run("zero time range does not set Since", func(t *testing.T) {
		t.Parallel()
		v := &AuditLogView{timeRangeIdx: 0}
		f := v.buildFilters()
		if f.Since != "" {
			t.Errorf("expected empty Since for all-time preset, got %q", f.Since)
		}
	})
}

func TestAutoRefreshScheduling(t *testing.T) {
	t.Parallel()

	t.Run("off interval returns nil cmd", func(t *testing.T) {
		t.Parallel()
		v := &AuditLogView{autoRefreshIdx: 0}
		cmd := v.scheduleAutoRefresh()
		if cmd != nil {
			t.Error("expected nil cmd when auto-refresh is off")
		}
	})

	t.Run("active interval returns non-nil cmd", func(t *testing.T) {
		t.Parallel()
		v := &AuditLogView{autoRefreshIdx: 1}
		cmd := v.scheduleAutoRefresh()
		if cmd == nil {
			t.Error("expected non-nil cmd when auto-refresh is on")
		}
	})
}

func TestAuditEventLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		event string
		want  string
	}{
		{"session_start", "Session Start"},
		{"tool_use", "Tool Use"},
		{"stop", "Stop"},
		{"budget_exceeded", "Budget Exceeded"},
		{"some_unknown_event", "Some Unknown Event"},
	}
	for _, tt := range tests {
		if got := auditEventLabel(tt.event); got != tt.want {
			t.Errorf("auditEventLabel(%q) = %q, want %q", tt.event, got, tt.want)
		}
	}
}

func TestRenderShowsCost(t *testing.T) {
	t.Parallel()

	v := &AuditLogView{
		summary: &api.AuditSummary{
			TotalSessions:   5,
			TotalToolUses:   10,
			TotalPrompts:    3,
			TotalCostUSD:    1.23,
			UniqueProjects:  2,
			UniqueWorktrees: 4,
		},
	}

	output := v.Render(120, 30)
	if !strings.Contains(output, "Cost: $1.23") {
		t.Error("expected cost to appear in summary line")
	}
}

func TestRenderShowsAllFilterLabels(t *testing.T) {
	t.Parallel()

	v := &AuditLogView{
		categoryIdx:  5, // "budget"
		projectNames: []string{"my-proj"},
		projectIdx:   1,
		timeRangeIdx: 2, // 6h
	}

	output := v.Render(120, 30)
	if !strings.Contains(output, "category:") {
		t.Error("missing category filter label")
	}
	if !strings.Contains(output, "project:") {
		t.Error("missing project filter label")
	}
	if !strings.Contains(output, "range:") {
		t.Error("missing range filter label")
	}
}

func TestRenderShowsAutoRefreshIndicator(t *testing.T) {
	t.Parallel()

	t.Run("no indicator when off", func(t *testing.T) {
		t.Parallel()
		v := &AuditLogView{autoRefreshIdx: 0}
		output := v.Render(120, 30)
		if strings.Contains(output, "⟳") {
			t.Error("should not show refresh indicator when off")
		}
	})

	t.Run("shows indicator when active", func(t *testing.T) {
		t.Parallel()
		v := &AuditLogView{autoRefreshIdx: 1}
		output := v.Render(120, 30)
		if !strings.Contains(output, "10s") {
			t.Error("should show 10s indicator when auto-refresh is active")
		}
	})
}

func TestRenderEntries(t *testing.T) {
	t.Parallel()

	v := &AuditLogView{
		entries: []api.AuditEntry{
			{
				Timestamp: time.Date(2026, 3, 27, 10, 0, 0, 0, time.UTC),
				Source:    api.AuditSourceAgent,
				Level:     api.AuditLevelInfo,
				Event:     "budget_exceeded",
				ProjectID: "test-project",
				Message:   "Budget limit reached",
			},
		},
	}

	output := v.Render(120, 30)
	if !strings.Contains(output, "Budget Exceeded") {
		t.Error("expected budget event label in rendered output")
	}
	if !strings.Contains(output, "test-project") {
		t.Error("expected container name in rendered output")
	}
}

func TestFormatFilterLabel(t *testing.T) {
	t.Parallel()

	t.Run("non-empty value shows bold label", func(t *testing.T) {
		t.Parallel()
		result := formatFilterLabel("category", "budget")
		if !strings.Contains(result, "category:") || !strings.Contains(result, "budget") {
			t.Errorf("expected formatted label, got %q", result)
		}
	})

	t.Run("empty value shows all", func(t *testing.T) {
		t.Parallel()
		result := formatFilterLabel("category", "")
		if !strings.Contains(result, "category:all") {
			t.Errorf("expected 'category:all', got %q", result)
		}
	})
}

func TestTimeRangePresetsOrder(t *testing.T) {
	t.Parallel()

	if timeRangePresets[0].Duration != 0 {
		t.Error("first time range preset should be zero (all time)")
	}

	for i := 1; i < len(timeRangePresets); i++ {
		if timeRangePresets[i].Duration <= timeRangePresets[i-1].Duration {
			t.Errorf("preset %d (%s) should have longer duration than preset %d (%s)",
				i, timeRangePresets[i].Label, i-1, timeRangePresets[i-1].Label)
		}
	}
}

func TestAutoRefreshIntervalsOrder(t *testing.T) {
	t.Parallel()

	if autoRefreshIntervals[0].Duration != 0 {
		t.Error("first auto-refresh interval should be zero (off)")
	}

	for i := 1; i < len(autoRefreshIntervals); i++ {
		if autoRefreshIntervals[i].Duration <= autoRefreshIntervals[i-1].Duration {
			t.Errorf("interval %d (%s) should be longer than interval %d (%s)",
				i, autoRefreshIntervals[i].Label, i-1, autoRefreshIntervals[i-1].Label)
		}
	}
}

func TestActiveProjectFilter(t *testing.T) {
	t.Parallel()

	t.Run("returns empty when no projects", func(t *testing.T) {
		t.Parallel()
		v := &AuditLogView{}
		if got := v.activeProjectFilter(); got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})

	t.Run("returns empty at index 0", func(t *testing.T) {
		t.Parallel()
		v := &AuditLogView{
			projectNames: []string{"alpha"},
			projectIdx:   0,
		}
		if got := v.activeProjectFilter(); got != "" {
			t.Errorf("expected empty at index 0, got %q", got)
		}
	})

	t.Run("returns correct project at index 1", func(t *testing.T) {
		t.Parallel()
		v := &AuditLogView{
			projectNames: []string{"alpha", "beta"},
			projectIdx:   2,
		}
		if got := v.activeProjectFilter(); got != "beta" {
			t.Errorf("expected %q, got %q", "beta", got)
		}
	})
}

func TestAutoRefreshGeneration(t *testing.T) {
	t.Parallel()

	v := &AuditLogView{autoRefreshGen: 0}

	// Stale tick with wrong generation should be ignored.
	result, cmd := v.Update(autoRefreshTickMsg{generation: 5})
	if cmd != nil {
		t.Error("stale tick should return nil cmd")
	}
	_ = result
}
