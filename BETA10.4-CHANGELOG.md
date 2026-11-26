# Beta 10.4

- Added an isolated Linux-PAM diagnostic executable using `pam_start_confdir`.
- The temporary service is generated under `/run` with root-only permissions
  and removed after each run.
- The test checks the invoking local account only, using `pam_unix.so` for auth
  and account management.
- No `/etc/pam.d`, sudoers, Polkit, systemd, or desktop authentication settings
  are changed.
- The diagnostic creates no policy grant and is not wired into the GUI fallback.
- Added a build script that checks for the PAM development headers.
