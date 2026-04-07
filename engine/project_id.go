package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
)

// projectIDLength is the number of hex characters in a project ID.
const projectIDLength = 12

// projectIDRegexp validates a project ID: exactly 12 lowercase hex characters.
var projectIDRegexp = regexp.MustCompile(`^[0-9a-f]{12}$`)

// ProjectID computes a deterministic project identifier from a host directory path.
// The ID is the first 12 hex characters of the SHA-256 hash of the cleaned,
// absolute path. Trailing slashes are stripped before hashing.
//
// The caller is responsible for resolving symlinks (via filepath.EvalSymlinks)
// before calling this function — symlink resolution requires filesystem access
// which is inappropriate at this level.
func ProjectID(hostPath string) (string, error) {
	cleaned := filepath.Clean(hostPath)
	if !filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("project path must be absolute: %q", hostPath)
	}

	// Normalize: strip any trailing separator that Clean might leave on root paths.
	cleaned = strings.TrimRight(cleaned, string(filepath.Separator))
	if cleaned == "" {
		// Edge case: root path "/" becomes empty after trimming.
		cleaned = string(filepath.Separator)
	}

	hash := sha256.Sum256([]byte(cleaned))
	return hex.EncodeToString(hash[:])[:projectIDLength], nil
}

// ProjectIDFromURL computes a deterministic project identifier from a git
// clone URL. The URL is normalized (lowercase host, stripped trailing .git
// and slash) before hashing so that equivalent URLs produce the same ID.
func ProjectIDFromURL(cloneURL string) (string, error) {
	normalized := normalizeCloneURL(cloneURL)
	if normalized == "" {
		return "", fmt.Errorf("clone URL is required")
	}

	hash := sha256.Sum256([]byte(normalized))
	id := hex.EncodeToString(hash[:])[:projectIDLength]
	if !projectIDRegexp.MatchString(id) {
		return "", fmt.Errorf("generated invalid project ID: %s", id)
	}
	return id, nil
}

// normalizeCloneURL produces a canonical form of a git clone URL for
// deterministic hashing. Handles both HTTPS and SSH (git@host:org/repo) URLs.
func normalizeCloneURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	// Handle SSH URLs: git@host:org/repo.git → git@host:org/repo
	if strings.Contains(raw, "@") && strings.Contains(raw, ":") && !strings.Contains(raw, "://") {
		// SSH format: git@github.com:org/repo.git
		parts := strings.SplitN(raw, "@", 2)
		if len(parts) == 2 {
			hostAndPath := parts[1]
			colonIdx := strings.Index(hostAndPath, ":")
			if colonIdx > 0 {
				host := strings.ToLower(hostAndPath[:colonIdx])
				path := hostAndPath[colonIdx+1:]
				path = strings.TrimSuffix(path, ".git")
				path = strings.TrimRight(path, "/")
				return parts[0] + "@" + host + ":" + path
			}
		}
	}

	// Handle HTTPS URLs: https://github.com/org/repo.git → https://github.com/org/repo
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Path = strings.TrimSuffix(parsed.Path, ".git")
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return parsed.String()
}

// ValidProjectID reports whether id is a valid project identifier
// (exactly 12 lowercase hex characters).
func ValidProjectID(id string) bool {
	return projectIDRegexp.MatchString(id)
}
