package engine

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
)

// bridgeFirewallChain is the iptables chain used to manage per-port
// ACCEPT rules for socket bridge TCP listeners. Using a dedicated chain
// keeps Warden rules isolated from the host firewall and enables
// atomic cleanup (flush + delete) on shutdown or crash recovery.
const bridgeFirewallChain = "WARDEN-BRIDGE"

// firewallImage is the container image used for host-network iptables
// operations. The default Warden image already has iptables installed
// (used for in-container network isolation).
const firewallImage = defaultImage

// SetupBridgeFirewall creates the WARDEN-BRIDGE iptables chain on the
// host and adds a jump rule from INPUT. Idempotent — safe to call on
// every startup. Flushes stale rules from a previous server run.
//
// Only runs on native Docker (not Docker Desktop). On Desktop, the VM's
// NAT handles container-to-host forwarding without iptables rules.
func (ec *EngineClient) SetupBridgeFirewall(ctx context.Context) error {
	if ec.isDesktop {
		return nil
	}

	// Create chain (ignore error if already exists), add jump rule
	// (idempotent via -C check), then flush any stale per-port rules.
	script := fmt.Sprintf(
		"iptables -N %s 2>/dev/null; "+
			"iptables -C INPUT -j %s 2>/dev/null || iptables -I INPUT -j %s; "+
			"iptables -F %s",
		bridgeFirewallChain,
		bridgeFirewallChain, bridgeFirewallChain,
		bridgeFirewallChain,
	)

	if err := ec.runHostIptables(ctx, script); err != nil {
		return fmt.Errorf("setting up bridge firewall chain: %w", err)
	}

	slog.Info("bridge firewall chain ready", "chain", bridgeFirewallChain)
	return nil
}

// AddBridgeFirewallRule adds an iptables ACCEPT rule for the given TCP
// port in the WARDEN-BRIDGE chain. This allows containers on the docker0
// bridge to reach the socket bridge TCP listener on the host.
func (ec *EngineClient) AddBridgeFirewallRule(ctx context.Context, port int) error {
	return ec.AddBridgeFirewallRules(ctx, []int{port})
}

// AddBridgeFirewallRules adds iptables ACCEPT rules for multiple TCP
// ports in a single container execution. Batching avoids the overhead
// of one Docker container per port.
func (ec *EngineClient) AddBridgeFirewallRules(ctx context.Context, ports []int) error {
	if ec.isDesktop || len(ports) == 0 {
		return nil
	}

	var cmds []string
	for _, port := range ports {
		cmds = append(cmds, fmt.Sprintf(
			"iptables -A %s -p tcp --dport %d -j ACCEPT",
			bridgeFirewallChain, port,
		))
	}

	if err := ec.runHostIptables(ctx, strings.Join(cmds, "; ")); err != nil {
		return fmt.Errorf("adding bridge firewall rules for %d ports: %w", len(ports), err)
	}

	slog.Debug("added bridge firewall rules", "ports", ports)
	return nil
}

// RemoveBridgeFirewallRule removes the iptables ACCEPT rule for the
// given TCP port from the WARDEN-BRIDGE chain. Called when a socket
// bridge is stopped (container delete, recreate, or shutdown).
func (ec *EngineClient) RemoveBridgeFirewallRule(ctx context.Context, port int) error {
	if ec.isDesktop {
		return nil
	}

	cmd := fmt.Sprintf(
		"iptables -D %s -p tcp --dport %d -j ACCEPT 2>/dev/null || true",
		bridgeFirewallChain, port,
	)

	if err := ec.runHostIptables(ctx, cmd); err != nil {
		slog.Debug("failed to remove bridge firewall rule (may already be gone)",
			"port", port, "err", err)
		return nil
	}

	slog.Debug("removed bridge firewall rule", "port", port)
	return nil
}

// TeardownBridgeFirewall removes the WARDEN-BRIDGE chain and its jump
// rule from INPUT. Called on graceful server shutdown. Errors are logged
// but not returned since this runs during cleanup.
func (ec *EngineClient) TeardownBridgeFirewall(ctx context.Context) error {
	if ec.isDesktop {
		return nil
	}

	script := fmt.Sprintf(
		"iptables -F %s 2>/dev/null; "+
			"iptables -D INPUT -j %s 2>/dev/null; "+
			"iptables -X %s 2>/dev/null; "+
			"true",
		bridgeFirewallChain,
		bridgeFirewallChain,
		bridgeFirewallChain,
	)

	if err := ec.runHostIptables(ctx, script); err != nil {
		slog.Warn("failed to tear down bridge firewall chain", "err", err)
		return nil
	}

	slog.Info("bridge firewall chain removed")
	return nil
}

// runHostIptables runs a shell command inside a short-lived container
// with host networking and NET_ADMIN capability. This modifies the
// HOST's iptables rules (not the container's).
func (ec *EngineClient) runHostIptables(ctx context.Context, shellCmd string) error {
	resp, err := ec.api.ContainerCreate(ctx,
		&container.Config{
			Image:      firewallImage,
			Entrypoint: []string{"sh"},
			Cmd:        []string{"-c", shellCmd},
		},
		&container.HostConfig{
			AutoRemove:  true,
			NetworkMode: "host",
			CapAdd:      []string{"NET_ADMIN"},
		},
		&network.NetworkingConfig{},
		nil,
		"",
	)
	if err != nil {
		return fmt.Errorf("creating firewall container: %w", err)
	}

	if err := ec.api.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("starting firewall container: %w", err)
	}

	waitCh, errCh := ec.api.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case result := <-waitCh:
		if result.StatusCode != 0 {
			return fmt.Errorf("firewall command exited with status %d", result.StatusCode)
		}
		return nil
	case err := <-errCh:
		return fmt.Errorf("waiting for firewall container: %w", err)
	case <-ctx.Done():
		return ctx.Err()
	}
}
