// Package seccomp provides an embedded seccomp profile for Warden containers.
//
// The profile uses a denylist approach (SCMP_ACT_ALLOW default) to block
// dangerous syscalls while permitting everything a development environment
// needs. This is deliberately lenient — it blocks kernel manipulation,
// filesystem mounting, BPF, and other syscalls that have no legitimate use
// in a containerised coding agent, while leaving standard dev tooling
// (git, npm, curl, iptables, etc.) unrestricted.
//
// Docker's API requires inline JSON in SecurityOpt ("seccomp=<json>"),
// while Podman requires a file path ("seccomp=<path>"). Both forms are
// provided: ProfileJSON() for Docker, WriteProfileFile() for Podman.
package seccomp

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed profile.json
var profileBytes []byte

// ProfileJSON returns the raw seccomp profile JSON as a string.
func ProfileJSON() string {
	return string(profileBytes)
}

// WriteProfileFile writes the embedded seccomp profile to a file in dir
// and returns the absolute path. The file is overwritten on each call to
// ensure it stays in sync with the embedded profile. The container runtime
// reads this file path via SecurityOpt ("seccomp=<path>").
func WriteProfileFile(dir string) (string, error) {
	path := filepath.Join(dir, "seccomp.json")
	if err := os.WriteFile(path, profileBytes, 0o644); err != nil {
		return "", fmt.Errorf("writing seccomp profile: %w", err)
	}
	return path, nil
}

// seccompProfile mirrors the top-level structure of a Docker/OCI seccomp
// profile for validation purposes only.
type seccompProfile struct {
	DefaultAction string   `json:"defaultAction"`
	Architectures []string `json:"architectures"`
	Syscalls      []struct {
		Names  []string `json:"names"`
		Action string   `json:"action"`
	} `json:"syscalls"`
}

// Validate checks that the embedded profile is well-formed JSON with the
// expected structure. Called by tests to catch malformed profiles at build time.
func Validate() error {
	var profile seccompProfile
	if err := json.Unmarshal(profileBytes, &profile); err != nil {
		return fmt.Errorf("parsing seccomp profile: %w", err)
	}
	if profile.DefaultAction == "" {
		return fmt.Errorf("seccomp profile missing defaultAction")
	}
	if len(profile.Architectures) == 0 {
		return fmt.Errorf("seccomp profile missing architectures")
	}
	if len(profile.Syscalls) == 0 {
		return fmt.Errorf("seccomp profile missing syscall rules")
	}
	return nil
}
