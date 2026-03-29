//go:build !windows

package engine

import (
	"fmt"
	"os"
	"syscall"
)

// hostOwner returns the UID and GID of the owner of the given path.
// Used to pass the host user's identity to the container entrypoint
// so it can match file ownership without probing bind mounts at runtime.
func hostOwner(path string) (uid, gid uint32, err error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, 0, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0, fmt.Errorf("unsupported platform: cannot read file ownership")
	}
	return stat.Uid, stat.Gid, nil
}
