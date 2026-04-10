//go:build windows

package service

import (
	"net"

	"github.com/Microsoft/go-winio"
)

// dialHost connects to the host-side socket for the bridge proxy.
// On Windows, this is a named pipe (e.g. \\.\pipe\openssh-ssh-agent).
func dialHost(hostPath string) (net.Conn, error) {
	return winio.DialPipe(hostPath, nil)
}
