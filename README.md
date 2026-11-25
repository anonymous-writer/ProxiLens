# ProxiLens

**Safe development build – direct Duo diagnostics and local grant simulation.**

> [!CAUTION] **No system authentication integration.** This branch does not install or enable PAM modules, Polkit agents, sudoers rules, systemd services, GNOME lock-screen extensions, or session-unlock agents. Duo approval in this application does not grant operating-system privileges and does not unlock a session. Do not use this build as a system security control.

ProxiLens Beta 4 is a non-privileged development build. Its purpose is to validate the existing Duo API client, show authentication progress, and exercise a 15-minute grant state held only in application memory.

## Included

- Fyne desktop settings application.
- Existing encrypted credential-store integration using the GNOME keyring.
- Signed Duo Auth API connectivity check.
- Actual Duo Push test with waiting, allow, deny, and timeout status.
- In-memory 15-minute grant simulation after successful Duo Push.
- German, English, and Spanish UI translations.

## Explicitly excluded

- `sudo`, PAM, `/etc/pam.d/*`, PAM modules, and sudoers changes.
- Polkit authentication agents, policies, and `/etc/polkit-1/*` changes.
- Root helpers, `pkexec` workflows, systemd services, GNOME extensions, and session unlocking.
- Persistent or cross-process authorization grants.

The simulated grant is informational only. It is not persisted, exported, or consumed by any operating-system authorization component. It grants no OS permissions.

## Duo test workflow

1. Enter the Duo Integration Key, Secret Key, API hostname, and Duo username in the application.
2. Save the settings.
3. Select **Verbindung prüfen** to test the signed Duo `/auth/v2/check` request.
4. Select **Duo Push testen** to send a real push and wait for an allow, deny, or timeout result.
5. After a successful push, select **15-Minuten-Freigabe simulieren** to start an in-memory test timer.
6. The UI reports when the simulated grant expires. Restarting the application clears it.

Never include the Duo Secret Key in logs, screenshots, support bundles, or chat messages.

## Build from source

Install Go and the Linux/Fyne development dependencies listed in [`.github/workflows/ci.yml`](.github/workflows/ci.yml), then run:

```sh
go test ./...
go build -trimpath -o proxilens-ui .
```

The Debian packaging rules build and install only the desktop UI, launcher, icon, and application metadata. They do not compile or install privileged or session-integration components.

## Safety boundary

The UI and Duo API client run as the logged-in user. The application has no mechanism in this branch to authorize `sudo` or Polkit actions. A future system-authentication integration requires a separate threat model, protocol and authorization design, recovery-tested rollback, and automated testing on disposable virtual machines. It must not be added by reintroducing PAM or Polkit edits opportunistically.

See [SAFE-DEVELOPMENT.md](SAFE-DEVELOPMENT.md) for the branch scope and test details.
