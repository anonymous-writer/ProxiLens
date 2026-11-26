# Beta 10.10.3

- Added `proxilens-duo-sync`, which decrypts the existing ProxiLens settings in the user session and sends them to a root importer via `pkexec` stdin.
- Added a root-only importer that atomically writes `/etc/proxilens/duo.conf` with mode 0600.
- Disabled the old manual duplicate credential-entry script.
- Root policy service now reads the configured Duo push type.
- Build/install now builds and installs the sync client and importer.
- No changes to `/etc/pam.d/sudo` or sudoers; production activation remains gated.
