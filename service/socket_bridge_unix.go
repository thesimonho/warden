//go:build !windows

package service

import "net"

// dialHost connects to the host-side socket for the bridge proxy.
// On Unix platforms, this is a Unix domain socket.
func dialHost(hostPath string) (net.Conn, error) {
	return net.Dial("unix", hostPath)
}
