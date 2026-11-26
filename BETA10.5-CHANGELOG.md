# Beta 10.5 changelog

- Added `internal/approval` to orchestrate Duo, explicitly selected password
  authentication, and read-only session inspection through interfaces.
- Added tests for manual-only fallback and the invariant that authentication
  never equals authorization.
- Kept privileged grants disabled; no production PAM, sudoers, Polkit, or
  system-service configuration is modified.
