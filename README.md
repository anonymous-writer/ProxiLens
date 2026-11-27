# ProxiLens

**ProxiLens 2.1.0 Beta – Duo-geschützte Systemauthentifizierung für sudo und Polkit sowie GNOME-Wake-Integration.**

Die Debian-Paketdatei stellt die GUI, das PAM-Modul, den Policy-Dienst, den Wake-Agenten, die GNOME-Erweiterung und die Integrationsskripte bereit. Die privilegierten Integrationen werden **nicht automatisch durch die Paketinstallation aktiviert**. Sie werden einzeln und bewusst über die ProxiLens-GUI installiert bzw. entfernt.

## Funktionen

- Duo Push als Standard für Systemauthentifizierung.
- Ubuntu-Passwort nur nach ausdrücklicher manueller Auswahl; kein automatischer Fallback bei Duo-Fehlern.
- Separate Aktivierung und Entfernung der sudo-PAM- und Polkit-PAM-Integration.
- Root-Policy-Dienst und geschützte Übertragung der Duo-Konfiguration.
- GNOME-Erweiterung für die GNOME-Sperrbildschirm-Integration.
- Benutzerbezogener ProxiLens Wake-Agent als systemd-User-Service.
- Deutsch, Englisch und Spanisch.

## Debian-Paket

Das Paket enthält insbesondere:

- `/usr/bin/proxilens-ui`
- `/usr/bin/proxilens-duo-sync`
- `/usr/libexec/proxilens/proxilens-policy`
- `/usr/libexec/proxilens/proxilens-wake-agent`
- `/usr/libexec/proxilens/scripts/`
- `/usr/libexec/proxilens/gnome-extension/`
- `/usr/lib/<multiarch>/security/pam_proxilens.so`
- `/usr/lib/systemd/system/proxilens-policy.service`

Ubuntu 24.04 und 26.04 verwenden die aufgeteilten Polkit-Binärpakete `polkitd` und `pkexec`; die DEB-Abhängigkeiten verwenden diese Paketnamen und nicht `policykit-1`.

## Installation und Aktivierung

Die DEB-Datei stellt die Komponenten bereit. Nach der Installation:

1. ProxiLens öffnen.
2. Duo-Zugangsdaten in der GUI speichern.
3. Über **Duo-Konfiguration für sudo/Polkit übernehmen** die Konfiguration geschützt an den Root-Dienst übertragen.
4. sudo und Polkit bei Bedarf separat über die GUI aktivieren.
5. GNOME-Erweiterung und Wake-Agent separat über die GUI installieren.

Die PAM-Installationsskripte sichern vorhandene PAM-Dateien vor Änderungen. Die Paketentfernung wird abgebrochen, solange `/etc/pam.d/sudo` oder `/etc/pam.d/polkit-1` noch `pam_proxilens.so` referenziert. Zuerst müssen die Integrationen über die GUI entfernt werden.

**Wichtig:** Das Systemarchiv mit `/etc/proxilens/duo.conf` enthält vertrauliche Duo-Zugangsdaten. Nicht veröffentlichen oder unverschlüsselt weitergeben.

## Build und CI

Der Debian-Build kompiliert GUI, Policy-Dienst, Wake-Agent, Duo-Synchronisierer und PAM-Modul. GitHub Actions prüft nach dem Paketbau zusätzlich, ob die erwarteten Binärdateien, PAM-Module, GNOME-Dateien und Installationsskripte tatsächlich im DEB enthalten sind und ob die Polkit-Abhängigkeiten `polkitd` und `pkexec` gesetzt sind.

```sh
go test ./...
```

Für den Debian-Build siehe `debian/rules` und `.github/workflows/test-deb-amd64.yml`.

## Sicherheit und Wiederherstellung

- sudo und Polkit werden getrennt verwaltet.
- Duo-Fehler führen nicht zu einem automatischen Passwort-Fallback.
- Die Paketinstallation überschreibt keine vorhandene Duo-Konfiguration.
- Das Entfernen des DEB-Pakets wird blockiert, solange PAM noch das ProxiLens-Modul verwendet.
- Vor Systemauthentifizierungsänderungen ein zweites administratives Terminal offen halten und die erzeugten Backups aufbewahren.

## Historische Entwicklungsnotizen

Die älteren `BETA*`- und `POLICY-SERVICE-*`-Dokumente beschreiben frühere Entwicklungsstände und sind nicht als Beschreibung des aktuellen 2.1.0-Beta-Paketumfangs zu verstehen.
