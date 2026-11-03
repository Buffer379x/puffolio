# Puffolio

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="assets/brand/logo-dark.svg">
  <img src="assets/brand/logo.svg" alt="Puffolio" width="210">
</picture>

Eine leichte, selbst gehostete Web-App zur Verwaltung persönlicher Vaping-Liquids.

**Projektstatus: Konzeptphase. Es gibt noch keine installierbare App und kein Release.**

Geplant sind frei definierbare Felder (einschließlich Datumsfeldern), ein konfigurierbares Dashboard, Listen- und Kachelansicht, Bilder, mehrere Benutzer sowie ein heller und dunkler Modus. Die Anwendung soll als einzelner Docker-Container mit SQLite und einem Unraid-Template bereitgestellt werden. Veröffentlichungen und Container werden über GitHub und die GitHub Container Registry verteilt.

Die lokale Planung, Designprototypen und temporäre Tests liegen in `work/`. Dieses Verzeichnis ist in Git und im Docker-Buildkontext ausgeschlossen und wird nicht veröffentlicht.

## Geplante Architektur

- Go-HTTP-Dienst mit eingebautem React-/TypeScript-Frontend.
- SQLite und Uploads in einem persistenten Datenverzeichnis.
- Docker-Images für `linux/amd64` und `linux/arm64`.
- Veröffentlichung mit Versions-Tags über GitHub Releases und GHCR.

Architektur, Funktionsumfang und Gestaltung befinden sich in Abstimmung.
