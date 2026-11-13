# Direct Duo Push Authentication

ProxiLens uses exclusively the direct Duo Auth API.

## Setup

In the settings, select `Direct Duo Push` and enter the IKEY, SKEY, API Hostname, and Duo username. The credentials are encrypted in SQLite; the AES key is stored in the GNOME keyring. The Secret Key is never logged.

`Check Connection` calls `/auth/v2/check` and does not send a push notification.

When the explicit GNOME unlock button is pressed, or during the existing resume flow, the agent sends `/auth/v2/auth` with `factor=push`, `device=auto`, and `async=1`. It then polls `/auth/v2/auth_status` for up to 90 seconds and accepts only `allow`. Afterwards, the agent checks again whether the same GNOME session is still locked before invoking the existing unlock path.

The Duo Secret Key must remain exclusively within the local application. Never store it in Git, CI variables for a regular package build, or logs.

## Testing Notes

- First, use `Check Connection`.
- Then test the GNOME button and approve or deny the Duo Mobile request.
- Run `go test ./...` and the manual AMD64 DEB workflow.
- Before production use, verify the Duo API integration and desktop unlock behavior on the target GNOME version.

## Signing

The Duo integration uses Duo's recommended v5 signing method (HMAC-SHA512). POST requests are sent as compact JSON; GET parameters are URL-encoded in accordance with RFC 3986. This corresponds to the current standard used by the official Duo API clients. The older v2/HMAC-SHA1 signing method is no longer used.