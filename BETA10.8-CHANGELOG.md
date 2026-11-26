# Beta 10.8 – Duo-PAM Staging

- Added read-only Duo-PAM preflight checks; secrets are never printed.
- Added a separate `proxilens-duo-test` PAM service installer/remover.
- Test service authenticates with Duo only; it does not touch sudo/common-auth.
- Requires `failmode=secure`, `autopush=yes`, `prompts=1`, root-owned private Duo config.
- Manual local-password fallback deliberately remains out of this PAM test until a trusted explicit-selection path is implemented.
