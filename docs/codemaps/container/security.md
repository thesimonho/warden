# Container Security

Every Warden container is hardened with three layers of process isolation, applied unconditionally at creation time.

## 1. Capability Dropping

All default Linux capabilities are dropped (`CapDrop: ALL`), then only the minimum required set is re-added:

| Capability       | Why needed                                      | When            |
| ---------------- | ----------------------------------------------- | --------------- |
| CHOWN            | Entrypoint chown of bind mounts                 | Always          |
| DAC_OVERRIDE     | Root reading/writing files owned by warden user | Always          |
| FOWNER           | Entrypoint file ownership operations            | Always          |
| FSETID           | Preserve setuid/setgid bits during chown        | Always          |
| KILL             | Shutdown handler: kill -TERM -1                 | Always          |
| SETUID           | gosu privilege drop (setuid syscall)            | Always          |
| SETGID           | gosu privilege drop (setgid syscall)            | Always          |
| NET_BIND_SERVICE | Dev servers binding to ports < 1024             | Always          |
| NET_RAW          | Ping and network diagnostics                    | Always          |
| SYS_CHROOT       | Some tools (e.g. npm) use chroot                | Always          |
| NET_ADMIN        | iptables for network isolation                  | restricted/none |

Dropped from Docker defaults: SETPCAP, MKNOD, SETFCAP, AUDIT_WRITE.

## 2. Seccomp Profile

A denylist-based seccomp profile (`SCMP_ACT_ALLOW` default) blocks dangerous syscalls while allowing all standard dev tooling. Embedded in the Go binary via `engine/seccomp/profile.json`. Blocked syscall categories:

- **Kernel manipulation**: kexec_load, reboot, init_module, delete_module
- **Filesystem mounting**: mount, umount2, pivot_root, fsopen, fsmount, move_mount, open_tree
- **Security-sensitive**: bpf, perf_event_open, userfaultfd, open_by_handle_at, keyring operations
- **System administration**: acct, swapon, swapoff, syslog, settimeofday

## 3. No New Privileges

The `no-new-privileges` flag prevents privilege escalation via setuid/setgid binaries inside the container. The entrypoint starts as root for privileged setup (UID remapping, iptables), then permanently drops to the `warden` user via `exec gosu`. PID 1 runs as `warden` — no root process remains after startup.

## Network Isolation

Container network modes are passed as environment variables to enforce isolation at container start:

| Mode         | Env Var                          | Behavior                                    |
| ------------ | -------------------------------- | ------------------------------------------- |
| `full`       | `WARDEN_NETWORK_MODE=full`       | Unrestricted internet access (default)      |
| `restricted` | `WARDEN_NETWORK_MODE=restricted` | Outbound traffic limited to allowed domains |
| `none`       | `WARDEN_NETWORK_MODE=none`       | All outbound traffic blocked (air-gapped)   |

For `restricted` mode, allowed domains are passed as `WARDEN_ALLOWED_DOMAINS=domain1.com,domain2.com`. The `setup-network-isolation.sh` script runs in the entrypoint (before user code executes) and configures iptables OUTPUT rules based on the network mode:

- **full**: No rules applied
- **restricted**: dnsmasq + ipset for dynamic domain-based filtering; all other outbound traffic blocked
- **none**: All outbound traffic blocked except loopback

Wildcard domains (e.g. `*.github.com`) are supported — dnsmasq's `/domain/` syntax matches all subdomains and adds resolved IPs to the ipset automatically.

The script supports hot-reload: when re-run via `docker exec` on a container where dnsmasq is already running, it regenerates the dnsmasq config, flushes and re-seeds the ipset, and signals dnsmasq with SIGHUP — without touching iptables rules or resolv.conf. The engine's `ReloadAllowedDomains` method triggers this by running the script as root with env var overrides.

Default allowed domains are agent-scoped (defined in `service/host.go`): Claude Code gets `*.anthropic.com`, Codex gets `*.openai.com` + `*.chatgpt.com`, both share GitHub and package registry domains. Served via `/api/v1/defaults` → `RestrictedDomains`.

NET_ADMIN capability is added only for `restricted` and `none` modes.
