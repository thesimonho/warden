package service

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
)

// socketBridge manages a TCP listener on the host that proxies
// connections to a local Unix domain socket. Used to bridge socket-based
// access items (SSH agent, GPG agent) into containers — this works
// identically on native Docker and Docker Desktop across all platforms.
//
// The container-side counterpart is a socat process started by the
// entrypoint that creates a Unix socket and forwards to this TCP port
// via host.docker.internal.
type socketBridge struct {
	listener      net.Listener
	hostPath      string // Unix socket path on the host
	containerPath string // where the socket appears in the container
	wg            sync.WaitGroup
}

// startSocketBridge starts a TCP listener on 127.0.0.1 (ephemeral port)
// that proxies every accepted connection to the given host socket path.
func startSocketBridge(hostPath, containerPath string) (*socketBridge, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listening for socket bridge: %w", err)
	}

	b := &socketBridge{
		listener:      ln,
		hostPath:      hostPath,
		containerPath: containerPath,
	}

	b.wg.Add(1)
	go b.acceptLoop()

	slog.Info("socket bridge started",
		"port", b.Port(),
		"hostSocket", hostPath,
		"containerSocket", containerPath,
	)

	return b, nil
}

// Port returns the TCP port the bridge is listening on.
func (b *socketBridge) Port() int {
	return b.listener.Addr().(*net.TCPAddr).Port
}

// Close stops the bridge listener and waits for the accept loop and all
// in-flight proxy goroutines to finish.
func (b *socketBridge) Close() {
	_ = b.listener.Close()
	b.wg.Wait()
	slog.Debug("socket bridge stopped",
		"hostSocket", b.hostPath,
		"containerSocket", b.containerPath,
	)
}

// acceptLoop accepts TCP connections and proxies each to the host socket.
func (b *socketBridge) acceptLoop() {
	defer b.wg.Done()

	for {
		conn, err := b.listener.Accept()
		if err != nil {
			if !errors.Is(err, net.ErrClosed) {
				slog.Warn("socket bridge accept error",
					"hostSocket", b.hostPath, "err", err)
			}
			return
		}
		b.wg.Add(1)
		go func() {
			defer b.wg.Done()
			b.proxy(conn)
		}()
	}
}

// stopSocketBridges stops and removes all bridges for the given container.
func (s *Service) stopSocketBridges(containerName string) {
	s.socketBridgesMu.Lock()
	bridges := s.socketBridges[containerName]
	delete(s.socketBridges, containerName)
	s.socketBridgesMu.Unlock()

	for _, b := range bridges {
		b.Close()
	}
}

// stopAllSocketBridges stops all active bridges. Called on shutdown.
func (s *Service) stopAllSocketBridges() {
	s.socketBridgesMu.Lock()
	all := s.socketBridges
	s.socketBridges = make(map[string][]*socketBridge)
	s.socketBridgesMu.Unlock()

	for _, bridges := range all {
		for _, b := range bridges {
			b.Close()
		}
	}
}

// proxy connects a TCP client to the host Unix socket and copies data
// bidirectionally until either side closes.
func (b *socketBridge) proxy(tcpConn net.Conn) {
	defer func() { _ = tcpConn.Close() }()

	unixConn, err := net.Dial("unix", b.hostPath)
	if err != nil {
		slog.Debug("socket bridge: failed to connect to host socket",
			"socket", b.hostPath, "err", err)
		return
	}
	defer func() { _ = unixConn.Close() }()

	var copyWg sync.WaitGroup
	copyWg.Add(2)

	go func() {
		defer copyWg.Done()
		_, _ = io.Copy(unixConn, tcpConn)
	}()

	go func() {
		defer copyWg.Done()
		_, _ = io.Copy(tcpConn, unixConn)
	}()

	copyWg.Wait()
}
