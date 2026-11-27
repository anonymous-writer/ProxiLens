# ProxiLens deinstallieren

Die ZIP-Version enthält zwei unterschiedliche Installationswege. Die Rücknahme
muss zum tatsächlich verwendeten Weg passen.

## Manuelle Desktop-Installation (`scripts/install-desktop-app.sh`)

Im entpackten Projektverzeichnis zuerst die geplanten Änderungen ansehen:

```bash
./scripts/uninstall-desktop-app.sh --dry-run
```

Dann die Desktop-Anwendung entfernen:

```bash
./scripts/uninstall-desktop-app.sh
```

Das entfernt den von diesem Installer angelegten Launcher, das Programm unter
`~/.local/bin/` und das ProxiLens-App-Icon. Die Benutzerdaten bleiben erhalten.

Für das zusätzliche Löschen des Standard-Konfigurations- und Datenbankordners:

```bash
./scripts/uninstall-desktop-app.sh --purge-data
```

`--purge-data` löscht nur `XDG_CONFIG_HOME/proxilens` (standardmäßig
`~/.config/proxilens`). Eine in den Einstellungen auf einen anderen Ort
verschobene SQLite-Datenbank wird nicht automatisch gefunden oder gelöscht.
Der GNOME-Schlüsselbund-Eintrag wird absichtlich nicht automatisch entfernt,
weil das Skript nicht sicher feststellen kann, ob er noch von anderen
ProxiLens-Installationen oder Datenbanken benötigt wird.

## Debian-/Ubuntu-Paket

Wenn ProxiLens mit dem Debian-Paket installiert wurde, den Paketmanager
verwenden:

```bash
sudo apt remove proxilens
```

Das entfernt die Programmdateien des Pakets. Für das Entfernen paketverwalteter
Konfigurationsdateien:

```bash
sudo apt purge proxilens
```

Benutzerdaten und Einträge im GNOME-Schlüsselbund werden dadurch nicht
automatisch gelöscht. Das ist beabsichtigt, damit ein Paket-Remove nicht
ungefragt persönliche Daten vernichtet.

## PAM, sudoers und Polkit

Die Projekt-Dokumentation beschreibt den normalen Desktop-/Debian-Build als
nicht aktivierend: Er installiert keine PAM-, sudoers- oder Polkit-Änderungen.
Im Quellbaum liegen jedoch separate experimentelle Installationsskripte für
solche Integrationen. Falls diese manuell ausgeführt wurden, entfernt das
Desktop-Deinstallationsskript sie ausdrücklich **nicht**.

Die Rücknahme muss gezielt und einzeln erfolgen, z. B.:

```bash
sudo ./scripts/remove-sudo-pam-proxilens.sh
sudo ./scripts/remove-sudoers-proxilens.sh
sudo ./scripts/remove-polkit-pam-proxilens.sh
```

Nur die Befehle ausführen, deren jeweilige Integration tatsächlich zuvor
installiert wurde. Vorher ein zweites administratives Terminal offen halten
und die Sicherungen/Prüfhinweise der jeweiligen Skripte beachten. Nicht
pauschal `/etc/pam.d`, `/etc/sudoers*`, `/etc/polkit-1` oder systemweite
Netzwerk-/Firewall-Konfigurationen löschen.

## Was „wie vorher“ hier bedeutet

Die manuelle Deinstallation nimmt die Änderungen des Desktop-Installers zurück.
Sie stellt nicht automatisch beliebige Änderungen des Betriebssystems wieder
her. Der normale Debian-Paketbuild dieses Archivs installiert laut Paketregeln
nur die Desktop-App, Launcher, Icon und Metadaten sowie den User-Service.
Systemweite Authentifizierungsintegrationen sind davon getrennt.
