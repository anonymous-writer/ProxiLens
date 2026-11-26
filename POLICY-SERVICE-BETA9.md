# ProxiLens Beta 9 – Root Policy Probe (observation only)

## Purpose

Beta 9 adds an isolated Linux root service that observes the connecting process identity. It is a diagnostic milestone, **not an authorization service**.

## What it does

- Runs as root when explicitly started with `--policy-probe-service`.
- Listens on `/run/proxilens-policy/probe.sock`.
- Reads peer PID/UID/GID from Linux `SO_PEERCRED`, not from JSON claims.
- Uses `loginctl` and the peer process cgroup to try to match a logind session.
- Accepts only the fixed `inspect-session` operation and a bounded line of JSON.
- Returns `authorized: false` unconditionally.
- Performs no `sudo`, Polkit, PAM, Duo, shell, or privileged action on behalf of the caller.

The socket is mode `0666` because this is an observation-only probe and must be callable by the logged-in desktop user. The endpoint exposes only the caller's own kernel-supplied credentials and session match. It must not be extended to accept arbitrary commands or authorization claims in this release.

## Build and local checks

The project declares Go 1.27 in `go.mod`. Build on the target Ubuntu system with its installed Go toolchain:

```bash
go test ./...
go test -race ./...
go build -trimpath -o proxilens-ui .
```

## Install the probe service manually (only after successful build)

This step installs only a new diagnostic systemd unit. It does not alter PAM, sudoers, or Polkit.

```bash
sudo install -m 0755 proxilens-ui /usr/bin/proxilens-ui
sudo install -m 0644 packaging/systemd/system/proxilens-policy-probe.service /etc/systemd/system/proxilens-policy-probe.service
sudo systemctl daemon-reload
sudo systemctl enable --now proxilens-policy-probe.service
```

Query it as the desktop user:

```bash
./proxilens-ui --policy-probe
```

Expected response includes `"mode":"observation-only"` and **always** `"authorized":false`. A session match is diagnostic only.

## Stop and remove

```bash
sudo systemctl disable --now proxilens-policy-probe.service
sudo rm -f /etc/systemd/system/proxilens-policy-probe.service /usr/bin/proxilens-ui
sudo systemctl daemon-reload
sudo rm -f /run/proxilens-policy/probe.sock
sudo rmdir /run/proxilens-policy 2>/dev/null || true
```

If `/usr/bin/proxilens-ui` is already used by another installation, do not overwrite it; instead adjust `ExecStart` to the built binary path after reviewing the unit.

## Explicit security boundary

No real root grants, 15-minute grants, Duo verification, password fallback, or cross-sudo/Polkit authorization are implemented. Do not connect this probe to PAM, Polkit rules, sudoers, or an authorization decision. Before any such work, the protocol needs a separate threat model, robust logind/D-Bus session identity verification, bounded concurrency, service lifecycle handling, action-specific policy evaluation, audit design, and independent recovery testing.
