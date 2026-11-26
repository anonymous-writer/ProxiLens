# PAM integration design – not an activation file

The future PAM adapter is an authentication conversation bridge only. It must
not grant privileges or modify sudoers/Polkit policy. It must preserve the
Ubuntu PAM stack and return `PAM_AUTH_ERR` on an explicit cancel, timeout,
protocol failure, or failed verification. `PAM_IGNORE` must not be used as a
fallback path because its effect depends on the surrounding PAM control flags.

A successful factor check may contribute to an action-specific policy decision;
it is not itself a general-purpose 15-minute root token. Any shared grant must
be validated by a separate privileged policy component against the current
kernel peer credentials, logind session, requested action, and existing
sudoers/Polkit policy.

This document is a design constraint, not source code or a PAM stack snippet.
