# ProxiLens 10.11 – sicherheitsorientierte Integrationskorrekturen

Dieser Stand verbessert die Vorbereitung der sudo/PAM-Integration. Er ist **kein Nachweis einer freigegebenen Produktivinstallation**: PAM- und Duo-Integration müssen auf dem Zielsystem mit dessen Go-Toolchain, PAM-Bibliotheken und realem Duo-Tenant getestet werden.

## Änderungen

- Der Policy-Dienst akzeptiert nur eine root-eigene, reguläre Duo-Konfigurationsdatei mit exakt Modus `0600`; Symlinks und Datei-Austausch während des Öffnens werden abgelehnt.
- Der Socket-Pfad wird auf ein echtes root-eigenes Verzeichnis geprüft. Vorhandene fremde oder unsichere Socket-Pfade werden nicht entfernt.
- `SO_PEERCRED` wird gegen die reale UID aus `/proc`, die PAM-Identität, den sudo-Executable-Pfad sowie Prozessstartzeit und erneuten Executable-Check abgeglichen.
- Root-Synchronisierung bindet die Duo-Konfiguration an den lokalen Desktop-Benutzer, der `pkexec` autorisiert hat. Andere lokale Benutzer werden vom Policy-Dienst abgewiesen. Pro Installation ist damit zunächst genau ein lokaler sudo-Benutzer zugeordnet.
- Das PAM-Modul behandelt partielle Socket-I/O und `EINTR`, akzeptiert ausschließlich die vollständige Antwort `ALLOW\n` und bereinigt Eingabepuffer.
- Der Installer erstellt vor Änderungen eine root-geschützte Sicherung, prüft Voraussetzungen und sudoers, aktiviert PAM als letzten Schritt und rollt bei Fehlern PAM/sudoers sowie den vorherigen Service-Zustand zurück.
- Das Entfernen der Integration erstellt ebenfalls eine Sicherung und stellt sie bei fehlgeschlagener sudoers-Prüfung wieder her.
- Der Build-Hinweis führt nun zur zentralen Synchronisierung statt zum deaktivierten manuellen Konfigurationsskript.

## Ablauf auf Ubuntu

1. Voraussetzungen installieren: `sudo apt install build-essential libpam0g-dev pkg-config`.
2. Im Projektverzeichnis `./scripts/build-install-sudo-pam.sh` ausführen. Dieser Schritt baut die Go-Dienste und das PAM-Modul und installiert nur die Dateien – er ändert noch nicht `/etc/pam.d/sudo`.
3. ProxiLens als der lokale Benutzer öffnen, der sudo verwenden soll, Duo-Einstellungen speichern und `./scripts/sync-duo-root.sh` ausführen. Die Root-Kopie wird per `pkexec` übertragen; Geheimnisse stehen weder in Argumenten noch in Logs.
4. Erst nach erfolgreicher Synchronisierung `sudo ./scripts/install-sudo-pam-proxilens.sh` ausführen.
5. Ein bereits geöffnetes Root-Terminal behalten. In einem zweiten Terminal `sudo -k` und danach `sudo id` testen. Duo Push muss funktionieren; explizite Wahl `P` muss zu Ubuntu `common-auth` führen. Duo-Deny, Timeout und Netzwerkausfall müssen fehlschlagen und dürfen nicht auf Passwort zurückfallen.
6. Bei Problemen `sudo ./scripts/remove-sudo-pam-proxilens.sh` ausführen oder aus der Sicherung unter `/var/backups/` wiederherstellen.

## Wichtige Grenzen

- Der Import bindet aktuell genau einen lokalen Benutzer an das in ProxiLens gespeicherte Duo-Konto. Mehrbenutzer-Zuordnung ist noch nicht implementiert.
- Die PAM-Steuerlogik muss auf dem konkreten Ubuntu-System mit einem isolierten PAM-Testdienst geprüft werden, bevor sie produktiv aktiviert wird.
- Das Go-Modul verlangt Go 1.27. Ein Build mit einer älteren Go-Version ist nicht als kompatibel anzunehmen.
- Der Dienst nutzt Duo Push; bei jedem Verbindungs- oder Konfigurationsfehler bleibt die Authentifizierung geschlossen.
