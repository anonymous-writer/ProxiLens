# ProxiLens Beta 10 – Integration engineering preview

## What this build adds

- `internal/policygrant`: an in-memory model for 15-minute approvals bound to
  UID, logind session ID, PAM service (`sudo`/`polkit`) and an exact action.
- Tests that reject cross-user, cross-session, cross-service and cross-action
  reuse, and that verify expiry and session revocation.
- A corrected systemd boolean in the observation-only policy probe unit.

## What this build does not activate

This is **not yet a working PAM module or Polkit authentication agent**.
There is no installable PAM shared object, no PAM stack fragment, no Polkit
agent registration, no sudoers modification, and no privileged grant endpoint.
The desktop broker remains development-only and must not be trusted by root.
The policy probe remains observation-only and always reports
`authorized: false`.

This separation is intentional. A Duo approval is not sufficient to grant
root access. A future privileged component must independently obtain peer
credentials from the kernel, resolve the request's logind session using a
trusted system interface, bind the request to the exact PAM service/action,
verify the Duo transaction, and preserve the existing sudoers/Polkit decision.

## Required implementation sequence

1. Implement and test the privileged policy daemon with root-owned IPC,
   kernel-derived peer credentials, logind D-Bus session lookup, replay
   protection, and action-specific policy checks.
2. Implement the PAM adapter with a trusted conversation contract. Every
   cancel, timeout, transport error and malformed response must fail closed.
3. Integrate with the existing Polkit authentication-agent flow; do not replace
   GNOME's agent until a fully tested agent is available.
4. Add sudo-specific PAM policy without broad `NOPASSWD` rules or changes to
   unrelated PAM services.
5. Add a transactional installer with verified backups and an independent
   rollback path; test in a disposable system first and then on the Lenovo
   while retaining a second root shell and Live USB recovery.

## Host safety

Do not install a PAM module or edit `/etc/pam.d`, `/etc/sudoers*`, or Polkit
configuration from this preview. The package contains no active integration
configuration. The supplied unit is for the observation-only probe only.

## Beta 10.2 policy-gate model

Beta 10.2 adds `internal/policygate` as a testable decision contract. It requires
matching peer identity, a fresh one-shot nonce, a successful factor verifier,
and an independent positive baseline-policy decision. The gate consumes a
nonce before external checks and fails closed on all errors. This package is
not wired to the desktop Duo client or the root probe and is not a grant
endpoint. PAM, Polkit, sudoers, and host configuration remain untouched.
