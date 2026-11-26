# ProxiLens 2.1.0 Beta 9.2 – Hardened Root Probe + PAM Development Scaffold

Beta 9.2 is a development-only milestone. The root policy probe remains
observation-only and never authorizes an action. PAM work is a design/test
scaffold only: there is no loadable PAM module and no PAM configuration file.

## Root probe hardening

- Reads peer PID/UID/GID using Linux `SO_PEERCRED`.
- Reads `/proc/PID/stat` start time before inspection and checks it again after
  session lookup to detect PID reuse during the request.
- Requires exactly one newline-terminated JSON frame and rejects trailing data.
- Matches the logind session scope as an exact cgroup path component rather
  than a substring.
- Keeps bounded request size, request deadlines, and bounded concurrency.
- Always returns `authorized: false`.

## PAM boundary

See `pam/dev/README.md` and `pam/dev/PAM-INTEGRATION-DESIGN.md`. These files
intentionally do not provide a loadable PAM module or a `/etc/pam.d` snippet.
Do not activate PAM integration in this release.

## Build and tests on the target Ubuntu system

```bash
go test ./internal/policyservice -count=1
go test -race ./internal/policyservice -count=1
go test ./...
go test -race ./...
go build -trimpath -o proxilens-ui .
```

Do not install or start the system service until these checks pass and its
systemd unit and runtime behavior have been reviewed. PAM, Polkit, and sudoers
must remain unchanged.
