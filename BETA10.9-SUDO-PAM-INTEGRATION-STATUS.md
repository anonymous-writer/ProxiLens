# Beta 10.9 – sudo/PAM integration work

## Implemented in this source drop

- Added a narrowly scoped sudoers drop-in: `Defaults timestamp_timeout=0`.
  It grants no command and does not widen existing sudoers authorization; it
  forces fresh authentication on each sudo invocation.
- Added transactional installer/remover scripts. The installer uses a new
  file under `/etc/sudoers.d`, refuses to overwrite an existing file, requires
  both the future PAM module and an active policy service, backs up sudoers,
  and validates with `visudo -c`. It never edits `/etc/sudoers` directly.
- The guard intentionally refuses activation in the current source state,
  because the real PAM module and authorizing service are not yet present.

## Not implemented / not safe to activate yet

- `pam_proxilens.so` is not yet implemented.
- The current `policyservice` is observation-only and never authorizes.
- The GUI keyring-encrypted Duo secret cannot be read by a root service. A
  dedicated root-owned credential provisioning path is still required.
- Therefore no PAM stack fragment is supplied and no production PAM file is
  modified by these scripts.
- The explicit user-selected local-password path is not wired into PAM yet.

## Why the sudoers drop-in is intentionally narrow

PAM controls authentication; sudoers controls which commands are permitted.
The drop-in only disables sudo timestamp reuse. It does not add `NOPASSWD`,
`SETENV`, command aliases, or blanket root permissions. It should only be
installed after the real PAM module and policy service have passed integration
and recovery tests.
