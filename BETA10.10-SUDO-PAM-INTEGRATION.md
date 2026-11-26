# Beta 10.10 – Direct Duo PAM + sudo integration

This iteration adds an initial real integration path:

- Native Linux PAM module `pam/src/pam_proxilens.c`.
- The PAM module defaults to Duo; an explicit `P` selection returns PAM_IGNORE so the existing Ubuntu `common-auth` stack asks for the local password. Any Duo failure is PAM_AUTH_ERR and the PAM control line is fail-closed.
- Root daemon `cmd/proxilens-policy` uses the existing `internal/duo` Auth API client and waits for an explicit Duo `allow` result.
- Daemon checks SO_PEERCRED, the peer executable (`sudo`), and the real UID from `/proc/PID/status` against PAM_USER.
- Root-only Duo credentials at `/etc/proxilens/duo.conf`, mode 0600.
- systemd unit, build/install script, PAM insertion script, sudoers drop-in, and removal script.

## Build and staged installation

1. `sudo ./scripts/build-install-sudo-pam.sh` builds and installs the PAM module, daemon, and unit. It does not edit `/etc/pam.d/sudo` or sudoers.
2. `sudo ./scripts/configure-duo-root.sh` provisions the Duo API credentials interactively into a root-only file. Do not paste the SKEY into chat.
3. Keep a root shell open and run `sudo ./scripts/install-sudo-pam-proxilens.sh` from another terminal. It backs up `/etc/pam.d/sudo` and sudoers state, inserts the PAM line before `common-auth`, installs `Defaults:%sudo timestamp_timeout=0`, validates `visudo -c`, and starts the service.
4. Test a new sudo command in another terminal before closing the root shell.

## Security limitations to review before production

- This is an initial single-Duo-identity broker: `username` in the root config is the Duo username for all local sudo users. Multi-user UID-to-Duo identity mapping is not implemented.
- The PAM choice prompt is textual. A blank/default response selects Duo; only explicit `P` selects the Ubuntu password stack.
- The service verifies that the connecting process is sudo and that its real UID matches PAM_USER. The socket is root-only (0600).
- The service authorizes sudo authentication only; sudoers continues to decide which commands are permitted. The drop-in only disables timestamp reuse for the sudo group.
- Test carefully with a separate root shell and recovery media available. PAM changes can lock out administrative access if the platform's sudo/PAM behavior differs.
- This environment did not run the target Ubuntu build or a live Duo transaction. Run the build and integration tests on the target host before relying on it.
