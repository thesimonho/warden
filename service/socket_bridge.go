package service

import (
	"context"
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
// The listener binds to the Docker bridge gateway IP (native Docker) or
// 127.0.0.1 (Docker Desktop, where VM NAT handles forwarding). Containers
// connect via host.docker.internal which resolves to the correct address.
//
// On native Docker, a per-port iptables ACCEPT rule is added to the
// WARDEN-BRIDGE chain so container traffic can reach the host listener
// through firewalls with INPUT DROP policy.
//
// The container-side counterpart is a socat process started via docker
// exec that creates a Unix socket and forwards to this TCP port via
// host.docker.internal.
type socketBridge struct {
	listener      net.Listener
	hostPath      string // Unix socket path on the host
	containerPath string // where the socket appears in the container
	wg            sync.WaitGroup
}

// startSocketBridge starts a TCP listener on the given bridge IP
// (ephemeral port) that proxies connections to the host socket.
func startSocketBridge(bridgeIP, hostPath, containerPath string) (*socketBridge, error) {
	ln, err := net.Listen("tcp", bridgeIP+":0")
	if err != nil {
		return nil, fmt.Errorf("listening for socket bridge on %s: %w", bridgeIP, err)
	}

	b := &socketBridge{
		listener:      ln,
		hostPath:      hostPath,
		containerPath: containerPath,
	}

	b.wg.Add(1)
	go b.acceptLoop()

	slog.Info("socket bridge started",
		"listenAddr", ln.Addr().String(),
		"hostSocket", hostPath,
		"containerSocket", containerPath,
	)

	return b, nil
}

// Port returns the TCP port the bridge is listening on.
func (b *socketBridge) Port() int {
	return b.listener.Addr().(*net.TCPAddr).Port
}

// Close stops the bridge listener and waits for all in-flight proxy
// goroutines to finish.
func (b *socketBridge) Close() {
	_ = b.listener.Close()
	b.wg.Wait()
	slog.Debug("socket bridge stopped",
		"hostSocket", b.hostPath,
		"containerSocket", b.containerPath,
	)
}

// acceptLoop accepts connections and proxies each to the host socket.
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

// startBridgeWithFirewall starts a TCP bridge and adds the corresponding
// firewall rule. Returns nil if the bridge or firewall rule fails.
func (s *Service) startBridgeWithFirewall(ctx context.Context, hostPath, containerPath string) *socketBridge {
	bridge, err := startSocketBridge(s.bridgeIP, hostPath, containerPath)
	if err != nil {
		slog.Warn("failed to start socket bridge, skipping",
			"hostSocket", hostPath,
			"containerSocket", containerPath,
			"err", err,
		)
		return nil
	}
	if err := s.docker.AddBridgeFirewallRule(ctx, bridge.Port()); err != nil {
		slog.Warn("failed to add firewall rule for bridge, skipping",
			"port", bridge.Port(), "err", err)
		bridge.Close()
		return nil
	}
	return bridge
}

// stopBridge stops a single bridge and removes its firewall rule.
func (s *Service) stopBridge(b *socketBridge) {
	port := b.Port()
	b.Close()
	if err := s.docker.RemoveBridgeFirewallRule(context.Background(), port); err != nil {
		slog.Debug("failed to remove firewall rule on bridge stop",
			"port", port, "err", err)
	}
}

// stopSocketBridges stops and removes all bridges for the given
// container, including their firewall rules on native Docker.
func (s *Service) stopSocketBridges(containerName string) {
	s.socketBridgesMu.Lock()
	bridges := s.socketBridges[containerName]
	delete(s.socketBridges, containerName)
	s.socketBridgesMu.Unlock()

	for _, b := range bridges {
		s.stopBridge(b)
	}
}

// stopAllSocketBridges stops all active bridges and kills socat
// processes inside their containers. Called on shutdown.
func (s *Service) stopAllSocketBridges() {
	s.socketBridgesMu.Lock()
	all := s.socketBridges
	s.socketBridges = make(map[string][]*socketBridge)
	s.socketBridgesMu.Unlock()

	ctx := context.Background()
	for containerName, bridges := range all {
		// Kill socat processes inside the container so they don't
		// linger after the TCP bridge listeners are closed.
		if row, err := s.db.GetProjectByContainerName(containerName); err == nil && row != nil {
			_ = s.docker.KillSocatBridges(ctx, row.ContainerID)
		}
		for _, b := range bridges {
			s.stopBridge(b)
		}
	}
}

// execSocatBridges starts socat processes inside the container for each
// active bridge. Each socat creates a Unix socket at the bridge's
// container path and forwards connections to the host TCP bridge port
// via host.docker.internal.
//
// For containers with restricted/none network modes, an iptables rule
// is added inside the container to allow outbound TCP to the gateway
// on each bridge port (otherwise the network isolation rejects it).
func (s *Service) execSocatBridges(ctx context.Context, containerID string, bridges []*socketBridge) {
	for _, b := range bridges {
		// Allow the bridge port through the container's network isolation.
		// No-op for full network mode (no iptables rules inside container).
		_ = s.docker.AllowBridgePortInContainer(ctx, containerID, b.Port())

		if err := s.docker.ExecSocatBridge(ctx, containerID, b.containerPath, b.Port()); err != nil {
			slog.Warn("failed to exec socat bridge into container",
				"container", containerID[:12],
				"containerPath", b.containerPath,
				"err", err,
			)
		}
	}
}

// proxy connects a TCP client to the host socket and copies data
// bidirectionally. When one direction's copy finishes (EOF or error),
// it half-closes the write side of the opposite connection so the
// other goroutine's Read unblocks promptly. Without half-close, proxy
// goroutines would hang until the connection is closed externally.
func (b *socketBridge) proxy(tcpConn net.Conn) {
	defer func() { _ = tcpConn.Close() }()

	hostConn, err := dialHost(b.hostPath)
	if err != nil {
		slog.Debug("socket bridge: failed to connect to host socket",
			"socket", b.hostPath, "err", err)
		return
	}
	defer func() { _ = hostConn.Close() }()

	var copyWg sync.WaitGroup
	copyWg.Add(2)

	go func() {
		defer copyWg.Done()
		_, _ = io.Copy(hostConn, tcpConn)
		closeWrite(hostConn)
	}()

	go func() {
		defer copyWg.Done()
		_, _ = io.Copy(tcpConn, hostConn)
		closeWrite(tcpConn)
	}()

	copyWg.Wait()
}

// closeWrite performs a half-close on connections that support it
// (TCP and Unix sockets). This signals EOF to the reader on the
// other end without tearing down the full connection.
func closeWrite(c net.Conn) {
	type writeCloser interface {
		CloseWrite() error
	}
	if wc, ok := c.(writeCloser); ok {
		_ = wc.CloseWrite()
	}
}
