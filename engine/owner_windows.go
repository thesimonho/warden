package engine

// hostOwner is not supported on Windows. Container runtimes on Windows
// use different ownership semantics — UID/GID passthrough is not applicable.
func hostOwner(_ string) (uid, gid uint32, err error) {
	return 0, 0, nil
}
