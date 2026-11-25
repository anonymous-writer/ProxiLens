# ProxiLens 2.1.0 Beta 7 — Security Architecture Preview

Diese Beta dokumentiert die vereinbarte gemeinsame, sitzungsgebundene 15-Minuten-Freigabe und den ausdrücklich gewählten Ubuntu-Passwort-Fallback. Sie ist eine Architektur-/Spezifikationsfassung auf Basis von Beta 6.

**Wichtig:** Beta 7 implementiert noch keine echte sudo- oder Polkit-Authentifizierung und enthält keinen privilegierten Root-Dienst. Der vorhandene Grant Broker ist ausschließlich ein unprivilegierter Entwicklungssimulator. Nicht als Autorisierungsinstanz verwenden.

Siehe [`SECURITY-ARCHITECTURE-BETA7.md`](SECURITY-ARCHITECTURE-BETA7.md) für Vertrauensmodell, Abläufe, Sicherheitsanforderungen und Freigabekriterien.

Die bisherige Beta-6-Anwendung und ihre Tests bleiben unverändert. PAM, Polkit, sudoers und systemweite Dienste werden durch diese Beta nicht verändert.
