# ProxiLens safe development branch (2.1.0-beta4)

## Purpose

This branch is a non-privileged development build for validating Duo API connectivity, Duo Push status reporting, and a local 15-minute grant-state simulation.

## Explicit security boundary

This branch does **not** integrate with or modify:

- PAM (`/etc/pam.d/*`, PAM modules, or PAM stacks)
- Polkit authentication agents or policies
- sudo configuration or `/etc/sudoers*`
- system-wide or user systemd services
- GNOME lock-screen authentication or session unlocking

The simulated grant exists only in process memory. It is not persisted, exported, or consumed by any operating-system authorization component. It grants no OS permissions and is not an authentication control.

## Build

On Ubuntu, install the normal Go and Fyne build dependencies for this repository, then run:

```sh
go test ./...
go build -trimpath -o proxilens-ui .
```

The Debian packaging rules build and install only `proxilens-ui`, its desktop launcher, icon, and application metadata. They do not compile or install a privileged helper, PAM module, wake agent, or GNOME extension.

## Duo test

1. Enter the Duo API integration details in the application and save them.
2. Use **Verbindung prüfen** to test the signed Duo API check request.
3. Use **Duo Push testen** to send an actual push and wait for allow, deny, or timeout.
4. After a successful push, use **15-Minuten-Freigabe simulieren** to test the local in-memory expiry state.

Do not treat the simulation as a real sudo or Polkit authorization. The Duo Secret Key is stored using the existing credential-store mechanism; never include it in logs, screenshots, support bundles, or chat messages.

## Deferred work

A future system-authentication integration requires a separate threat model, protocol and authorization design, rollback/recovery implementation, and automated tests on disposable virtual machines. It must not be added to this safe branch by reintroducing PAM or Polkit edits opportunistically.
