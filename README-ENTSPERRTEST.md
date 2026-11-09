# ProxiLens – einmaliger, manuell scharfgeschalteter Entsperrtest

Diese Version behält die OIDC-Prüfung bei und ergänzt eine standardmäßig
nicht aktivierte Einmalfreigabe für einen Entsperrtest.

## Sicherheitsablauf

1. ProxiLens starten und zunächst **keine** Entsperrfreigabe aktivieren.
2. Für einen gezielten Test das Kontrollkästchen „Einmaligen Entsperrtest … scharf schalten“ aktivieren.
3. Den Device Authorization Flow starten und den Code in Authelia bestätigen.
4. Nur wenn ID-Token-Signatur, Issuer, Audience, Ablaufzeit, UserInfo-`sub` und
   `authelia.pam.username == <Authelia-Benutzername>` erfolgreich geprüft wurden, wird der
   einmalige Entsperrversuch ausgeführt.
5. Die Einmalfreigabe wird nach dem Flow verbraucht und automatisch deaktiviert.

Der Entsperrversuch wird nur für die aktuelle, aktive Sitzung der Klasse `user`
und des Typs `wayland` ausgeführt, wenn `LockedHint=yes` gemeldet wird. Ist die
Sitzung nicht gesperrt, wird kein Entsperrbefehl ausgeführt. Es werden keine
PAM-, GDM- oder GNOME-Konfigurationen geändert.

## Build auf Ubuntu

```bash
gofmt -w internal/session/session.go internal/ui/dashboard.go
go test ./...
go build -o proxilens-ui ./cmd/proxilens-ui
```

## Hinweis zum Testablauf

Die Einmalfreigabe vor dem Sperren der Sitzung aktivieren und den Device Flow
starten. Anschließend die Sitzung manuell sperren und den Authelia-Flow auf
einem anderen Gerät bestätigen. ProxiLens muss währenddessen weiterlaufen.

Wenn die Sitzung während der Bestätigung nicht als gesperrt erkannt wird,
bricht ProxiLens sicher ab und führt keinen Entsperrbefehl aus.


## Zusätzliche Sitzungsbindung

ProxiLens berücksichtigt ausschließlich genau eine aktive Wayland-Sitzung der Klasse `user` mit einem zugeordneten Seat und passendem lokalen Benutzernamen. Manager-/Hintergrundsitzungen werden ignoriert. Bei null oder mehreren passenden Sitzungen wird sicher abgebrochen.
