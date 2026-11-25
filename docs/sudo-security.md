# ProxiLens sudo security (experimental)

The optional sudo integration is separate from GNOME session unlocking. When enabled for the configured local Ubuntu user:

- Duo Push is the only PAM authentication factor for sudo. For the configured user, PAM skips the normal Ubuntu password stack and uses Duo as the only authentication gate. Other users continue through Ubuntu `common-auth`.
- The first sudo authentication requires a Duo `allow`. A successful approval is cached for the configured 1–120 minute window (15 minutes by default).
- The grant is bound to the local UID and logind login-session ID, so it is shared across terminals and remains valid across screen lock/unlock. It expires by wall-clock time and is cleared at reboot and deactivation.
- A Duo deny, timeout, malformed response, missing configuration, or network failure denies sudo (fail closed). There is no password fallback.
- Other local users retain the original PAM password stack.

The root-only helper is installed at `/usr/lib/proxilens/proxilens-sudo-helper`. Duo credentials are stored in `/etc/proxilens/sudo.json` with mode 0600. The original `/etc/pam.d/sudo` and Polkit PAM stack are backed up under `/etc/proxilens/` with mode 0600 and restored on deactivation. When `/etc/pam.d/polkit-1` did not previously exist, ProxiLens derives an override from `/usr/lib/pam.d/polkit-1` and removes that override again on deactivation; the vendor file is never modified. Sudo and graphical Polkit authentication share one grant bound to the unique active logind desktop session, UID and expiry. If logind cannot identify exactly one active desktop session, authentication fails closed.

Polkit's `polkit-agent-helper@.service` is shipped with `PrivateNetwork=yes` and `RestrictAddressFamilies=AF_UNIX`. Those defaults prevent the PAM helper from resolving or reaching Duo. On activation ProxiLens therefore installs the narrowly scoped systemd drop-in `/etc/systemd/system/polkit-agent-helper@.service.d/50-proxilens-network.conf`: it sets `PrivateNetwork=no` and permits `AF_UNIX`, `AF_INET`, and `AF_INET6`. Other vendor hardening settings (including `NoNewPrivileges`, `PrivateTmp`, `ProtectSystem`, and syscall filters) remain in force. The previous contents of this exact drop-in path, or its prior absence, are backed up under `/etc/proxilens/`; deactivation restores that state and runs `systemctl daemon-reload`. The vendor unit under `/usr/lib/systemd/system` is never edited.

Activation validates the Duo API before modifying PAM. It replaces the `@include common-auth` entry with a conditional PAM substack: only the configured user skips Ubuntu password authentication and is gated by Duo; other users retain the normal password stack. Account and session rules remain intact. It manages only `/etc/sudoers.d/proxilens`. Keep a second root-capable recovery session open during first activation and verify the Duo flow before closing it.

## Recovery

If sudo authentication behaves unexpectedly, use an already-open root-capable session or Ubuntu recovery mode. Restore the original PAM file and disable the feature with:

```sh
/usr/lib/proxilens/proxilens-sudo-helper disable
```

Do not manually edit `/etc/pam.d/sudo` or `/etc/pam.d/polkit-1` while activation or deactivation is running.


## Interaktiver Duo-Backup-Pfad (ProxiLens 2.1.0 Beta)

Die sudo- und Polkit-PAM-Stacks verwenden `pam_proxilens.so`. Das Modul startet zuerst den root-geschützten ProxiLens-Duo-Helfer. Der PAM-Konversationskanal erhält eine Statusmeldung. Wenn Duo fehlschlägt oder nach dem Push-Zeitlimit nicht antwortet, fragt PAM ausdrücklich nach einer Auswahl: `P` setzt mit Ubuntus regulärem `common-auth`-Stack fort; jede andere Eingabe bricht die Authentifizierung ab. Ein erfolgreicher Duo-Push überspringt `common-auth`. Für andere lokale Benutzer wird das Modul übersprungen und der Ubuntu-Standardstack bleibt aktiv.

Die Passwortauswahl ist bewusst eine textbasierte PAM-Konversationsabfrage (keine separate GNOME-Schaltfläche). Ob der installierte Polkit-Agent diese Abfrage gut lesbar darstellt, muss auf dem Zielsystem getestet werden. Der PAM-Baustein wird gegen `libpam0g-dev` gebaut und unter dem Multiarch-Security-Verzeichnis installiert.
