# Container Security

Every Warden container is hardened with three layers of process isolation, applied unconditionally at creation time.

## 1. Capability Dropping

All default Linux capabilities are dropped (`CapDrop: ALL`), then only the minimum required set is re-added:

| Capability       | Why needed                                      | When   |
| ---------------- | ----------------------------------------------- | ------ |
| CHOWN            | Entrypoint chown of bind mounts                 | Always |
| DAC_OVERRIDE     | Root reading/writing files owned by warden user | Always |
| FOWNER           | Entrypoint file ownership operations            | Always |
| FSETID           | Preserve setuid/setgid bits during chown        | Always |
| KILL             | Shutdown handler: kill -TERM -1                 | Always |
| SETUID           | gosu privilege drop and sudo elevation          | Always |
| SETGID           | gosu privilege drop and sudo elevation          | Always |
| NET_BIND_SERVICE | Dev servers binding to ports < 1024             | Always |
| NET_RAW          | Ping and network diagnostics                    | Always |
| SYS_CHROOT       | Some tools (e.g. npm) use chroot                | Always |

Dropped from Docker defaults: SETPCAP, MKNOD, SETFCAP, AUDIT_WRITE.

Notably absent: **NET_ADMIN**. Network isolation (iptables) is applied externally via privileged docker exec, keeping it out of the container's capability bounding set. See "External Network Isolation" below.

## 2. Seccomp Profile

A denylist-based seccomp profile (`SCMP_ACT_ALLOW` default) blocks dangerous syscalls while allowing all standard dev tooling. Embedded in the Go binary via `engine/seccomp/profile.json`. Blocked syscall categories:

- **Kernel manipulation**: kexec_load, reboot, init_module, delete_module
- **Filesystem mounting**: mount, umount2, pivot_root, fsopen, fsmount, move_mount, open_tree
- **Security-sensitive**: bpf, perf_event_open, userfaultfd, open_by_handle_at, keyring operations
- **System administration**: acct, swapon, swapoff, syslog, settimeofday

## 3. Sudo Support

The warden user has passwordless sudo (`NOPASSWD:ALL`) for package installation (e.g. `sudo apt-get install`). This is safe because the capability bounding set is tight:

- No NET_ADMIN → cannot modify iptables rules (network isolation is tamper-proof)
- No SYS_ADMIN → cannot mount/unmount filesystems or escape the container
- No MKNOD → cannot create device nodes
- No SETPCAP/SETFCAP → cannot modify capability sets

The `no-new-privileges` security option is intentionally not set, because sudo requires the SUID bit to function. The tight bounding set limits what root-via-sudo can do, making this a safe trade-off.

## External Network Isolation

Network isolation is enforced **from outside the container** via `docker exec --privileged`. This is the key security property: the container never has NET_ADMIN in its capability bounding set, so even root inside the container cannot modify iptables rules.

### How it works

1. Container is created without NET_ADMIN capability.
2. After `ContainerStart`, the Go server runs `setup-network-isolation.sh` via `docker exec --privileged` (Docker grants ALL capabilities to privileged exec processes regardless of the container's bounding set).
3. The script sets up iptables rules, dnsmasq, and ipset — same mechanism as before, just invoked externally.
4. The user-entrypoint waits for a `/tmp/warden-network-ready` marker before allowing the agent to start. This prevents any network-dependent work before isolation is active.

### Container restart handling

Iptables rules don't persist across container restarts. The Go server re-applies them via two mechanisms:

- **Explicit restarts** (via API): `RestartProject` re-runs the privileged exec after the restart completes.
- **Auto-restarts** (Docker `unless-stopped` policy): A Docker events watcher subscribes to container start events filtered by the `dev.warden.managed` label. On each start event, it looks up the project's network mode from the DB and re-applies isolation.

The readiness gate in `user-entrypoint.sh` ensures the agent cannot start until network isolation is confirmed, even during restarts.

### Hot-reload

Allowed domains can be updated on a running container without restarting it. `ReloadAllowedDomains` uses the same privileged exec mechanism to re-run the script with updated env vars. dnsmasq is signaled with SIGHUP to reload its config.

## Network Modes

| Mode         | Env Var                          | Behavior                                    |
| ------------ | -------------------------------- | ------------------------------------------- |
| `full`       | `WARDEN_NETWORK_MODE=full`       | Unrestricted internet access (default)      |
| `restricted` | `WARDEN_NETWORK_MODE=restricted` | Outbound traffic limited to allowed domains |
| `none`       | `WARDEN_NETWORK_MODE=none`       | All outbound traffic blocked (air-gapped)   |

For `restricted` mode, allowed domains are passed as `WARDEN_ALLOWED_DOMAINS=domain1.com,domain2.com`. The `setup-network-isolation.sh` script configures iptables OUTPUT rules based on the network mode:

- **full**: No rules applied
- **restricted**: dnsmasq + ipset for dynamic domain-based filtering; all other outbound traffic blocked
- **none**: All outbound traffic blocked except loopback

Wildcard domains (e.g. `*.github.com`) are supported — dnsmasq's `/domain/` syntax matches all subdomains and adds resolved IPs to the ipset automatically.

Default allowed domains are agent-scoped (defined in `service/host.go`): Claude Code gets `*.anthropic.com`, Codex gets `*.openai.com` + `*.chatgpt.com`, both share GitHub and package registry domains. Served via `/api/v1/defaults` → `RestrictedDomains`.
