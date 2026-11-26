# ProxiLens Beta 10.10.3 – zentrale Duo-Verwaltung

## Ziel

Duo-Zugangsdaten werden ausschließlich in der ProxiLens-Oberfläche gepflegt. Der Root-Dienst benötigt wegen der Trennung zwischen GNOME-Schlüsselbund und root-eigenem systemd-Dienst weiterhin eine geschützte Laufzeitkopie unter `/etc/proxilens/duo.conf`. Diese Kopie wird aus der bereits gespeicherten ProxiLens-Datenbank synchronisiert; eine zweite manuelle Eingabe ist nicht mehr vorgesehen.

## Ablauf

1. Duo wie bisher in ProxiLens unter Einstellungen speichern.
2. Nach dem Build/Install einmal `./scripts/sync-duo-root.sh` als angemeldeter Benutzer ausführen. `pkexec` fordert die lokale Administratorbestätigung an.
3. Das Synchronisierungsprogramm entschlüsselt die Werte über den GNOME-Schlüsselbund im Benutzerkontext und überträgt JSON ausschließlich über stdin an den root-eigenen Importhelfer. Geheimnisse stehen weder in Argumenten noch in Logs.
4. Der Importhelfer validiert die Eingabe und schreibt atomar `/etc/proxilens/duo.conf` mit `root:root`, Modus `0600`.
5. Bei späteren Änderungen an Duo-Einstellungen `./scripts/sync-duo-root.sh` erneut ausführen.

Optional kann mit `./scripts/sync-duo-root.sh --db /pfad/zur/proxilens.db` ein abweichender Datenbankpfad verwendet werden.

## Wichtige Abgrenzung

- ProxiLens ist die einzige Verwaltungsoberfläche und Quelle der Wahrheit.
- Die root-geschützte Datei ist ein abgeleiteter Systemdienst-Cache, keine zweite manuell gepflegte Konfiguration.
- `scripts/configure-duo-root.sh` ist absichtlich deaktiviert und verweist auf den Synchronisierungsweg.
- Diese Änderung aktiviert weder PAM noch sudoers. Die produktive PAM-Aktivierung bleibt bis zu separaten Integrationstests gesperrt.
- Vorliegende Systemdienst-/PAM-Codepfade müssen vor produktiver Nutzung separat auf Authentifizierungsstatus, PAM-Kontrollfluss und Root-Peer-Prüfung auditiert werden.
