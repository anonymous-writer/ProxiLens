# ProxiLens Beta 8 – isolierter Policy-Protokoll-Prototyp

Beta 8 baut auf dem Beta-7-Sicherheitsentwurf auf. Neu ist ausschließlich das eigenständige Go-Paket `internal/policyproto` mit einem streng begrenzten Nachrichtenmodell, Peer-Identitätsabgleich und Nonce-Replay-Schutz samt Tests.

**Keine Systemintegration:** Es gibt weiterhin keinen Root-Dienst, keinen privilegierten IPC-Endpunkt, keine Duo-Verifikation und keine Änderungen an PAM, Polkit oder sudoers. Der vorhandene User-Grant-Broker bleibt eine reine Simulation und ist kein Autorisierungsdienst.

Siehe `POLICY-PROTOCOL-PROTOTYPE.md` und `SECURITY-ARCHITECTURE-BETA7.md`.

Tests auf einem System mit passender Go-Toolchain:

```sh
go test ./internal/policyproto -count=1
go test -race ./internal/policyproto -count=1
```
