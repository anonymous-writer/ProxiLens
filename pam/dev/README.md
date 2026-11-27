# ProxiLens PAM development scaffold (Beta 9.2)

This directory is deliberately **not an installable PAM module**. It contains
requirements and a test plan for a future module. No `.so` is built, no PAM
service file is shipped, and no file under `/etc/pam.d` is changed.

## Required behavior for a future implementation

- Fail closed on every transport, protocol, timeout, identity, or policy error.
- Never treat a successful Duo Push as blanket root authorization.
- Start Duo Push first when Duo is enabled.
- Offer Ubuntu-password authentication only after an explicit user choice in a
  trusted conversation owned by the appropriate authentication agent.
- Cancellation must return authentication failure; it must not fall through to
  another PAM module or silently select password authentication.
- Preserve the normal Ubuntu account and session PAM stacks.
- Bind any short-lived approval to the requesting UID, logind session, PAM
  service, and a specific action; re-check the action through sudoers/Polkit.
- Do not trust environment variables supplied by the client as proof of session
  identity. Obtain peer credentials from the kernel and session identity from
  logind over its system interface.

## Why no module is included yet

A PAM module that returns success without a completed, verified transaction
would weaken authentication. A module that returns ignore or success on an
error could also produce unsafe stack behavior. The protocol and trusted UI
conversation contract are not complete enough to implement this safely.

Do not copy any example into `/etc/pam.d`, and do not compile or install a PAM
shared object from this scaffold. The current root policy probe remains
observation-only and always returns `authorized: false`.

## Required test gates before implementation

1. Unit tests for every PAM return path, including cancellation and timeout.
2. Integration tests in a disposable VM with console access.
3. Verify password login, graphical login, `sudo`, `su`, lock/unlock, and
   recovery from a broken module before any host activation.
4. Install only through a transaction with a verified backup and an independent
   rollback mechanism; keep an existing root shell open during activation.
