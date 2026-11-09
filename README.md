# ProxiLens 0.0.2 – Alpha.7.3.1

ProxiLens ist ein experimenteller GNOME-/Ubuntu-Prototyp mit einem optionalen
Benutzer-Agenten für Sleep/Wake und einem Duo-Button im GNOME-Sperrbildschirm.
Der aktuelle Teststand ist für Ubuntu mit GNOME Shell 50.x und eine Wayland-
Benutzersitzung ausgelegt.

> **Status:** Alpha-/Testsoftware. Der hier beschriebene Ablauf wurde auf dem
> Zielgerät des Projekts (GNOME Shell 50.1, Wayland) praktisch getestet. Das ist
> kein allgemeines Sicherheitsversprechen für andere Systeme oder Versionen.

## Aktueller Funktionsumfang

### GNOME-Sperrbildschirm und Duo-Button

- Die GNOME-Erweiterung zeigt im `unlock-dialog` den Button **„Mit Duo entsperren“**.
- Ein bewusster Klick sendet `DuoUnlockRequested` über den Session-D-Bus an den
  ProxiLens-Benutzer-Agenten.
- Der Agent liest die gespeicherten Authelia-Zugangsdaten aus dem vorhandenen
  ProxiLens-Konfigurationsspeicher und fordert Authelia/Duo zur Authentifizierung auf.
- Nach erfolgreicher Duo-Bestätigung prüft der Agent erneut, dass dieselbe aktive
  Wayland-Sitzung weiterhin gesperrt ist, und fordert dann `loginctl unlock-session`
  für diese Sitzung an.
- Auf dem getesteten Gerät wurde der Desktop nach erfolgreicher Duo-Bestätigung
  ohne erneute Eingabe des Ubuntu-Passworts freigegeben.
- GNOME meldet den Wechsel zwischen `unlock-dialog` und `user`; zusätzlich wird
  `LockedHint` über logind beobachtet.

### Duo-Statusanzeige

Der Agent meldet Zustandswechsel an den Button zurück:

1. `LOCKED`
2. `DUO_PENDING`
3. `DUO_SUCCESS`
4. `UNLOCK_REQUESTED`
5. `UNLOCKED` – erst wenn GNOME den Wechsel aus dem Entsperrdialog meldet
6. `FAILED` – bei fehlgeschlagener Authentifizierung oder Entsperranforderung

Alpha.7.3.1 dedupliziert diese Zustandsmeldungen zentral. Wenn GNOME-Moduswechsel
und Authentifizierungsablauf denselben Zustand nahezu gleichzeitig melden, wird
der Status nur einmal protokolliert und einmal an die Erweiterung gesendet.

### Sleep/Wake

- Der Benutzer-Agent beobachtet logind `PrepareForSleep`.
- Nach dem Aufwachen wird die Netzwerkverfügbarkeit erneut geprüft, weil DNS und
  Netzwerkdienste nach Resume verzögert verfügbar sein können.
- Duo wird nach Sleep/Wake nur fortgesetzt, wenn die Sitzung weiterhin gesperrt ist.
- Die Sleep/Wake-Funktion ist unabhängig vom manuellen Klick auf den Duo-Button.

## Was ProxiLens nicht verändert

- Keine Änderung an `/etc/pam.d/*`, PAM oder GDM.
- Keine Änderung an der normalen GNOME-Sperrbildschirm-Konfiguration.
- Kein automatisches Duo beim normalen `Super+L`; dort bleibt der explizite Button-Klick erforderlich.
- Keine Freigabe allein aufgrund des GNOME-Statussignals. Der Agent prüft die
  Authelia-/Duo-Antwort und den aktuellen logind-Sperrstatus vor der Freigabe.

## Voraussetzungen

- Ubuntu mit GNOME Shell 50.x (Erweiterungsinstaller prüft GNOME 50.x).
- Wayland-Benutzersitzung.
- `systemd-logind`, `loginctl`, `dbus-monitor`, `gdbus`.
- Go 1.26.0 oder neuer entsprechend `go.mod`.
- Eine bereits eingerichtete ProxiLens-Konfiguration mit Authelia-URL,
  Authelia-Benutzername und gespeichertem Authelia-Passwort.
