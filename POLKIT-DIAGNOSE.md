# Polkit-Diagnose (nur Quellcode)

Diese Revision ergänzt ausschließlich Diagnosemeldungen im PAM-Helfer.

## Was protokolliert wird

Bei `pam-auth` werden in `authpriv` protokolliert:

- PAM-Service und PAM-Typ
- PAM-Benutzer, PAM-Ruser und PAM-TTY
- Real-UID und Effective-UID des Helfers
- Diagnosephase und interner Ablehnungsgrund

Duo-IKEY/SKEY, HTTP-Anfrageinhalte und Push-Tokens werden nicht protokolliert.
Die Meldungen funktionieren auch mit `pam_exec.so quiet`, weil sie direkt
an syslog geschrieben werden.

## Nach Build und kontrollierter Installation prüfen

Nach einem fehlgeschlagenen grafischen Polkit-Versuch:

```bash
sudo journalctl -b -t proxilens-sudo-helper --no-pager
```

Erwartete Diagnosephasen sind `start`, gegebenenfalls `privilege-check`,
`authentication` oder `success`. Die Zeile `reason=...` benennt den konkreten
Fehlerpfad. Für einen fokussierten Polkit-Test:

```bash
sudo journalctl -b -t proxilens-sudo-helper --no-pager | grep 'service="polkit-1"'
```

Diese Diagnoseversion ändert keine PAM-Datei und wird nicht automatisch
installiert. Vor dem Austausch des installierten Helfers muss der Projektbuild
und anschließend ein kontrollierter Polkit-Test auf dem Zielsystem erfolgen.
