# ProxiLens sudo security (experimental)

The optional sudo integration is separate from GNOME session unlocking.
When enabled for the current local Ubuntu user:

- Ubuntu's normal password authentication remains required for every `sudo`
  invocation (`timestamp_timeout=0` scoped to that user).
- Duo Push is required on the first successful sudo authentication for a
  terminal/login session, then suppressed for the configured 1–120 minute
  approval window (15 minutes by default).
- A Duo deny, timeout, malformed response, or network failure denies the PAM
  authentication (fail closed).
- The grant is stored under `/run` and bound to the local UID and logind
  session ID (shared across terminals in one GNOME login session). Systems
  without `XDG_SESSION_ID` fall back to the kernel process-session ID plus
  terminal. Grants are cleared at reboot and on package removal/deactivation.
- Other local users are not subject to ProxiLens Duo and retain their existing
  sudo timestamp policy.

The root-only helper is installed at `/usr/lib/proxilens/proxilens-sudo-helper`.
The Duo credentials are copied into `/etc/proxilens/sudo.json` with mode 0600.
The non-secret UI status is `/etc/proxilens/sudo-status.json` (0644).

Activation requires a working `pkexec`, `visudo`, the `pam_exec.so` PAM module,
network access to the configured Duo Auth API and saved Duo credentials.
The helper validates the Duo API connection before it changes PAM/sudoers.
It inserts one marked `pam_exec.so` line immediately after `@include common-auth`
in `/etc/pam.d/sudo`, and manages only `/etc/sudoers.d/proxilens`. Deactivation
removes only that marked line and that dedicated sudoers file; it does not
rewrite unrelated PAM or sudoers configuration.

This feature is experimental and should first be enabled on a machine where an
administrator has console/recovery access. Keep a second root-capable session
open during first activation. Never manually edit `/etc/pam.d/sudo` while the
feature is being enabled or disabled.

## Recovery

If sudo authentication behaves unexpectedly, use an already-open root-capable
session or Ubuntu recovery mode. Disable the feature with:

```sh
/usr/lib/proxilens/proxilens-sudo-helper disable
```

If the helper cannot run, remove only the ProxiLens marker and its immediately
following `pam_exec.so` line from `/etc/pam.d/sudo`, then remove
`/etc/sudoers.d/proxilens`. Do not replace the entire PAM file with a generic
copy; preserve the host's other authentication rules.
