package engine

import (
	"testing"

	"github.com/thesimonho/warden/api"
)

func TestIsValidWorktreeID(t *testing.T) {
	t.Parallel()

	validCases := []string{
		"main",
		"feature-branch",
		"fix.something",
		"myWorktree",
		"task_123",
		"a",
		"A1",
	}

	for _, id := range validCases {
		if !IsValidWorktreeID(id) {
			t.Errorf("expected %q to be valid", id)
		}
	}
}

func TestIsValidWorktreeID_RejectsInvalid(t *testing.T) {
	t.Parallel()

	invalidCases := []struct {
		name string
		id   string
	}{
		{name: "empty string", id: ""},
		{name: "path traversal", id: "../../../etc/passwd"},
		{name: "starts with hyphen", id: "-feature"},
		{name: "starts with dot", id: ".hidden"},
		{name: "starts with underscore", id: "_private"},
		{name: "has spaces", id: "my branch"},
		{name: "has slash", id: "feature/branch"},
		{name: "shell injection", id: "$(rm -rf /)"},
		{name: "null bytes", id: "worktree\x00id"},
	}

	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if IsValidWorktreeID(tc.id) {
				t.Errorf("expected %q to fail validation", tc.id)
			}
		})
	}
}

func TestWorktreeIDFromPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		path     string
		wsDir    string
		expected string
	}{
		{name: "project root", path: "/home/warden/my-app", wsDir: "/home/warden/my-app", expected: "main"},
		{name: "claude worktree path", path: "/home/warden/my-app/.claude/worktrees/griod", wsDir: "/home/warden/my-app", expected: "griod"},
		{name: "claude worktree with hyphens", path: "/home/warden/my-app/.claude/worktrees/my-feature", wsDir: "/home/warden/my-app", expected: "my-feature"},
		{name: "claude worktree with dots", path: "/home/warden/my-app/.claude/worktrees/fix.bug", wsDir: "/home/warden/my-app", expected: "fix.bug"},
		{name: "warden worktree path", path: "/home/warden/my-app/.warden/worktrees/feature-x", wsDir: "/home/warden/my-app", expected: "feature-x"},
		{name: "warden worktree with dots", path: "/home/warden/my-app/.warden/worktrees/fix.bug", wsDir: "/home/warden/my-app", expected: "fix.bug"},
		{name: "unrecognized path", path: "/some/other/path", wsDir: "/home/warden/my-app", expected: "main"},
		{name: "/project path", path: "/project", wsDir: "/project", expected: "main"},
		{name: "/project claude worktree", path: "/project/.claude/worktrees/feat", wsDir: "/project", expected: "feat"},
		{name: "/project warden worktree", path: "/project/.warden/worktrees/feat", wsDir: "/project", expected: "feat"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := worktreeIDFromPath(tc.path, tc.wsDir)
			if got != tc.expected {
				t.Errorf("worktreeIDFromPath(%q, %q) = %q, want %q", tc.path, tc.wsDir, got, tc.expected)
			}
		})
	}
}

func TestParseGitWorktreeList(t *testing.T) {
	t.Parallel()

	output := `worktree /project
HEAD abc123def456789
branch refs/heads/main

worktree /project/.claude/worktrees/feature-x
HEAD def456abc123789
branch refs/heads/feature-x

worktree /project/.claude/worktrees/fix.login
HEAD 789abc123def456
branch refs/heads/fix/login
`

	worktrees := parseGitWorktreeList("container-123", output, "/project")

	if len(worktrees) != 3 {
		t.Fatalf("expected 3 worktrees, got %d", len(worktrees))
	}

	// Main project root
	if worktrees[0].ID != "main" {
		t.Errorf("expected ID 'main', got %q", worktrees[0].ID)
	}
	if worktrees[0].Path != "/project" {
		t.Errorf("expected path '/project', got %q", worktrees[0].Path)
	}
	if worktrees[0].Branch != "main" {
		t.Errorf("expected branch 'main', got %q", worktrees[0].Branch)
	}
	if worktrees[0].ProjectID != "container-123" {
		t.Errorf("expected project ID 'container-123', got %q", worktrees[0].ProjectID)
	}
	if worktrees[0].State != WorktreeStateDisconnected {
		t.Errorf("expected disconnected state, got %q", worktrees[0].State)
	}

	// Feature worktree
	if worktrees[1].ID != "feature-x" {
		t.Errorf("expected ID 'feature-x', got %q", worktrees[1].ID)
	}
	if worktrees[1].Branch != "feature-x" {
		t.Errorf("expected branch 'feature-x', got %q", worktrees[1].Branch)
	}

	// Fix worktree (branch has slash but ID doesn't)
	if worktrees[2].ID != "fix.login" {
		t.Errorf("expected ID 'fix.login', got %q", worktrees[2].ID)
	}
	if worktrees[2].Branch != "fix/login" {
		t.Errorf("expected branch 'fix/login', got %q", worktrees[2].Branch)
	}
}

