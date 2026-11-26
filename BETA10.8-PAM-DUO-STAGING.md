# ProxiLens 2.1.0 Beta 10.8 – Duo-PAM Staging

## Zweck

Diese Stufe beginnt die echte PAM-Integration, ohne den produktiven `sudo`-PAM-Stack umzuschreiben. Sie richtet einen separaten PAM-Service `proxilens-duo-test` ein, der ausschließlich Duo authentifiziert. `sudo` und `common-auth` bleiben unverändert.

## Warum zuerst Duo-only?

Die offizielle Duo-Unix-PAM-Integration ist ein PAM-Modul (`pam_duo.so`). Die übliche Ubuntu-Anleitung setzt es als zweiten Faktor hinter `pam_unix`; das wäre nicht Michaels gewünschter Ablauf. Die isolierte Testkonfiguration verwendet daher `pam_duo.so` als alleinigen Authentifizierer und `pam_unix.so` nur für die Account-Prüfung. Das prüft Duo Push selbst, nicht den späteren Passwort-Fallback.

Der gewünschte manuelle lokale Passwort-Fallback lässt sich nicht sicher durch einfaches Hinzufügen einer zweiten `pam_unix.so`-Zeile realisieren: PAM-Stack-Kontrollflags würden keinen vertrauenswürdigen UI-Auswahlschritt darstellen. Dafür ist ein eigener, expliziter und authentisierter ProxiLens-Fallback-Pfad erforderlich. Bis dieser gebaut und geprüft ist, wird kein lokaler Passwort-Fallback in den Testdienst aufgenommen.

## Kritische Duo-Einstellung

Duo dokumentiert `failmode=safe` als Standard. Bei Konfigurations-/Dienstfehlern kann das Zugriff erlauben. Für eine privilegierte Anwendung muss die Duo-Konfiguration daher `failmode=secure` enthalten. Diese Staging-Prüfung verlangt zusätzlich:

- `[duo]` mit `ikey`, `skey`, `host`
- `failmode=secure`
- `autopush=yes`
- `prompts=1`
- root als Eigentümer und keine Gruppen-/Andere-Rechte (z.B. Modus 0600)

Duo-Konfigurationswerte oder Schlüssel werden von den Skripten niemals ausgegeben.

## Ablauf

1. Offizielle Duo-Unix-Pakete nach der aktuellen Cisco-Duo-Anleitung installieren. Nicht `pam_authelia` verwenden; das ist ein anderes Produkt.
2. `/etc/duo/pam_duo.conf` oder `/etc/security/pam_duo.conf` mit den Duo-Unix-Anwendungsdaten konfigurieren. Geheimnisse nur lokal und root-only eintragen.
3. `./scripts/pam-duo-preflight.sh` ausführen. Das Skript liest nur und ändert nichts.
4. `pamtester` installieren, falls erforderlich: `sudo apt install pamtester`.
5. Separaten Testdienst anlegen: `sudo ./scripts/install-pam-duo-test-service.sh`.
6. Duo-Test ausführen: `sudo pamtester proxilens-duo-test "$USER" authenticate`.
7. Testdienst entfernen: `sudo ./scripts/remove-pam-duo-test-service.sh`.

## Strikte Grenzen

- Keine Änderung an `/etc/pam.d/sudo`, `/etc/pam.d/common-auth`, sudoers, Polkit oder systemd.
- Kein automatischer Passwort-Fallback.
- Der Duo-Testdienst gewährt keine Root-Aktion; er prüft nur PAM-Authentifizierung.
- Ein erfolgreicher Duo-Test ist keine produktive sudo-Freigabe.
- Für eine spätere sudo-Umstellung sind ein geprüfter Fallback-Entwurf, eine zweite Root-Sitzung, Recovery-Medium und ein getesteter Rollback zwingend.

## Offizielle Dokumentation

Cisco Duo Unix: https://duo.com/docs/duounix
Cisco Duo Unix FAQ: https://duo.com/docs/duounix-faq
