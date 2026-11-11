# ProxiLens design

## Current implementation

The Fyne desktop UI reads the current user's systemd-logind session via `loginctl`. It reports the session identifier, session type, class, active flag, and lock hint. Polling is read-only.

## Security boundary

No unlock operation is implemented. `loginctl unlock-session` must never be called merely because a device is nearby, a notification arrived, or a generic HTTP response succeeded. Any future unlock must require a fresh, one-time, request-bound approval from Authelia and a local privileged broker that only unlocks the caller's active graphical session.

## Planned architecture

- UI: status and configuration.
- Session monitor: read-only logind state.
- Authelia client: supported passwordless flow; no undocumented endpoint assumptions.
- Unlock broker: narrow local IPC, authenticated caller, one-time nonce, expiry, session binding, replay protection.

GSConnect remains installed and independent. It is not trusted as an authentication authority.