func TestParseGitWorktreeList_ClaudeWorktreePath(t *testing.T) {
	t.Parallel()

	// Worktrees created via `claude --worktree` land in .claude/worktrees/
	output := `worktree /project
HEAD abc123def456789
branch refs/heads/develop

worktree /project/.claude/worktrees/griod
HEAD def456abc123789
branch refs/heads/worktree-griod
`

	worktrees := parseGitWorktreeList("ctr", output, "/project")

	if len(worktrees) != 2 {
		t.Fatalf("expected 2 worktrees, got %d", len(worktrees))
	}
	if worktrees[1].ID != "griod" {
		t.Errorf("expected ID 'griod', got %q", worktrees[1].ID)
	}
	if worktrees[1].Branch != "worktree-griod" {
		t.Errorf("expected branch 'worktree-griod', got %q", worktrees[1].Branch)
	}
}

func TestParseGitWorktreeList_SkipsPrunable(t *testing.T) {
	t.Parallel()

	// A prunable worktree (gitdir broken) should be silently dropped.
	// This is the exact scenario Simon hit: griod worktree lingering in git
	// metadata after the worktree directory was deleted.
	output := `worktree /project
HEAD abc123def456789
branch refs/heads/develop

worktree /project/.claude/worktrees/griod
HEAD def456abc123789
branch refs/heads/worktree-griod
prunable gitdir file points to non-existent location
`

	worktrees := parseGitWorktreeList("ctr", output, "/project")

	if len(worktrees) != 1 {
		t.Fatalf("expected 1 worktree (prunable skipped), got %d", len(worktrees))
	}
	if worktrees[0].ID != "main" {
		t.Errorf("expected only 'main' worktree, got %q", worktrees[0].ID)
	}
}

func TestParseGitWorktreeList_Empty(t *testing.T) {
	t.Parallel()

	worktrees := parseGitWorktreeList("ctr", "", "/project")
	if len(worktrees) != 0 {
		t.Errorf("expected 0 worktrees from empty output, got %d", len(worktrees))
	}
}

func TestParseGitWorktreeList_SingleWorktree(t *testing.T) {
	t.Parallel()

	output := `worktree /project
HEAD abc123
branch refs/heads/main
`

	worktrees := parseGitWorktreeList("ctr", output, "/project")
	if len(worktrees) != 1 {
		t.Fatalf("expected 1 worktree, got %d", len(worktrees))
	}
	if worktrees[0].ID != "main" {
		t.Errorf("expected ID 'main', got %q", worktrees[0].ID)
	}
}

func TestParseTerminalBatch(t *testing.T) {
	t.Parallel()

	output := `---WT_START:main---
---EXIT_END---1
---ABDUCO_END------WT_START:feature-x---0
---EXIT_END---1
---ABDUCO_END---`

	states := parseTerminalBatch(output)

	if len(states) != 2 {
		t.Fatalf("expected 2 terminal states, got %d", len(states))
	}

	// Main worktree: no exit code, abduco alive
	mainState := states["main"]
	if mainState == nil {
		t.Fatal("expected state for 'main'")
		return // unreachable — staticcheck SA5011
	}
	if mainState.exitCode != -1 {
		t.Errorf("expected exit code -1 (not set), got %d", mainState.exitCode)
	}
	if !mainState.abducoAlive {
		t.Error("expected abducoAlive=true")
	}

	// Feature worktree: exit code 0, abduco alive
	featureState := states["feature-x"]
	if featureState == nil {
		t.Fatal("expected state for 'feature-x'")
		return // unreachable — staticcheck SA5011
	}
	if featureState.exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", featureState.exitCode)
	}
	if !featureState.abducoAlive {
		t.Error("expected abducoAlive=true")
	}
}

