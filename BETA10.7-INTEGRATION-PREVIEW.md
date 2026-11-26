# ProxiLens 2.1.0 Beta 10.7 – Sicherheitsgrenzen-Prototyp

## Neu in Beta 10.7

- `policygrant.Store.Consume` prüft und entfernt eine aktionsgebundene Freigabe atomar. Ein zweiter oder paralleler Aufruf kann dieselbe Freigabe nicht erneut verwenden.
- Linux `policyproto.ReadKernelPeerCredentials` liest PID, UID und GID über `SO_PEERCRED` direkt vom angenommenen Unix-Domain-Socket. Die Funktion ist für die serverseitige Verbindung vorgesehen.
- Tests prüfen Einmalverwendung, parallelen Zugriff und die vom Kernel gelieferten Peer-Credentials.

## Wichtige Identitätsgrenze

`SO_PEERCRED` liefert Kernel-Peer-Credentials, aber keine logind-Session-ID. `Request.SessionID` bleibt eine nicht vertrauenswürdige Behauptung. Ein späterer privilegierter Dienst muss PID/UID unabhängig einer aktuellen logind-Sitzung zuordnen, diese Sitzungs-ID selbst ermitteln und erst dann mit der Anfrage vergleichen. Umgebungsvariablen und GUI-Angaben sind dafür keine vertrauenswürdige Quelle.

## Nicht enthalten

- Kein privilegierter IPC-Dienst und keine Integration mit sudo, PAM oder Polkit
- Keine produktive Systemautorisierung und keine privilegierten Aktionen
- Keine Änderungen an `/etc/pam.d`, sudoers, Polkit, systemd oder Desktop-Konfiguration
- Die Freigabe bleibt ein In-Memory-Prototyp und ist keine produktive Sicherheitsgrenze

## Tests

Im Projektverzeichnis:

```bash
go test -race ./...
go build -trimpath -o proxilens-ui .
```
