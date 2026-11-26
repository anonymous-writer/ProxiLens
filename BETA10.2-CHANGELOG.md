# Beta 10.2 changelog

- Added `internal/policygate`, a fail-closed decision model requiring matching
  peer identity, fresh one-shot request, factor verification, and independent
  baseline authorization.
- Added tests for successful decisions, peer mismatch, replay, expired and
  malformed requests, factor failure, and baseline denial/backend failure.
- Documented the non-activation boundary and remaining production integration
  requirements.
- No PAM, Polkit, sudoers, or host authentication configuration was changed.
