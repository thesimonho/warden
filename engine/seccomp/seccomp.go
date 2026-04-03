// Package seccomp provides an embedded seccomp profile for Warden containers.
//
// The profile uses a denylist approach (SCMP_ACT_ALLOW default) to block
// dangerous syscalls while permitting everything a development environment
// needs. This is deliberately lenient — it blocks kernel manipulation,
// filesystem mounting, BPF, and other syscalls that have no legitimate use
// in a containerised coding agent, while leaving standard dev tooling
// (git, npm, curl, iptables, etc.) unrestricted.
//
// Docker's API applies the profile via SecurityOpt ("seccomp=<json>").
package seccomp

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed profile.json
var profileBytes []byte

// ProfileJSON returns the raw seccomp profile JSON as a string.
func ProfileJSON() string {
	return string(profileBytes)
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
