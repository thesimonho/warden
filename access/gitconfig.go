package access

import (
	"bufio"
	"path/filepath"
	"regexp"
	"strings"
)

// includeHeaderRe matches [include] and [includeIf "..."] section headers.
var includeHeaderRe = regexp.MustCompile(`(?i)^\s*\[(include(?:If\s+"[^"]*")?)\]\s*$`)

// pathLineRe matches path = value lines within include sections. Captures
// the leading whitespace+key portion (group 1) and the path value (group 2),
// handling both quoted and unquoted values.
var pathLineRe = regexp.MustCompile(`^(\s*path\s*=\s*)("?)(.+?)("?)\s*$`)

// sectionHeaderRe matches any git config section header to detect when
// we've left an include/includeIf section.
var sectionHeaderRe = regexp.MustCompile(`^\s*\[`)

// ParseGitIncludePaths extracts all include/includeIf path values from
// a gitconfig file's content. Returns paths as they appear in the file
// (may use ~/, be relative, etc.). Handles both quoted and unquoted values.
func ParseGitIncludePaths(content string) []string {
	var paths []string
	inInclude := false

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()

		if includeHeaderRe.MatchString(line) {
			inInclude = true
			continue
		}

		if sectionHeaderRe.MatchString(line) {
			inInclude = false
			continue
		}

		if !inInclude {
			continue
		}

		m := pathLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}

		path := m[3]
		if path != "" {
			paths = append(paths, path)
		}
	}

	return paths
}

// RewriteGitIncludePaths replaces include/includeIf path values in
// gitconfig content using the provided mapping from original path to
// new path. Only paths present in the map are rewritten; all other
// content (comments, section headers, conditions) is preserved exactly.
func RewriteGitIncludePaths(content string, pathMap map[string]string) string {
	if len(pathMap) == 0 {
		return content
	}

	var result strings.Builder
	result.Grow(len(content))

	inInclude := false
	scanner := bufio.NewScanner(strings.NewReader(content))
	first := true

	for scanner.Scan() {
		line := scanner.Text()

		if !first {
			result.WriteByte('\n')
		}
		first = false

		if includeHeaderRe.MatchString(line) {
			inInclude = true
			result.WriteString(line)
			continue
		}

		if sectionHeaderRe.MatchString(line) {
			inInclude = false
			result.WriteString(line)
			continue
		}

		if !inInclude {
			result.WriteString(line)
			continue
		}

		m := pathLineRe.FindStringSubmatch(line)
		if m == nil {
			result.WriteString(line)
			continue
		}

		originalPath := m[3]
		if newPath, ok := pathMap[originalPath]; ok {
			// Preserve the original formatting: prefix, optional quotes,
			// and trailing quote.
			result.WriteString(m[1])
			result.WriteString(m[2])
			result.WriteString(newPath)
			result.WriteString(m[4])
		} else {
			result.WriteString(line)
		}
	}

	// Preserve trailing newline if the original content had one.
	if strings.HasSuffix(content, "\n") {
		result.WriteByte('\n')
	}

	return result.String()
}

// ResolveIncludePath resolves a gitconfig include path to an absolute
// host path. Handles tilde expansion and relative path resolution
// (relative to the directory containing the gitconfig file).
func ResolveIncludePath(path string, configDir string) string {
	if strings.HasPrefix(path, "~/") {
		return expandHome(path)
	}
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(configDir, path)
}

// ContainerGitIncludePath returns the container path for a git include
// file, placed under [ContainerGitIncludeDir]. Uses the file's basename;
// when disambiguation is needed, the caller appends a suffix.
func ContainerGitIncludePath(basename string) string {
	return ContainerGitIncludeDir + "/" + basename
}
