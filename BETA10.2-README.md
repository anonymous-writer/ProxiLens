# ProxiLens 2.1.0 Beta 10.2

Beta 10.2 adds a fail-closed, action-scoped policy decision model on top of
Beta 10.1. It is not a production PAM/Polkit integration and does not grant
system privileges.

## Tests

```sh
go test -race ./internal/policygrant ./internal/policyproto ./internal/policygate ./internal/policyservice
```

## Safety boundary

No host authentication configuration is changed. No PAM module, Polkit agent,
sudoers rule, or privileged authorization endpoint is enabled. See
`BETA10.2-INTEGRATION-PREVIEW.md` and
`docs/integration/BETA10-INTEGRATION-STATUS.md` before testing.
