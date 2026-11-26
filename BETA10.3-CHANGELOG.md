# Beta 10.3 changelog

- Added `internal/authflow`, a thread-safe state machine for Duo-primary authentication and explicitly selected password backup.
- Password backup is unavailable while Duo is pending and remains disabled after successful Duo authentication.
- Password backup becomes selectable only after Duo failure/unavailability or explicit cancellation; selection requires a separate click.
- Added tests for primary Duo, manual fallback, cancellation, and invalid/implicit transitions.
- Updated the desktop UI labels and status messages to make the manual backup policy explicit.
- No password collection, PAM authentication, Polkit, sudoers, or host configuration changes are included.
