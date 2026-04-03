package components

import (
	"regexp"
	"strings"
	"testing"
)

// stripANSI removes ANSI escape sequences for content matching.
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

func TestRenderTabBar(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		labels      []string
		activeIndex int
		width       int
		wantLabels  []string // expected label text within the tab bar
	}{
		{
			name:        "two tabs first active",
			labels:      []string{"Projects", "Settings"},
			activeIndex: 0,
			width:       60,
			wantLabels:  []string{"[1] Projects", "[2] Settings"},
		},
		{
			name:        "two tabs second active",
			labels:      []string{"Projects", "Settings"},
			activeIndex: 1,
			width:       60,
			wantLabels:  []string{"[1] Projects", "[2] Settings"},
		},
		{
			name:        "three tabs last active",
			labels:      []string{"Projects", "Settings", "Event Log"},
			activeIndex: 2,
			width:       80,
			wantLabels:  []string{"[1] Projects", "[2] Settings", "[3] Event Log"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := RenderTabBar(tt.labels, tt.activeIndex, tt.width, "")
			plain := stripANSI(got)
			for _, label := range tt.wantLabels {
				if !strings.Contains(plain, label) {
					t.Errorf("RenderTabBar() missing label %q in:\n%s", label, plain)
				}
			}
		})
	}
}

func TestRenderTabBarEmpty(t *testing.T) {
	t.Parallel()

	got := RenderTabBar([]string{}, 0, 60, "")
	plain := stripANSI(got)
	// Should only contain border fill, no tab labels.
	if strings.Contains(plain, "[") {
		t.Errorf("empty labels should have no tab labels, got %q", plain)
	}
}

func TestRenderTabBarHasThreeLines(t *testing.T) {
	t.Parallel()

	got := RenderTabBar([]string{"Projects", "Settings"}, 0, 60, "")
	lines := strings.Split(got, "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 lines (top border, content, bottom border), got %d:\n%s", len(lines), got)
	}
}

func TestRenderTabBarShowsVersion(t *testing.T) {
	t.Parallel()

	got := RenderTabBar([]string{"Projects", "Settings"}, 0, 80, "v0.5.2")
	plain := stripANSI(got)
	if !strings.Contains(plain, "v0.5.2") {
		t.Errorf("expected version string in tab bar, got:\n%s", plain)
	}
}

func TestRenderTabBarActiveHasOpenBottom(t *testing.T) {
	t.Parallel()

	got := RenderTabBar([]string{"A", "B"}, 0, 40, "")
	plain := stripANSI(got)
	lines := strings.Split(plain, "\n")
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines, got %d", len(lines))
	}
	bottomLine := lines[2]
	// Active tab's bottom border should have spaces (open), not ─.
	if !strings.Contains(bottomLine, "┘") || !strings.Contains(bottomLine, "└") {
		t.Errorf("active tab bottom should have ┘ and └ corners, got: %s", bottomLine)
	}
}