func TestParseTerminalBatch_Empty(t *testing.T) {
	t.Parallel()

	states := parseTerminalBatch("")
	if len(states) != 0 {
		t.Errorf("expected 0 states from empty output, got %d", len(states))
	}
}

func TestParseTerminalBatch_NoExitCode(t *testing.T) {
	t.Parallel()

	output := `---WT_START:orphan---
---EXIT_END---0
---ABDUCO_END---`

	states := parseTerminalBatch(output)

	orphan := states["orphan"]
	if orphan == nil {
		t.Fatal("expected state for 'orphan'")
		return // unreachable — staticcheck SA5011
	}
	if orphan.exitCode != -1 {
		t.Errorf("expected exit code -1, got %d", orphan.exitCode)
	}
	if orphan.abducoAlive {
		t.Error("expected abducoAlive=false")
	}
}

func TestParseTerminalBatch_AbducoAlive(t *testing.T) {
	t.Parallel()

	// Simulate: abduco alive
	output := `---WT_START:bg-task---
---EXIT_END---1
---ABDUCO_END---`

	states := parseTerminalBatch(output)

	bg := states["bg-task"]
	if bg == nil {
		t.Fatal("expected state for 'bg-task'")
		return // unreachable — staticcheck SA5011
	}
	if !bg.abducoAlive {
		t.Error("expected abducoAlive=true")
	}
}

func TestParseTerminalBatch_AbducoDead(t *testing.T) {
	t.Parallel()

	// Simulate: abduco dead
	output := `---WT_START:dead-task---
---EXIT_END---0
---ABDUCO_END---`

	states := parseTerminalBatch(output)

	dead := states["dead-task"]
	if dead == nil {
		t.Fatal("expected state for 'dead-task'")
		return // unreachable — staticcheck SA5011
	}
	if dead.abducoAlive {
		t.Error("expected abducoAlive=false")
	}
}

func TestParseTerminalBatch_ConnectedWithAbduco(t *testing.T) {
	t.Parallel()

	// Simulate: abduco alive (normal connected state)
	output := `---WT_START:active---
---EXIT_END---1
---ABDUCO_END---`

	states := parseTerminalBatch(output)

	active := states["active"]
	if active == nil {
		t.Fatal("expected state for 'active'")
		return // unreachable — staticcheck SA5011
	}
	if !active.abducoAlive {
		t.Error("expected abducoAlive=true")
	}
}

func TestParseTerminalBatch_BackwardsCompatible(t *testing.T) {
	t.Parallel()

	// Output without ABDUCO_END marker (old container image)
	output := `---WT_START:legacy---
---EXIT_END---`

	states := parseTerminalBatch(output)

	legacy := states["legacy"]
	if legacy == nil {
		t.Fatal("expected state for 'legacy'")
		return // unreachable — staticcheck SA5011
	}
	if legacy.abducoAlive {
		t.Error("expected abducoAlive=false for legacy output")
	}
}

func TestParseDiffOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		input         string
		wantFiles     int
		wantRawDiff   bool
		wantTruncated bool
	}{
		{
			name:      "empty output",
			input:     "",
			wantFiles: 0,
		},
		{
			name:      "separator only — no changes",
			input:     "---WARDEN_DIFF_SEP---",
			wantFiles: 0,
		},
		{
			name:        "tracked changes with raw diff",
			input:       "5\t2\tmain.go\n---WARDEN_DIFF_SEP---\ndiff --git a/main.go b/main.go\n--- a/main.go\n+++ b/main.go",
			wantFiles:   1,
			wantRawDiff: true,
		},
		{
			name:      "numstat but no raw diff",
			input:     "3\t0\tREADME.md\n---WARDEN_DIFF_SEP---",
			wantFiles: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := parseDiffOutput(tc.input)
			if len(got.Files) != tc.wantFiles {
				t.Errorf("got %d files, want %d", len(got.Files), tc.wantFiles)
			}
			if tc.wantRawDiff && got.RawDiff == "" {
				t.Error("expected non-empty RawDiff")
			}
			if got.Truncated != tc.wantTruncated {
				t.Errorf("Truncated = %v, want %v", got.Truncated, tc.wantTruncated)
			}
		})
	}
}

