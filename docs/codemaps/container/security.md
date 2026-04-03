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
- **restricted**: DNS server IP and resolved domain IPs are whitelisted; all other outbound traffic blocked
- **none**: All outbound traffic blocked except loopback

Wildcard domains (e.g. `*.github.com`) are supported — the base domain is resolved and its IPs are whitelisted.

Note: Domain IPs are resolved once at container start. CDN IP rotation or dynamic IP changes require container restart.

NET_ADMIN capability is added only for `restricted` and `none` modes.
