// Package version provides build-time version information and update
// checking against GitHub releases.
package version

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// Version is the current build version, set at build time via ldflags:
//
//	-X github.com/thesimonho/warden/version.Version=v0.5.2
//
// Defaults to "dev" for local development builds.
var Version = "dev"

// releaseURL is the GitHub API endpoint for the latest release.
// Exported as a var so tests can override it.
var releaseURL = "https://api.github.com/repos/thesimonho/warden/releases/latest"

// checkTimeout is the HTTP client timeout for update checks.
const checkTimeout = 10 * time.Second

// githubRelease is the subset of the GitHub release API response we need.
type githubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

// CheckLatest queries the GitHub releases API for the latest release.
// Returns the latest version tag, the release page URL, and any error.
// Network failures return a non-nil error; the caller decides how to handle it.
func CheckLatest(ctx context.Context) (latest string, pageURL string, err error) {
	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releaseURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("fetching latest release: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("unexpected status %d from GitHub API", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", "", fmt.Errorf("decoding release response: %w", err)
	}

	return release.TagName, release.HTMLURL, nil
}

// IsNewer reports whether latest is a higher semantic version than current.
// Both values may optionally have a "v" prefix. Returns false if either
// value cannot be parsed as semver.
func IsNewer(current, latest string) bool {
	curParts, curOK := parseSemver(current)
	latParts, latOK := parseSemver(latest)
	if !curOK || !latOK {
		return false
	}

	for i := range 3 {
		if latParts[i] > curParts[i] {
			return true
		}
		if latParts[i] < curParts[i] {
			return false
		}
	}

	return false
}

// CheckAndPrint checks for a newer release and prints a colored message
// to stderr if one is available. Intended to be called as a goroutine
// during startup. Silently returns when the version is "dev", the
// WARDEN_NO_UPDATE_CHECK env var is set, or on any network error.
func CheckAndPrint() {
	if Version == "dev" || os.Getenv("WARDEN_NO_UPDATE_CHECK") == "1" {
		return
	}

	latest, pageURL, err := CheckLatest(context.Background())
	if err != nil {
		slog.Debug("update check failed", "err", err)
		return
	}

	if !IsNewer(Version, latest) {
		return
	}

	const yellow = "\033[33m"
	const reset = "\033[0m"
	fmt.Fprintf(os.Stderr, "  %sUpdate available: %s → %s — %s%s\n", yellow, Version, latest, pageURL, reset)
}

// parseSemver extracts major, minor, patch from a version string.
// Accepts optional "v" prefix. Returns false if parsing fails.
func parseSemver(v string) ([3]int, bool) {
	v = strings.TrimPrefix(v, "v")
	parts := strings.SplitN(v, ".", 3)
	if len(parts) != 3 {
		return [3]int{}, false
	}

	var result [3]int
	for i, p := range parts {
		// Strip any pre-release suffix (e.g. "2-beta.1" → "2").
		if idx := strings.IndexByte(p, '-'); idx >= 0 {
			p = p[:idx]
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return [3]int{}, false
		}
		result[i] = n
	}

	return result, true
}
