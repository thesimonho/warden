package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"

	"github.com/docker/docker/pkg/stdcopy"

	"github.com/thesimonho/warden/engine"
)

// portBridge proxies TCP connections from a host listener into a
// container port via docker exec. Used on Docker Desktop where the
// container's bridge IP is unreachable from the host.
//
// Each accepted TCP connection spawns a docker exec running
// socat STDIO TCP:127.0.0.1:<port>, then copies data bidirectionally
// between the TCP connection and the exec's hijacked stream.
type portBridge struct {
	listener    net.Listener
	docker      engine.Client
	containerID string
	port        int
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

// startPortBridge creates a TCP listener on 127.0.0.1 (ephemeral port)
// that proxies connections into the container via docker exec.
func startPortBridge(docker engine.Client, containerID string, port int) (*portBridge, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listening for port bridge: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	b := &portBridge{
		listener:    ln,
		docker:      docker,
		containerID: containerID,
		port:        port,
		ctx:         ctx,
		cancel:      cancel,
	}

	b.wg.Add(1)
	go b.acceptLoop()

	slog.Info("port bridge started",
		"listenAddr", ln.Addr().String(),
		"container", containerID[:12],
		"containerPort", port,
	)

	return b, nil
}

// ListenPort returns the host-side TCP port the bridge is listening on.
func (b *portBridge) ListenPort() int {
	return b.listener.Addr().(*net.TCPAddr).Port
}

// Close cancels in-flight exec processes, stops the listener, and
// waits for connections to drain.
func (b *portBridge) Close() {
	b.cancel()
	_ = b.listener.Close()
	b.wg.Wait()
	slog.Debug("port bridge stopped",
		"container", b.containerID[:12],
		"containerPort", b.port,
	)
}

// acceptLoop accepts connections and proxies each into the container.
func (b *portBridge) acceptLoop() {
	defer b.wg.Done()

	for {
		conn, err := b.listener.Accept()
		if err != nil {
			if !errors.Is(err, net.ErrClosed) {
				slog.Warn("port bridge accept error",
					"container", b.containerID[:12],
					"port", b.port,
					"err", err,
				)
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

// proxy connects a TCP client to the container port via docker exec.
// Writes go directly to exec stdin; reads are demuxed from the
// docker multiplexed stream via stdcopy.
func (b *portBridge) proxy(tcpConn net.Conn) {
	defer func() { _ = tcpConn.Close() }()

	stream, err := b.docker.ExecPortForward(b.ctx, b.containerID, b.port)
	if err != nil {
		slog.Debug("port bridge: exec failed",
			"container", b.containerID[:12],
			"port", b.port,
			"err", err,
		)
		return
	}
	defer stream.Close()

	var copyWg sync.WaitGroup
	copyWg.Add(2)

	// TCP → exec stdin (raw, no framing).
	go func() {
		defer copyWg.Done()
		_, _ = io.Copy(stream.Conn, tcpConn)
		closeWrite(stream.Conn)
	}()

	// Exec stdout → TCP (demux docker's multiplexed stream).
	go func() {
		defer copyWg.Done()
		_, _ = stdcopy.StdCopy(tcpConn, io.Discard, stream.Reader)
		closeWrite(tcpConn)
	}()

	copyWg.Wait()
}

// portBridgeKey identifies a port bridge by container and port.
type portBridgeKey struct {
	containerName string
	port          int
}

// getOrStartPortBridge returns an existing port bridge or starts a new
// one for the given container and port. Thread-safe.
func (s *Service) getOrStartPortBridge(containerID, containerName string, port int) (*portBridge, error) {
	key := portBridgeKey{containerName: containerName, port: port}

	s.portBridgesMu.Lock()
	defer s.portBridgesMu.Unlock()

	if b, ok := s.portBridges[key]; ok {
		return b, nil
	}

	b, err := startPortBridge(s.docker, containerID, port)
	if err != nil {
		return nil, err
	}

	s.portBridges[key] = b
	return b, nil
}

// stopPortBridges stops all port bridges for the given container.
func (s *Service) stopPortBridges(containerName string) {
	s.portBridgesMu.Lock()
	var toStop []*portBridge
	for key, b := range s.portBridges {
		if key.containerName == containerName {
			toStop = append(toStop, b)
			delete(s.portBridges, key)
		}
	}
	s.portBridgesMu.Unlock()

	for _, b := range toStop {
		b.Close()
	}
}

// stopAllPortBridges stops all active port bridges. Called on shutdown.
func (s *Service) stopAllPortBridges() {
	s.portBridgesMu.Lock()
	all := s.portBridges
	s.portBridges = make(map[portBridgeKey]*portBridge)
	s.portBridgesMu.Unlock()

	for _, b := range all {
		b.Close()
	}
}
