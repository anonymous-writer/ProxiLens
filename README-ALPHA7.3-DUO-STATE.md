# ProxiLens 0.0.2 Alpha.7.3.1 – Duo-Status und Entsperrung

Diese Version bereinigt die Statusmeldungen aus Alpha.7.3 und aktualisiert die
Dokumentation anhand des erfolgreichen Gerätetests.

## Bestätigtes Verhalten auf dem Zielgerät

- GNOME Shell 50.1 unter Wayland.
- Der Button **„Mit Duo entsperren“** wird im GNOME-Sperrbildschirm angezeigt.
- Der Button-Klick löst den Authelia-/Duo-Ablauf aus.
- Nach erfolgreicher Duo-Bestätigung wird die gesperrte Sitzung über
  `loginctl unlock-session` freigegeben.
- Laut Test wird danach **kein Ubuntu-Passwort** mehr abgefragt.
- Sleep/Wake bleibt als separater Ablauf enthalten.

Dies ist ein Testbericht für das konkrete Zielgerät, keine Garantie für andere
GNOME-Versionen oder Installationen.

## Zustandsfolge

- `LOCKED`: GNOME meldet `unlock-dialog`.
- `PENDING`: Authentifizierung wurde gestartet.
- `SUCCESS`: Authelia hat den Duo-Zweitfaktor bestätigt.
- `UNLOCK_REQUESTED`: Der Agent hat die Sitzung erneut geprüft und fordert die
  Freigabe an.
- `UNLOCKED`: GNOME hat den Wechsel in den normalen `user`-Modus gemeldet.
- `FAILED`: Authentifizierung oder Freigabe ist fehlgeschlagen.

`UNLOCKED` wird nicht mehr vorschnell direkt nach dem Aufruf von
`loginctl unlock-session` gemeldet. Der Status wird erst gesetzt, wenn die
GNOME-Shell den Wechsel aus `unlock-dialog` meldet.

## Bereinigung doppelter Meldungen in Alpha.7.3.1

Die GNOME-Modusüberwachung und der Authentifizierungsablauf können denselben
Zustandswechsel nahezu gleichzeitig beobachten. Der Agent verwaltet deshalb
jetzt einen zentralen letzten Duo-Zustand. Ein unveränderter Zustand wird weder
erneut protokolliert noch erneut über `DuoAuthStateChanged` gesendet.

Außerdem wurden redundante zusätzliche Logzeilen nach `SUCCESS` und
`UNLOCK_REQUESTED` entfernt. Die Zustandsmeldung selbst ist jetzt die einzige
Quelle für die entsprechende `Duo-Zustand:`-Logzeile.

## Installation

Vorherige Version als Rückfallkopie behalten. ZIP entpacken und im Projektordner:

```bash
bash deploy/install-wake-agent.sh
bash deploy/install-gnome-extension.sh
systemctl --user restart proxilens-wake-agent.service
```

## Test

```bash
journalctl --user -u proxilens-wake-agent.service -f
```

Dann `Super+L`, „Mit Duo entsperren“ anklicken und den Push bestätigen.

Erwartete `Duo-Zustand:`-Zeilen (jeweils genau einmal pro Übergang):

```text
Duo-Zustand: LOCKED
Duo-Zustand: DUO_PENDING
Duo-Zustand: DUO_SUCCESS
Duo-Zustand: UNLOCK_REQUESTED
Duo-Zustand: UNLOCKED
```

Nach erneutem Sperren beginnt ein neuer Zyklus mit `LOCKED`.

## Sicherheit und Grenzen

PAM und GDM werden nicht verändert. Die Funktion ist für eine bereits
angemeldete und gesperrte Wayland-Benutzersitzung gedacht. Der Agent führt vor
der Authentifizierung und unmittelbar vor der Freigabe eine logind-Prüfung
(`LockedHint`) durch. GNOME-Statussignale sind zusätzliche Statusinformationen,
keine Authentifizierung.

Der Agent läuft im Benutzerkontext und nutzt den Session-D-Bus. Diese
experimentelle Konstruktion ist nicht mit einem PAM-Modul oder einem
systemweiten Authentifizierungsbroker gleichzusetzen. Für produktiven Einsatz
ist eine separate Sicherheitsprüfung erforderlich.
