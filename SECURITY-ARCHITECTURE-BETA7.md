# ProxiLens Beta 7 — Sicherheitsarchitektur (Entwurfs- und Prüfstand)

## Status und Sicherheitsgrenze

Beta 7 legt das vereinbarte Zielverhalten und die Vertrauensgrenzen fest. Es aktiviert **keine** echte sudo- oder Polkit-Authentifizierung. Der vorhandene `grantbroker` bleibt ein unprivilegierter Entwicklungstestdienst. Sein Socket, seine Statusmeldung und seine 15-Minuten-Simulation dürfen niemals von Root, PAM, sudo oder Polkit als Berechtigungsnachweis verwendet werden.

Es gibt in dieser Fassung keinen privilegierten Authentifizierungsdienst, keine PAM-Regeländerung, keinen Polkit-Agenten und keine sudoers-Änderung. Die folgenden Abläufe sind Spezifikation, nicht bereits implementierte Systemfunktionen.

## Vereinbarte Benutzerentscheidung

1. **Eine gemeinsame 15-Minuten-Freigabe** für sudo- und Polkit-Anfragen innerhalb derselben angemeldeten Linux-Sitzung.
2. **Ubuntu-Passwort nur nach ausdrücklicher Auswahl** von „Mit Ubuntu-Passwort fortfahren“. Es gibt keine automatische Passwortabfrage als stillen Fallback.
3. **Abbrechen** beendet die konkrete Authentifizierungsanfrage und erteilt keine Berechtigung.
4. Eine Freigabe überlebt weder Abmeldung noch Neustart. Sie ist an UID, Login-Sitzung und eine serverseitig verwaltete Ablaufzeit gebunden.
5. Die gemeinsame Freigabe ist kein pauschaler Root-Zugang. Jede angeforderte Aktion muss weiterhin durch die zuständige sudo-/Polkit-Policy geprüft werden.

## Geplantes Vertrauensmodell

### A. Desktop-GUI (nicht privilegiert)

- Zeigt Duo-Status und die ausdrücklich auswählbaren Optionen.
- Darf keine erfolgreiche Duo-Prüfung behaupten oder selbst einen privilegierten Grant erzeugen.
- Darf keine Geheimnisse oder Tokens in Logs schreiben.
- Ist kein Vertrauensanker für Root.

### B. Duo-Transaktionskomponente (nicht privilegiert)

- Startet eine frische Duo-Transaktion für eine konkrete Anfrage.
- Muss das Ergebnis serverseitig aus der authentischen Duo-Antwort ableiten; UI-Text oder Client-Callback sind kein Beweis.
- Verwendet eindeutige, kurzlebige Transaktions-IDs und bindet das Ergebnis an UID, Login-Sitzung, Zweck und Ablaufzeit.
- Verhindert Replay: jede Transaktion ist einmalig, zeitlich begrenzt und nach Verbrauch ungültig.

### C. Zukünftiger privilegierter Policy-Dienst (noch nicht implementiert)

- Muss ein eigenes, root-kontrolliertes IPC-Ende besitzen. Er darf den benutzerkontrollierten Socket unter `$XDG_RUNTIME_DIR` niemals als Autorisierungseingang verwenden.
- Muss Client-Identität und Sitzung mit Betriebssysteminformationen prüfen (z. B. Peer-Credentials und Logind-Sitzungsdaten), nicht anhand frei übermittelter UID-/Sitzungsstrings vertrauen.
- Muss Duo-Nachweise kryptografisch prüfen oder selbst die Duo-Transaktion kontrollieren. Eine bloße Meldung „Duo erfolgreich“ von der GUI reicht nicht.
- Muss Nonces, kurze Gültigkeit, Replay-Schutz, Protokollversion und streng begrenzte Nachrichtengrößen verwenden.
- Muss Aktionen einzeln autorisieren und die bestehende sudo-/Polkit-Policy respektieren.
- Muss bei Fehlern geschlossen bleiben (fail closed): kein Grant bei Timeout, ungültiger Signatur, nicht passender Sitzung, abgelaufener Transaktion oder Dienstfehler.

