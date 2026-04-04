package engine

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/thesimonho/warden/api"
)

// maxRawDiffBytes is the maximum size of the raw diff output before truncation.
const maxRawDiffBytes = 1 << 20 // 1 MB

// diffSeparator delimits numstat output from the raw unified diff.
const diffSeparator = "---WARDEN_DIFF_SEP---"

// untrackedMarker is appended to numstat lines for untracked files so
// parseNumstat can distinguish them from tracked modified files.
const untrackedMarker = "\t[untracked]"

// GetWorktreeDiff returns the uncommitted changes (tracked + untracked) for a
// worktree inside the container. Uses a temporary index copy so the real
// index is never modified.
func (ec *EngineClient) GetWorktreeDiff(ctx context.Context, containerID, worktreeID string) (*api.DiffResponse, error) {
	worktreePath := ec.resolveWorktreePath(ctx, containerID, worktreeID)

	// Single exec: copy the git index, intent-to-add untracked files on the
	// copy, then run numstat + unified diff. The awk script tags untracked
	// files with [untracked] in O(1) per line. It reads the untracked list
	// in BEGIN via getline to avoid the NR==FNR empty-file bug (when the
	// first file is empty, NR==FNR stays true for stdin lines too, causing
	// the entire numstat to be swallowed).
	cmd := fmt.Sprintf(
		`cd %[1]s 2>/dev/null || exit 0; `+
			`git rev-parse --git-dir >/dev/null 2>&1 || exit 0; `+
			`gitdir=$(git rev-parse --git-dir); `+
			`tmpidx=$(mktemp); `+
			`cp "$gitdir/index" "$tmpidx" 2>/dev/null || true; `+
			`uf=/tmp/warden_untracked$$; `+
			`git -c safe.directory=%[1]s ls-files --others --exclude-standard 2>/dev/null > "$uf"; `+
			`GIT_INDEX_FILE="$tmpidx" git -c safe.directory=%[1]s add -N . 2>/dev/null; `+
			`GIT_INDEX_FILE="$tmpidx" git -c safe.directory=%[1]s diff HEAD --numstat 2>/dev/null | `+
			`awk -v uf="$uf" 'BEGIN{while((getline line < uf)>0) u[line]; close(uf)} {f=$3; for(i=4;i<=NF;i++) f=f" "$i; if(f in u) print $0"\t[untracked]"; else print}'; `+
			`echo '%[2]s'; `+
			`GIT_INDEX_FILE="$tmpidx" git -c safe.directory=%[1]s diff HEAD 2>/dev/null; `+
			`rm -f "$tmpidx" "$uf"`,
		worktreePath, diffSeparator,
	)

	output, err := ec.execAndCapture(ctx, containerID, container.ExecOptions{
		Cmd:          []string{"sh", "-c", cmd},
		User:         ContainerUser,
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		// Non-git repos or errors return empty response, not an error.
		slog.Debug("worktree diff failed", "container", containerID, "worktree", worktreeID, "err", err)
		return &api.DiffResponse{Files: []api.DiffFileSummary{}}, nil
	}

	return parseDiffOutput(output), nil
}

// parseDiffOutput splits the combined exec output into numstat + raw diff
// and returns a fully populated DiffResponse.
func parseDiffOutput(output string) *api.DiffResponse {
	resp := &api.DiffResponse{}

	parts := strings.SplitN(output, diffSeparator, 2)
	numstatSection := ""
	rawDiff := ""
	if len(parts) >= 1 {
		numstatSection = strings.TrimSpace(parts[0])
	}
	if len(parts) >= 2 {
		rawDiff = strings.TrimSpace(parts[1])
	}

	if files := parseNumstat(numstatSection); files != nil {
		resp.Files = files
	} else {
		resp.Files = []api.DiffFileSummary{}
	}

	// Truncate raw diff if too large.
	if len(rawDiff) > maxRawDiffBytes {
		rawDiff = rawDiff[:maxRawDiffBytes]
		resp.Truncated = true
	}
	resp.RawDiff = rawDiff

	for _, f := range resp.Files {
		resp.TotalAdditions += f.Additions
		resp.TotalDeletions += f.Deletions
	}

	return resp
}

// parseNumstat parses git diff --numstat output into file summaries.
//
// Format:
//
//	<add>\t<del>\t<path>          — normal file
//	-\t-\t<path>                  — binary file
//	<add>\t<del>\t{old => new}    — rename (various patterns)
//	<add>\t<del>\t<path>\t[untracked] — untracked file (Warden marker)
func parseNumstat(input string) []api.DiffFileSummary {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil
	}

	var files []api.DiffFileSummary
	for _, line := range strings.Split(input, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Check for untracked marker at the end.
		isUntracked := strings.HasSuffix(line, untrackedMarker)
		if isUntracked {
			line = strings.TrimSuffix(line, untrackedMarker)
		}

		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 3 {
			continue
		}

		addStr, delStr, path := parts[0], parts[1], parts[2]

		var f api.DiffFileSummary

		// Binary files show "-" for both add and delete counts.
		if addStr == "-" && delStr == "-" {
			f.Path = path
			f.IsBinary = true
			f.Status = "modified"
			files = append(files, f)
			continue
		}

		f.Additions, _ = strconv.Atoi(addStr)
		f.Deletions, _ = strconv.Atoi(delStr)

		// Detect renames: git uses {old => new} inside the path.
		if strings.Contains(path, " => ") && strings.Contains(path, "{") {
			f.Path, f.OldPath = parseRenamePath(path)
			f.Status = "renamed"
		} else {
			f.Path = path
			f.Status = deriveFileStatus(f.Additions, f.Deletions, isUntracked)
		}

		files = append(files, f)
	}

	return files
}

// deriveFileStatus infers the change type from line counts.
// Only the [untracked] marker reliably indicates a new file — a tracked file
// with additions-only is a modification, not an addition.
func deriveFileStatus(additions, deletions int, isUntracked bool) string {
	if isUntracked {
		return "added"
	}
	if additions == 0 && deletions > 0 {
		return "deleted"
	}
	return "modified"
}

// parseRenamePath extracts old and new paths from git's rename notation.
//
// Examples:
//
//	{old.txt => new.txt}            → new.txt, old.txt
//	src/{utils => helpers}/parse.go → src/helpers/parse.go, src/utils/parse.go
//	{old => new}/file.go            → new/file.go, old/file.go
func parseRenamePath(path string) (newPath, oldPath string) {
	braceStart := strings.Index(path, "{")
	braceEnd := strings.Index(path, "}")
	if braceStart < 0 || braceEnd < 0 || braceEnd <= braceStart {
		return path, ""
	}

	prefix := path[:braceStart]
	suffix := path[braceEnd+1:]
	inner := path[braceStart+1 : braceEnd]

	arrowIdx := strings.Index(inner, " => ")
	if arrowIdx < 0 {
		return path, ""
	}

	oldPart := inner[:arrowIdx]
	newPart := inner[arrowIdx+4:]

	return prefix + newPart + suffix, prefix + oldPart + suffix
}