- Eine erreichbare Authelia-Instanz und ein eingerichteter Duo-Zweitfaktor.

## Installation / Aktualisierung

Die bisherige funktionierende Version vorher als Rückfallkopie behalten. ZIP in
ein eigenes Verzeichnis entpacken und dort ausführen:

```bash
bash deploy/install-wake-agent.sh
bash deploy/install-gnome-extension.sh
systemctl --user restart proxilens-wake-agent.service
```

Erweiterungsstatus prüfen:

```bash
gnome-extensions info proxilens-input@local
```

Agent-Logs verfolgen:

```bash
journalctl --user -u proxilens-wake-agent.service -f
```

GNOME-Erweiterungen können nach Updates oder einem Wechsel des Session-Modus neu
initialisiert werden. Falls der Button nicht erscheint:

```bash
gnome-extensions disable proxilens-input@local
gnome-extensions enable proxilens-input@local
```

## Testablauf

1. `Super+L` drücken und prüfen, ob „Mit Duo entsperren“ erscheint.
2. Den Button anklicken.
3. Den Duo-Push auf dem iPhone bestätigen.
4. Prüfen, dass der Desktop ohne erneute Ubuntu-Passworteingabe freigegeben wird.
5. Sleep/Wake separat testen und prüfen, dass bei weiterhin gesperrter Sitzung der
   bisherige Duo-Ablauf erhalten bleibt.

Erwartete Statusfolge beim Button-Ablauf:

```text
GNOME sessionMode: unlock-dialog aktiv
Duo-Zustand: LOCKED
Duo-Zustand: DUO_PENDING
Duo-Zustand: DUO_SUCCESS
Duo-Zustand: UNLOCK_REQUESTED
GNOME sessionMode: user-Modus aktiv
Duo-Zustand: UNLOCKED
```

Jeder Zustandswechsel soll im Agent-Log nur einmal erscheinen. Je nach Timing
kann `GNOME sessionMode: user-Modus aktiv` unmittelbar vor oder nach der letzten
Statusmeldung erscheinen.

## Sicherheits- und Betriebsgrenzen

- Dies ist eine experimentelle Entsperrhilfe für eine bereits angemeldete,
  gesperrte Benutzersitzung – kein PAM-Modul und kein Ersatz für die Anmeldung
  beim Systemstart oder den Login-Bildschirm von GDM.
- Der Agent läuft als Benutzerprozess im angemeldeten Benutzerkontext. Der
  Session-D-Bus ist kein systemweiter privilegierter Authentifizierungsbroker.
  Vor einem produktiven Einsatz ist eine weitergehende Sicherheitsprüfung nötig.
- Der GNOME-Button wird in private GNOME-Shell-Oberflächen (`Main.screenShield`
  und deren Dialog-/Clock-Widgets) eingefügt. GNOME-Updates können diese internen
  Schnittstellen ändern.
- Bei Netzwerk-/DNS-Ausfällen kann der Duo-Ablauf verzögert starten oder
  fehlschlagen. Bei Fehlern darf die Sitzung nicht durch den Duo-Ablauf freigegeben
  werden; die reguläre GNOME-Entsperrmöglichkeit bleibt bestehen.
- Zugangsdaten und persönliche Serverwerte gehören nicht in Quellcode, README,
  Logs oder öffentliche Projektarchive.

## Rückbau

GNOME-Erweiterung deaktivieren:

```bash
gnome-extensions disable proxilens-input@local
```

Benutzer-Agent stoppen und deaktivieren:

```bash
systemctl --user disable --now proxilens-wake-agent.service
```

PAM und GDM wurden von dieser Alpha-Version nicht verändert.

## Weitere Dokumentation

- [Alpha.7.3.1 – Duo-Status und Testanleitung](README-ALPHA7.3-DUO-STATE.md)
- [Alpha.7 – GNOME-Duo-Button](README-ALPHA7-DUO-BUTTON-TEST.md)
- [Sleep/Wake-Test](README-ALPHA6-TEST.md)
- [Konfiguration](README-KONFIGURATION.md)
