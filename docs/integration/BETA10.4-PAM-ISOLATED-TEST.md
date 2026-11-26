# ProxiLens Beta 10.4 – isolierter PAM-Diagnosetest

## Ziel

Diese Stufe prüft, ob Linux-PAM auf dem Ubuntu-Rechner das lokale Passwort des
aufrufenden Kontos authentifizieren kann. Sie ist ein Diagnosewerkzeug und
noch **keine** ProxiLens-Passwort-Fallback-Integration.

## Sicherheitsgrenzen

- Es wird keine Datei unter `/etc/pam.d`, `/etc/sudoers*` oder in der Polkit-
  Konfiguration angelegt oder verändert.
- Das Programm erzeugt zur Laufzeit eine private, root-eigene PAM-Service-
  Konfiguration unter `/run/proxilens-pamtest.XXXXXX` (0700/0600) und entfernt
  sie nach dem Test.
- Der temporäre Service enthält ausschließlich `pam_unix.so` für `auth` und
  `account`; er bindet absichtlich nicht `common-auth` ein und testet daher
  nicht sämtliche lokalen MFA-, Lockout- oder Kontorichtlinien.
- Das Programm akzeptiert keine Passwörter als Argument oder Umgebungsvariable.
  Das Passwort wird verdeckt vom kontrollierenden Terminal gelesen und nicht
  protokolliert oder gespeichert.
- Unter sudo-rs werden `SUDO_UID` und `SUDO_USER` zur Ermittlung des
  aufrufenden lokalen Kontos verwendet; beide müssen zu einem lokalen Konto
  passen. Das Programm selbst läuft dabei mit UID 0. Direkte Root-Aufrufe ohne
  diese sudo-Metadaten werden abgelehnt. Diese Umgebungsvariablenprüfung ist
  eine Schutzmaßnahme gegen Fehlbedienung, keine Sicherheitsgrenze gegen einen
  bereits privilegierten Root-Akteur.
- Erfolg bedeutet ausschließlich „PAM hat das lokale Konto authentifiziert“.
  Es wird keine Aktion ausgeführt, kein Grant erzeugt und keine Systemberechtigung
  erteilt.
- Da der Test mit `sudo` gestartet wird, kann sudo bereits vorher eine eigene
  Authentifizierung verlangen. Dieser Diagnosetest beweist daher nicht den
  späteren interaktiven Backup-Ablauf in ProxiLens.

## Voraussetzungen

Ubuntu mit Linux-PAM >= 1.4.0, `build-essential` und `libpam0g-dev`.
Die API `pam_start_confdir()` wird für die isolierte Konfiguration verwendet.

## Bauen und testen

Im Beta-10.4-Projektordner als normaler Benutzer:

```bash
sudo apt install build-essential libpam0g-dev
./scripts/build-pam-test.sh
sudo -- ./build/proxilens-pam-test
```

Das Passwort wird im Terminal ohne Echo abgefragt. Erwartet wird entweder eine
Erfolgsmeldung zur PAM-Diagnose oder eine Ablehnung ohne Freigabe.

## Noch nicht enthalten

- kein PAM-Modul für `/etc/pam.d/sudo` oder andere produktive PAM-Dienste;
- kein GUI-Passwortdialog und keine Verbindung zum `authflow`;
- kein Root-IPC-Daemon und keine Grant-Erteilung;
- keine Polkit-Integration;
- kein automatischer Installer und keine Änderungen an der Host-Konfiguration.

## Nächste sichere Schritte

1. Den isolierten Diagnosetest auf einer Wegwerf-VM und danach auf dem Zielsystem
   ausführen.
2. Einen separaten, root-eigenen Broker mit `SO_PEERCRED`, PID-Startzeit,
   logind-Sitzungsprüfung und strikt begrenztem Protokoll entwerfen.
3. PAM-Conversation nur über einen authentisierten, sitzungsgebundenen Kanal
   abwickeln; Abbruch, Timeout und jeder Protokollfehler müssen verweigern.
4. Erst nach unabhängiger Sicherheitsprüfung eine optionale PAM-Stack-Integration
   mit Backup und getesteter Rückrollmöglichkeit planen.
