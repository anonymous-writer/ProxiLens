# ProxiLens Beta 9.1 Fix 1

This is a source-level compatibility correction for Linux builds: `os.Chown` expects the group ID as an `int`, so the validated sudo-group GID is converted to `int` at both ownership calls.

No PAM, Polkit, sudoers, or system service configuration is changed by this fix. The root policy probe remains observation-only and always returns `authorized: false`.

Run on the target Ubuntu system:

```bash
go test ./internal/policyservice -count=1
go test -race ./internal/policyservice -count=1
go test ./...
go test -race ./...
go build -trimpath -o proxilens-ui .
```

Do not install or start the root service until all checks pass and the service implementation has been reviewed. This fix has not been built in the packaging environment because Go 1.27 toolchain download was unavailable there.
