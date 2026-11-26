# ProxiLens 2.1.0 Beta 9 – Root Policy Probe

Beta 9 introduces a **root-owned, observation-only Linux IPC service**. It reads the caller's kernel-provided peer credentials (`SO_PEERCRED`) and attempts to match the caller to a logind session using the process cgroup.

It is deliberately incapable of granting privileges: every response contains `authorized: false`. It does not modify PAM, Polkit, sudoers, Duo settings, or the existing per-user grant broker.

See `POLICY-SERVICE-BETA9.md` for the security boundary, build commands, optional manual installation, probe usage, and removal instructions.

This archive has not been built or tested in the packaging environment because its available Go compiler is older than the project's declared Go 1.27 toolchain and network access to download the toolchain is unavailable. Run the full test suite and build on the target Ubuntu system before installing the service.