func TestParseNumstat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected []api.DiffFileSummary
	}{
		{
			name:  "normal files",
			input: "10\t2\tsrc/main.go\n3\t0\tREADME.md\n0\t5\told.txt",
			expected: []api.DiffFileSummary{
				{Path: "src/main.go", Additions: 10, Deletions: 2, Status: "modified"},
				{Path: "README.md", Additions: 3, Deletions: 0, Status: "modified"},
				{Path: "old.txt", Additions: 0, Deletions: 5, Status: "deleted"},
			},
		},
		{
			name:  "binary file",
			input: "-\t-\timage.png",
			expected: []api.DiffFileSummary{
				{Path: "image.png", IsBinary: true, Status: "modified"},
			},
		},
		{
			name:  "rename with arrow syntax",
			input: "5\t3\t{old => new}/file.go",
			expected: []api.DiffFileSummary{
				{Path: "new/file.go", OldPath: "old/file.go", Additions: 5, Deletions: 3, Status: "renamed"},
			},
		},
		{
			name:  "rename at root level",
			input: "0\t0\t{old.txt => new.txt}",
			expected: []api.DiffFileSummary{
				{Path: "new.txt", OldPath: "old.txt", Additions: 0, Deletions: 0, Status: "renamed"},
			},
		},
		{
			name:  "rename nested",
			input: "2\t1\tsrc/{utils => helpers}/parse.go",
			expected: []api.DiffFileSummary{
				{Path: "src/helpers/parse.go", OldPath: "src/utils/parse.go", Additions: 2, Deletions: 1, Status: "renamed"},
			},
		},
		{
			name:     "empty input",
			input:    "",
			expected: nil,
		},
		{
			name:     "whitespace only",
			input:    "  \n  \n",
			expected: nil,
		},
		{
			name:  "tracked file with only additions is modified not added",
			input: "50\t0\texisting-file.ts",
			expected: []api.DiffFileSummary{
				{Path: "existing-file.ts", Additions: 50, Deletions: 0, Status: "modified"},
			},
		},
		{
			name:  "deleted file (all deletions)",
			input: "0\t30\tremoved.ts",
			expected: []api.DiffFileSummary{
				{Path: "removed.ts", Additions: 0, Deletions: 30, Status: "deleted"},
			},
		},
		{
			name:  "path with spaces",
			input: "3\t1\tmy project/some file.txt",
			expected: []api.DiffFileSummary{
				{Path: "my project/some file.txt", Additions: 3, Deletions: 1, Status: "modified"},
			},
		},
		{
			name:  "untracked file marker",
			input: "25\t0\tnew-file.go\t[untracked]",
			expected: []api.DiffFileSummary{
				{Path: "new-file.go", Additions: 25, Deletions: 0, Status: "added"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := parseNumstat(tc.input)
			if len(got) != len(tc.expected) {
				t.Fatalf("parseNumstat() returned %d files, want %d", len(got), len(tc.expected))
			}
			for i := range tc.expected {
				if got[i].Path != tc.expected[i].Path {
					t.Errorf("[%d] Path = %q, want %q", i, got[i].Path, tc.expected[i].Path)
				}
				if got[i].OldPath != tc.expected[i].OldPath {
					t.Errorf("[%d] OldPath = %q, want %q", i, got[i].OldPath, tc.expected[i].OldPath)
				}
				if got[i].Additions != tc.expected[i].Additions {
					t.Errorf("[%d] Additions = %d, want %d", i, got[i].Additions, tc.expected[i].Additions)
				}
				if got[i].Deletions != tc.expected[i].Deletions {
					t.Errorf("[%d] Deletions = %d, want %d", i, got[i].Deletions, tc.expected[i].Deletions)
				}
				if got[i].IsBinary != tc.expected[i].IsBinary {
					t.Errorf("[%d] IsBinary = %v, want %v", i, got[i].IsBinary, tc.expected[i].IsBinary)
				}
				if got[i].Status != tc.expected[i].Status {
					t.Errorf("[%d] Status = %q, want %q", i, got[i].Status, tc.expected[i].Status)
				}
			}
		})
	}
}
