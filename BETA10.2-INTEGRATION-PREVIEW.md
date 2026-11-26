# ProxiLens 2.1.0 Beta 10.2 – Integration Preview

This release adds a fail-closed policy decision model. It is an engineering
preview and does not activate system authentication or grant privileges.

## Added in Beta 10.2

- `internal/policygate`: a one-shot, short-lived decision gate bound to UID,
  logind session ID, PAM service, and the exact action.
- Independent factor-verification and baseline-policy interfaces. Both must
  succeed; any error, cancellation represented as a failed verification,
  timeout, missing dependency, malformed request, or baseline denial rejects.
- Peer identity is supplied separately from request claims and must match.
- Nonces are consumed before external verification, so failed proofs cannot be
  replayed within the gate process.
- Unit tests cover successful evaluation, peer mismatch, replay, factor failure,
  baseline denial/error, expiry, and malformed bindings.

## Not activated

- No PAM shared object or PAM stack fragment is installed or shipped.
- No Polkit authentication agent is registered or replaced.
- No sudoers, `/etc/pam.d`, Polkit, or systemd host configuration is changed.
- The existing policy probe remains observation-only and always reports
  `authorized: false`.
- The policy gate is not wired to Duo, PAM, Polkit, or a privileged endpoint.
  Its interfaces are contracts for integration work, not proof that a trusted
  factor verifier or baseline authorizer exists in this build.

## Why activation remains disabled

A secure end-to-end integration needs a production Duo transaction verifier,
trusted PAM conversation handling, a privileged service that obtains peer
credentials and session identity independently, an authoritative sudoers/
Polkit policy check, and a tested transactional installer with recovery. The
current desktop Duo flow and user broker are not trusted root-side evidence.

Do not install a PAM module, edit `/etc/pam.d`, modify sudoers, or replace the
Polkit agent using this preview. Test on a disposable VM before any future
activation release. Keep an independent root shell and recovery media available.
