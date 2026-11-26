# Beta 10.6

- Added a testable adapter from the approval coordinator to the existing Duo Push client.
- Added diagnostic inspection of the active logind graphical user session.
- Added tests for cancellation, adapter callback propagation, and session-state rejection.
- Kept password/PAM integration unconnected and manual fallback semantics unchanged.
- No system authorization, grant creation, PAM/sudoers/Polkit/systemd changes, or privileged actions.
