# ProxiLens – isolierter Policy-Protokoll-Prototyp

Dieser Prototyp ergänzt Beta 7 um ein reines Go-Datenmodell für Anfragen an einen späteren privilegierten Policy-Dienst.

## Enthalten

- versionierte Request-Hülle mit ID, kryptografisch zufälliger Nonce und engem Zeitfenster (max. 30 Sekunden);
- strenge Feld- und Zeitvalidierung;
- Abgleich der behaupteten UID/Sitzung mit einer separat vom Betriebssystem ermittelten Peer-Identität;
- atomarer, prozesslokaler Replay-Schutz für Nonces;
- Tests für Replay, abweichende Peer-Identität, Ablauf und Feldgrenzen.

## Harte Sicherheitsgrenze

Ein erfolgreicher `ValidateRequest`-Aufruf ist **keine Berechtigung**. Er besagt nur, dass die Nachrichtenhülle formal frisch ist und die Identitätsangaben mit den vom Aufrufer gelieferten Peer-Daten übereinstimmen. Die Peer-Daten müssen später durch den Root-Dienst z. B. aus Unix-Peer-Credentials und Logind ermittelt werden. Dieses Paket ermittelt sie nicht selbst.

Nicht enthalten sind ein Root-Dienst, Socket/IPC, Duo-Verifikation, Grant-Ausstellung, Passwortprüfung, PAM-/Polkit-/sudo-Integration oder irgendeine Systemberechtigung. Das Paket darf nicht in den bestehenden User-Broker eingebaut werden, um diesen als Autorisierungsinstanz zu verwenden.

## Test

```sh
go test ./internal/policyproto -count=1
go test -race ./internal/policyproto -count=1
```

Der nächste Schritt nach Review ist ein separat gestarteter, zunächst noch nicht aktivierter Linux-Testdienst, der Peer-Credentials tatsächlich aus dem Socket liest und Sitzungen über logind verifiziert. Erst danach folgen Threat-Model-Review und VM-Tests. PAM/Polkit bleiben bis dahin unverändert.
