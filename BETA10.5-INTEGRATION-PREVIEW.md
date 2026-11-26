# ProxiLens 2.1.0 Beta 10.5 — Authentication integration preview

## Scope

Beta 10.5 adds a tested orchestration boundary for Duo Push, a manually invoked
local-password/PAM adapter, and a read-only current-session inspector. It is a
development integration preview, not a system authorization mechanism.

## User flow

1. Duo Push is the primary authentication route.
2. A Duo failure/unavailable result makes the separate manual-password action
   eligible. The coordinator never invokes password authentication itself.
3. The password adapter is called only through the explicit
   `ManualPasswordFallback` entry point. The future adapter must prompt through
   a trusted PAM conversation and must not accept credentials from the GUI,
   environment, command line, or IPC payload.
4. The session inspector is diagnostic only. The existing policy probe reports
   `Authorized=false` and does not create a grant.

## Security boundary

`approval.Result.Authorized` is always false. `ErrNotAuthorizing` is returned
for a successful authentication/inspection outcome to make the non-granting
boundary explicit to callers. No PAM adapter is enabled by this package and no
production PAM, sudoers, Polkit, or systemd configuration is changed.

A future privileged service must derive peer identity from kernel-provided
Unix-socket credentials (for example `SO_PEERCRED`) and validate the logind
session/cgroup association. Environment variables such as `SUDO_UID` and
`SUDO_USER` are not identity proofs. A real grant protocol requires a separate
security review, replay protection, action/resource binding, expiry, audit, and
revocation before any privileged action can be considered.

## Validation

Run:

```sh
go test -race ./...
go build -trimpath -o proxilens-ui .
```

The new tests verify that Duo failure never triggers password fallback, that
fallback is unavailable before the failed Duo attempt, and that a successful
flow never reports authorization.
