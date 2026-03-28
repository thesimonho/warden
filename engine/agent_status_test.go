package engine

import (
	"testing"

	"github.com/thesimonho/warden/agent"
)

func TestIsEstimatedCostFromConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{name: "subscription user", raw: `{"oauthAccount":{"billingType":"stripe_subscription"}}`, want: true},
		{name: "API key user", raw: `{"oauthAccount":{"billingType":"api_key"}}`, want: false},
		{name: "no oauth account", raw: `{"numStartups":1}`, want: false},
		{name: "empty config", raw: `{}`, want: false},
		{name: "invalid JSON", raw: `not-json`, want: true},
		{name: "null billing type", raw: `{"oauthAccount":{"billingType":""}}`, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := isEstimatedCostFromConfig([]byte(tt.raw))
			if got != tt.want {
				t.Errorf("isEstimatedCostFromConfig(%q) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestContainerWorkspaceDir(t *testing.T) {
	t.Parallel()

	got := ContainerWorkspaceDir("my-project")
	if got != "/home/dev/my-project" {
		t.Errorf("ContainerWorkspaceDir(\"my-project\") = %q, want /home/dev/my-project", got)
	}
}

func TestProjectCostFromContainerStatuses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		statuses map[string]*agent.Status
		prefix   string
		want     float64
	}{
		{name: "nil map", statuses: nil, prefix: "/home/dev/test", want: 0},
		{name: "empty map", statuses: map[string]*agent.Status{}, prefix: "/home/dev/test", want: 0},
		{name: "single entry", statuses: map[string]*agent.Status{
			"/home/dev/app": {CostUSD: 1.23},
		}, prefix: "/home/dev/app", want: 1.23},
		{name: "multiple entries sums matching prefix", statuses: map[string]*agent.Status{
			"/home/dev/app":                        {CostUSD: 1.00},
			"/home/dev/app/.claude/worktrees/feat": {CostUSD: 0.50},
			"/home/dev/app/.claude/worktrees/fix":  {CostUSD: 0.25},
		}, prefix: "/home/dev/app", want: 1.75},
		{name: "nil entries skipped", statuses: map[string]*agent.Status{
			"/home/dev/app":                        {CostUSD: 1.00},
			"/home/dev/app/.claude/worktrees/feat": nil,
		}, prefix: "/home/dev/app", want: 1.00},
		{name: "filters out non-matching paths", statuses: map[string]*agent.Status{
			"/home/dev/my-app":                        {CostUSD: 1.00},
			"/home/dev/my-app/.claude/worktrees/feat": {CostUSD: 0.50},
			"/home/user/other-project":                {CostUSD: 5.00},
			"/run/media/Projects/Services/myapp":      {CostUSD: 3.77},
		}, prefix: "/home/dev/my-app", want: 1.50},
		{name: "empty prefix defaults to /project", statuses: map[string]*agent.Status{
			"/project":                        {CostUSD: 2.00},
			"/project/.claude/worktrees/feat": {CostUSD: 0.50},
			"/home/user/other":                {CostUSD: 9.99},
		}, prefix: "", want: 2.50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ProjectCostFromContainerStatuses(tt.statuses, tt.prefix)
			if got != tt.want {
				t.Errorf("ProjectCostFromContainerStatuses() = %f, want %f", got, tt.want)
			}
		})
	}
}
