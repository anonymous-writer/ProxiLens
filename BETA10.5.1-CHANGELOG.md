# Beta 10.5.1 test correction

- Corrected `TestPasswordFallbackRequiresExplicitCallAfterDuoFailure` to first
  execute the Duo primary attempt and verify that the failed attempt does not
  call the password adapter automatically.
- The test then calls `ManualPasswordFallback` as the separate explicit user
  action and checks the expected `ErrNotAuthorizing` result. That sentinel is
  intentional: Beta 10.5 can report successful authentication, but it cannot
  create or authorize a system grant.
- No production authentication, PAM, sudoers, Polkit, or system-service code
  was changed by this test-only correction.
