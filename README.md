# Puffolio

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="assets/brand/logo-dark.svg">
  <img src="assets/brand/logo.svg" alt="Puffolio" width="210">
</picture>

Selbst gehostete Web-App zum Verwalten und Zeigen einer Vaping-Liquid-Sammlung. Ein
Go-Prozess liefert Oberfläche und API aus; SQLite und Bilder liegen in einem
persistenten Datenverzeichnis. Kein zusätzlicher Datenbankserver.

- Eigene Felder: Text, langer Text, Sternebewertung, Bild, Tags, Farbe, Datum,
  Zahl, Auswahl und Ja/Nein.
- Konfigurierbares Dashboard, gemeinsame Listen-/Kachelseite, Suche, Filter
  einschließlich Favoriten und editierbare Einträge.
- Bilder mit manuellem quadratischem Ausschnitt, grauer Außenmaske und späterer
  Nachbearbeitung; Tagfarben aus einer wählbaren Palette.
- Ein Administrator pflegt die Sammlung; alle anderen können sie ohne Anmeldung
  ansehen, durchsuchen und sortieren, aber nichts ändern.
- Haltbarkeitsbalken je Liquid: Standardhaltedauer in den Einstellungen, je Liquid
  überschreibbar, läuft ab Kaufdatum von Grün (voll) nach Rot (leer).
- Heller, dunkler und Systemmodus; responsive Oberfläche mit gemeinsamen
  Gestaltungsregeln.
- Tools-Seite mit Nikotinrechner: Endfüllmenge, Aroma, Shotvolumen und Shotstärke in mg/ml;
  Ausgabe der Nikotinstärke und benötigten Base.
- Sammlungsexport/-import und konsistente Instanzsicherung mit Offline-Restore.

## Ansicht

| Liquids | Dashboard |
| --- | --- |
| ![Liquids als Kacheln](docs/screenshots/liquids-kacheln.png) | ![Dashboard](docs/screenshots/dashboard.png) |
| ![Liste, sortiert nach Haltbarkeit](docs/screenshots/liquids-liste.png) | ![Dashboard im hellen Modus](docs/screenshots/dashboard-hell.png) |
| ![Einstellungen mit Feldmatrix und Haltbarkeit](docs/screenshots/einstellungen-felder.png) | ![Nikotinrechner](docs/screenshots/nikotinrechner.png) |

<img src="docs/screenshots/mobil-liquids.png" alt="Liquids auf dem Smartphone" width="260"> <img src="docs/screenshots/mobil-dashboard.png" alt="Dashboard auf dem Smartphone" width="260">

Die Bilder zeigen die echte App mit erfundenen Beispieldaten. Sie entstehen mit
`pnpm --dir web screenshots`.

## Installation

Die [GitHub Releases](https://github.com/Buffer379x/puffolio/releases) enthalten
Installationsdateien. Das fertige Image liegt in GitHubs Container Registry:
`ghcr.io/buffer379x/puffolio:0.2.0` für `linux/amd64` und `linux/arm64`.

```sh
mkdir puffolio
cd puffolio
curl -fsSLO https://github.com/Buffer379x/puffolio/releases/download/v0.2.0/compose.yaml
mkdir data
sudo chown 99:100 data
sudo chmod 700 data
docker compose -f compose.yaml up -d
docker logs puffolio 2>&1 | grep "setup code"
```

Im Browser `http://localhost:8787` oder die Server-IP öffnen und mit dem
Setup-Code das erste Administratorkonto anlegen. Es gibt keine
Standardzugangsdaten. Für HTTPS `PUFFOLIO_COOKIE_SECURE=true` verwenden.
[Vollständige Installation](docs/installation.md).

Die Sammlung ist für alle lesbar, die die Adresse erreichen; ändern kann nur der
angemeldete Administrator.

Für Unraid das Releaseasset
[puffolio.xml](https://github.com/Buffer379x/puffolio/releases/download/v0.2.0/puffolio.xml)
verwenden. Das Template setzt Benutzer99:100, schreibgeschütztes Root-Dateisystem
und den persistenten Appdata-Pfad. Der Container führt kein `chown` beim Start
als Root aus. Ein Test auf einem echten Unraid-Host wird gesondert ausgewiesen;
XML-/Containerprüfungen allein belegen ihn nicht.

SQLite-WAL benötigt lokalen Speicher. Kein SMB-/NFS-Mount für die aktive
Datenbank und niemals mehrere schreibende Instanzen auf demselben Verzeichnis.

## Betrieb

Bilder: JPEG, PNG oder WebP, höchstens 8 MiB, 24 Megapixel und 65.535 Pixel
pro Seite. Auch 16-Bit- und interlaced PNG werden unterstützt. Die App verwendet
ohne explizites `GOMEMLIMIT` ein Go-Heap-Softlimit von 384 MiB; ein bereits
strengeres Laufzeitlimit bleibt erhalten. Details in der [Installation](docs/installation.md).

- [Sicherung, Wiederherstellung und Updates](docs/backup-restore.md)
- [API-Vertrag](docs/api/openapi.yaml)
- [Gestaltungsregeln](docs/design.md)
- [Release erstellen](docs/releasing.md)
- [Sicherheit](SECURITY.md) und [Änderungen](CHANGELOG.md)

Bei einem Downgrade das passende alte Image **und** dessen Backup verwenden.
Ein GitHub-Quellcode-ZIP ersetzt kein installierbares Containerimage.

## Entwicklung

Go1.27.3, Node24.22.2 und pnpm12.16.1. Abhängigkeiten sind festgeschrieben.

```sh
pnpm --dir web install --frozen-lockfile
pnpm --dir web test
pnpm --dir web build
go test ./... -count=1
go vet ./...
go build -o work/bin/puffolio ./cmd/puffolio
pnpm --dir web demo 8787
```

`demo` startet die echte App mit Beispieldaten in einem temporären Verzeichnis und ist die
Vorschau für jede Änderung an der Oberfläche. Das Frontend muss vor dem Go-Build gebaut werden, weil `web/dist` in das Binary
eingebettet wird. CI prüft zusätzlich Browserabläufe, Container und
Wiederherstellung. `work/` enthält ausschließlich lokale Konzepte, temporäre
Umsetzung, Laufdaten und Berichte. Es wird weder versioniert noch in Images
oder Releases aufgenommen. Reproduzierbare Produkttests selbst werden
versioniert, damit sie in CI laufen.

Lizenz: [MIT](LICENSE).
