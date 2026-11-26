# ProxiLens 2.1.0 Beta 10.1

Beta 10.1 improves the desktop authentication workflow while preserving the
safe development boundary from Beta 10. It adds Duo request cancellation,
explicit simulation-grant revocation, and a clearly labelled Ubuntu-password
choice that explains the not-yet-enabled PAM path.

## Build and test

Use the Go version declared by the project (`go 1.27`) and run:

```sh
go test -race ./...
go build -trimpath -o proxilens-ui .
```

## Important limitations

- The 15-minute grant is simulation-only and grants no OS permissions.
- The Ubuntu password option is informational only; the app does not request or
  handle the user's Ubuntu password.
- No PAM module, Polkit agent, sudoers rule, or privileged authorization endpoint
  is installed or activated.
- Do not use the development grant broker as an authorization source for root.
