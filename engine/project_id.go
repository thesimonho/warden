package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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

// ValidProjectID reports whether id is a valid project identifier
// (exactly 12 lowercase hex characters).
func ValidProjectID(id string) bool {
	return projectIDRegexp.MatchString(id)
}
