# Policy service Beta 9.2

This is a hardened, observation-only diagnostic probe. It is not an
authorization service. The service reads kernel peer credentials, records the
peer process start time, checks that identity again after logind session lookup,
requires an exact `session-N.scope` cgroup component, and accepts one bounded
newline-terminated JSON frame only. It always returns `authorized: false`.

The PAM directory in this release is documentation-only. There is no loadable
PAM module and no activation snippet. PAM, Polkit, sudoers, and the existing
user broker are not modified.