## Gemeinsame 15-Minuten-Regel

Der zukünftige Policy-Dienst verwaltet den gemeinsamen Grant zentral. Ein erfolgreicher Duo-Nachweis kann einen Grant für maximal 15 Minuten in der aktuellen Login-Sitzung eröffnen. Die Gültigkeit endet beim frühesten dieser Ereignisse:

- 15 Minuten seit Ausstellung;
- Abmeldung oder Ende der zugehörigen Login-Sitzung;
- expliziter Widerruf;
- Neustart des Rechners bzw. Neustart des Policy-Dienstes;
- Sicherheits- oder Integritätsfehler.

Der Grant wird nicht dauerhaft gespeichert. Ein Grant darf nicht auf eine andere UID oder Sitzung übertragen werden. Der Dienst muss Ablaufzeiten mit einer monotonen Uhr messen und Statusanzeigen dürfen niemals selbst über Gültigkeit entscheiden.

## Interaktionsablauf

### sudo

1. sudo führt seine normale Policy-Prüfung aus und fordert ProxiLens-Authentifizierung nur im dafür vorgesehenen, später separat geprüften Integrationspunkt an.
2. Duo Push wird gestartet; GUI zeigt „Push gestartet“, „Warte auf Bestätigung“, „Bestätigt“, „Abgelehnt“ oder „Zeitüberschreitung“.
3. Nur ein verifizierter, frischer Duo-Nachweis kann den sitzungsgebundenen Grant eröffnen bzw. erneuern.
4. „Mit Ubuntu-Passwort fortfahren“ ist eine explizite Auswahl und führt in den normalen Ubuntu-Authentifizierungspfad zurück. Kein automatischer Wechsel.
5. „Abbrechen“ beendet die konkrete Anfrage. Keine Freigabe.

### Polkit

1. Polkit bleibt für die konkrete Aktion und deren Policy zuständig.
2. Der Agent zeigt einen echten Dialog mit „Mit Duo-Freigabe fortfahren“, „Mit Ubuntu-Passwort fortfahren“ und „Abbrechen“.
3. Passwort-Fallback wird nur nach ausdrücklicher Auswahl gestartet und muss über den von Polkit vorgesehenen Authentifizierungsweg erfolgen.
4. Keine selbstgebaute Passwortprüfung und keine Umgehung von Polkit durch einen GUI-Prozess.

## Implementierungs-Gates vor jeder Aktivierung

1. Threat Model und Protokollreview abgeschlossen.
2. Root-Dienst ohne PAM-/Polkit-Aktivierung separat gebaut und mit Peer-Credential-, Replay-, Ablauf- und Fail-closed-Tests geprüft.
3. Disposable-VM-Tests mit Snapshot und funktionierendem Konsolen-/Recovery-Zugang.
4. Automatischer Rollback vor jeder PAM-/Polkit-Änderung; Rollback muss auch bei abgebrochener Installation funktionieren.
5. Negative Tests: gefälschte GUI-Antwort, falsche UID/Sitzung, wiederverwendeter Nachweis, abgelaufener Grant, Dienstabsturz, Socket-Manipulation, parallele Anfragen und Abmeldung.
6. Erst nach bestandenen Tests getrennte, opt-in Aktivierung für sudo und Polkit. Kein stilles Aktivieren bei Paketinstallation.

## In Beta 7 ausdrücklich nicht enthalten

- Kein Root-Dienst und kein Root-IPC-Protokoll.
- Keine PAM-Module oder PAM-Konfigurationsänderungen.
- Keine Polkit-Agent- oder Policy-Änderungen.
- Keine sudoers-Änderungen.
- Keine echte Systemberechtigung durch den vorhandenen User-Broker.

## Aktueller nächster technischer Schritt

Zuerst wird das Root-Protokoll als eigenständiger, testbarer Entwurf umgesetzt und überprüft. Die bestehende Beta-6-User-Broker-Funktion bleibt davon getrennt und darf nicht als Sicherheitsbeweis für die spätere privilegierte Komponente gelten.
