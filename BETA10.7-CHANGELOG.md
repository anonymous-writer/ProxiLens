# Beta 10.7 – Security-boundary prototype

- Added atomic, single-use consumption to the in-memory action-scoped approval store.
- Added a concurrent-consumption test proving exactly one caller can consume a grant.
- Added Linux `SO_PEERCRED` retrieval for accepted Unix-domain socket connections, exposing kernel-provided PID/UID/GID.
- Kept logind session resolution as a separate required step: request `SessionID` remains an assertion and is not trusted.
- No privileged IPC server, system authorization, PAM/sudoers/Polkit/systemd changes, or privileged actions were added.

## Security boundary

`SO_PEERCRED` must be read by the server from the accepted socket. The service must independently map the returned PID/UID to a current logind session and compare that result with the requested session. The GUI's UID/session fields, environment variables, and this prototype's in-memory store are not a privileged authorization boundary.
