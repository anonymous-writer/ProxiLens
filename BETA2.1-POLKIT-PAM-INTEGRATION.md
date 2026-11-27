# ProxiLens 2.1.0 Beta – Polkit PAM integration patch

## Scope

This source update adds a second PAM caller type (`polkit-1`) to the existing
Duo authorization broker. The existing `sudo` peer validation remains a
separate code path. The PAM module sends the PAM service name and PAM user as
two newline-terminated fields.

The broker accepts only:
- `sudo`, after the existing sudo executable and real-UID checks; or
- `polkit-1`, after checking SO_PEERCRED, the `polkitd` UID, the executable
  `/usr/lib/polkit-1/polkitd`, and the configured PAM user.

The socket is created as `root:polkitd` with mode `0660`. Its parent directory
remains `root:root` with mode `0755`. Group membership is not treated as
authorization: the broker still validates the peer process.

## Files changed

- `pam/src/pam_proxilens.c`
- `cmd/proxilens-policy/main.go`
- `packaging/systemd/system/proxilens-policy.service`
- `scripts/install-polkit-pam-proxilens.sh` (new)
- `scripts/remove-polkit-pam-proxilens.sh` (new)

## Build

On the target Ubuntu system, with Go and `libpam0g-dev` installed:

```sh
gofmt -w cmd/proxilens-policy/main.go
go test -race ./...
make -C pam clean all
CGO_ENABLED=0 go build -trimpath -o build/proxilens-policy ./cmd/proxilens-policy
```

Then install the updated broker, PAM module, and systemd unit using the
existing staged build/install workflow. Restart `proxilens-policy.service`
and verify:

```sh
stat -c '%U:%G %a %n' /run/proxilens-policy/auth.sock
```

Expected socket ownership and mode: `root:polkitd 660`.

Only after that should the separate Polkit PAM installer be run:

```sh
sudo ./scripts/install-polkit-pam-proxilens.sh
```

Rollback:

```sh
sudo ./scripts/remove-polkit-pam-proxilens.sh
```

The installer refuses to overwrite an existing `/etc/pam.d/polkit-1` and
requires the updated socket permissions. It leaves `/usr/lib/pam.d/polkit-1`,
`/etc/pam.d/sudo`, and sudoers untouched.

## Validation status and limitations

This patch is source-level only. It has not been tested against a live GNOME
Polkit authentication agent. In particular, the text prompt using PAM
`PAM_PROMPT_ECHO_ON` must be verified with the actual agent before relying on
this integration. A successful C build, Go tests, live Duo approval, denial,
manual-password selection, and rollback test on the target Ubuntu host are
required before activation.

The broker still uses the existing single configured Duo identity. It does
not implement a per-local-user Duo identity map.
