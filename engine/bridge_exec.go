package engine

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strconv"

	"github.com/docker/docker/api/types/container"

	"github.com/thesimonho/warden/constants"
)

// AllowBridgePortInContainer adds an iptables rule inside the container
// that allows outbound TCP connections to the Docker gateway on the
// given port. Required for containers with restricted/none network
// modes where outbound traffic is blocked by default — without this
// rule, socat cannot reach the host TCP bridge via host.docker.internal.
//
// The rule is inserted at position 2 in the OUTPUT chain (after the
// loopback ACCEPT rule) so it takes precedence over the REJECT rule.
// No-op if the container has no iptables rules (full network mode).
func (ec *EngineClient) AllowBridgePortInContainer(ctx context.Context, containerID string, port int) error {
	// Resolve the IP that host.docker.internal points to inside the
	// container. On Docker Desktop this is the VM gateway (e.g.
	// 192.168.65.254), which differs from the container's default
	// route gateway (172.17.0.1). On native Docker it matches the
	// default route. Using the resolved IP ensures the rule matches
	// the actual destination socat connects to.
	script := fmt.Sprintf(
		"HOST_IP=$(getent ahostsv4 host.docker.internal | head -1 | awk '{print $1}') && "+
			"[ -n \"$HOST_IP\" ] && "+
			"iptables -C OUTPUT -d $HOST_IP -p tcp --dport %d -j ACCEPT 2>/dev/null || "+
			"iptables -I OUTPUT 2 -d $HOST_IP -p tcp --dport %d -j ACCEPT 2>/dev/null || true",
		port, port,
	)

	_, err := ec.execAndCapture(ctx, containerID, container.ExecOptions{
		Cmd:          []string{"sh", "-c", script},
		User:         "root",
		Privileged:   true,
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		slog.Debug("failed to allow bridge port in container (may be full network mode)",
			"container", containerID[:12], "port", port, "err", err)
	}
	return nil
}

// ExecSocatBridge starts a socat process inside the container that
// creates a Unix socket at containerPath and forwards connections to
// the host TCP bridge via host.docker.internal:port. Runs as the
// warden user in detached mode so it survives the exec returning.
func (ec *EngineClient) ExecSocatBridge(ctx context.Context, containerID, containerPath string, port int) error {
	parentDir := filepath.Dir(containerPath)
	portStr := strconv.Itoa(port)

	// Create parent dir, remove stale socket, then exec socat.
	// Using exec (not nohup) since the detached docker exec already
	// runs the process independently of any TTY.
	script := fmt.Sprintf(
		"mkdir -p %s && chmod 700 %s 2>/dev/null; rm -f %s; "+
			"exec socat UNIX-LISTEN:%s,fork,mode=600 TCP:host.docker.internal:%s",
		parentDir, parentDir, containerPath,
		containerPath, portStr,
	)

	resp, err := ec.api.ContainerExecCreate(ctx, containerID, container.ExecOptions{
		Cmd:    []string{"sh", "-c", script},
		User:   constants.ContainerUser,
		Detach: true,
	})
	if err != nil {
		return fmt.Errorf("creating socat exec: %w", err)
	}

	if err := ec.api.ContainerExecStart(ctx, resp.ID, container.ExecStartOptions{Detach: true}); err != nil {
		return fmt.Errorf("starting socat exec: %w", err)
	}

	slog.Debug("exec'd socat bridge into container",
		"container", containerID[:12],
		"containerPath", containerPath,
		"port", port,
	)
	return nil
}

// KillSocatBridges kills all socat bridge processes inside the
// container and removes stale iptables rules for old bridge ports.
// Called before re-exec'ing bridges with new ports (e.g. after server
// restart) to avoid stale connections and leftover firewall rules.
func (ec *EngineClient) KillSocatBridges(ctx context.Context, containerID string) error {
	// Kill socat processes (unprivileged).
	_, err := ec.execAndCapture(ctx, containerID, container.ExecOptions{
		Cmd:          []string{"sh", "-c", "pkill -f 'socat.*host.docker.internal' 2>/dev/null || true"},
		User:         constants.ContainerUser,
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		slog.Debug("failed to kill socat bridges (container may be stopping)",
			"container", containerID[:12], "err", err)
	}

	// Remove stale per-port iptables rules for old bridge ports
	// (privileged). Uses the resolved host.docker.internal IP to
	// match the rules added by AllowBridgePortInContainer. Loops
	// until no matching rule remains.
	_, err = ec.execAndCapture(ctx, containerID, container.ExecOptions{
		Cmd: []string{"sh", "-c",
			"HOST_IP=$(getent ahostsv4 host.docker.internal | head -1 | awk '{print $1}') && " +
				"[ -n \"$HOST_IP\" ] && " +
				"while iptables -D OUTPUT -d $HOST_IP -p tcp -j ACCEPT 2>/dev/null; do true; done; true",
		},
		User:         "root",
		Privileged:   true,
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		slog.Debug("failed to clean up stale bridge iptables rules",
			"container", containerID[:12], "err", err)
	}

	return nil
}
