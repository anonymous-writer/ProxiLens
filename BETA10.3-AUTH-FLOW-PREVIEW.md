# ProxiLens 2.1.0 Beta 10.3 – Auth Flow Preview

## Selected authentication policy

- Duo Push is the primary authentication method.
- The Ubuntu account password is a backup only.
- The backup is never selected automatically. It becomes selectable only after a Duo request fails, is unavailable, or is explicitly cancelled, and requires a separate user click.
- A successful Duo result keeps the password-backup control disabled.
- The application never collects, stores, logs, or sends the Ubuntu password to Duo.

## What this preview implements

- `internal/authflow` models the allowed state transitions and rejects password selection while Duo is pending or after Duo succeeds.
- The desktop UI disables the backup choice while Duo is pending and after Duo succeeds; it exposes the choice after a failed or cancelled Duo attempt.
- The UI records the manual selection and explains that PAM is not yet connected.
- Tests cover primary Duo, manual fallback selection, cancellation, and rejection of implicit password authentication.

## Important boundary

This is an authentication-flow prototype, not a production PAM integration. Selecting the backup does not authenticate the user and does not grant system privileges. No PAM, Polkit, sudoers, or systemd authorization configuration is installed or changed. The actual local password check must be implemented by a separately reviewed PAM adapter, with a trusted PAM conversation and fail-closed handling.
