# Direkte Duo-Push-Authentifizierung

ProxiLens verwendet ausschließlich die direkte Duo Auth API.

## Einrichtung

In den Einstellungen `Direkt mit Duo Push` wählen und IKEY, SKEY, API Hostname sowie Duo-Benutzername eintragen. Die Zugangsdaten werden in SQLite verschlüsselt; der AES-Schlüssel liegt im GNOME-Schlüsselbund. Der Secret Key wird nicht protokolliert.

`Verbindung prüfen` ruft `/auth/v2/check` auf und sendet keine Push-Nachricht.

Beim expliziten GNOME-Entsperrbutton bzw. beim vorhandenen Resume-Ablauf sendet der Agent `/auth/v2/auth` mit `factor=push`, `device=auto` und `async=1`, fragt `/auth/v2/auth_status` bis zu 90 Sekunden ab und akzeptiert ausschließlich `allow`. Danach prüft der Agent erneut, ob dieselbe GNOME-Sitzung weiterhin gesperrt ist, bevor er den vorhandenen Unlock-Pfad aufruft.

Der Duo Secret Key gehört ausschließlich in die lokale Anwendung. Niemals in Git, CI-Variablen für einen normalen Paketbuild oder Logs eintragen.

## Testhinweise

- Erst `Verbindung prüfen` verwenden.
- Danach den GNOME-Button testen und die Duo-Mobile-Anfrage bestätigen/ablehnen.
- `go test ./...` und den manuellen AMD64-DEB-Workflow ausführen.
- Vor produktivem Einsatz die Duo-API-Integration und die Desktop-Unlock-Semantik auf der Ziel-GNOME-Version prüfen.


## Signierung

Die Duo-Anbindung verwendet die von Duo empfohlene v5-Signierung (HMAC-SHA512). POST-Anfragen werden als kompaktes JSON gesendet; GET-Parameter werden RFC-3986-konform in der URL kodiert. Dies entspricht dem aktuellen Standard der offiziellen Duo-API-Clients. Die ältere v2/HMAC-SHA1-Signierung wird nicht mehr verwendet.
