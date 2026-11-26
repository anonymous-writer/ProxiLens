# ProxiLens 2.1.0 Beta 10.4 – PAM Isolated Test Preview

Beta 10.4 adds a standalone Linux-PAM diagnostic utility. It uses a private,
throwaway PAM service configuration under `/run` and does not edit the host's
PAM, sudoers, or Polkit configuration. It authenticates only the invoking local
account and never authorizes a system action.

See `docs/integration/BETA10.4-PAM-ISOLATED-TEST.md` for prerequisites,
commands, limitations, and the next integration steps.
