# Änderungen

## 0.2.0 – 2026-11-04

**Geändertes Modell**

- Ein Administrator pflegt die Sammlung; alle anderen können ohne Anmeldung ansehen,
  suchen, filtern und sortieren. Einladungen und Nutzerverwaltung entfallen. Weitere
  Konten aus 0.1.0 können sich nicht mehr anmelden, ihre Daten bleiben erhalten.
- Haltbarkeitsbalken statt Füllmenge: Standardhaltedauer in den Einstellungen, je
  Liquid überschreibbar, ab Kaufdatum von Grün nach Rot. Aufgebrauchte Liquids haben
  keinen Balken. Neue Widgets „Abgelaufen“ und „Läuft bald ab“.
- Bilder sind quadratisch, die Kacheln dadurch kompakter. 4:3-Bilder und Exporte aus
  0.1.0 bleiben nutzbar und werden beim nächsten Zuschneiden zum Quadrat.
- Verwaltung ist in den Einstellungen aufgegangen (Daten).
- Nikotinrechner rechnet ausschließlich in mg/ml.

**Behoben**

- Setup und Anmeldung schlugen mit „Herkunft nicht erlaubt“ fehl, wenn die App über
  die Server-IP statt über `localhost` aufgerufen wurde. `PUFFOLIO_BASE_URL` ist nun
  optional.
- Neu angelegte Felder erschienen nicht in Liste und Kacheln.
- Tabellenaktionen lagen außerhalb des sichtbaren Bereichs.
- Datums-Widgets veralteten durch feste Zeiträume; neu sind mitlaufende Zeiträume.
- Ein langsamer Upload oder Download blockierte alle anderen Schreibvorgänge.
- Bilder wurden bei jedem Aufruf in voller Größe neu geladen; nun Thumbnails und 304.
- Namen und Textfelder sortierten Umlaute ans Ende und Großschreibung vor Kleinschreibung.

**Oberfläche**

- Neu aufgebaut nach dem abgestimmten Entwurf: Karten mit Bild und Favoriten-Herz,
  Flaschen-Platzhalter, Tag-Filter als Chips, sortierbare Spalten, Feldmatrix,
  Dashboard mit Kennzahlkarten, Rückgängig nach dem Löschen, zwei Karten pro Reihe
  auf dem Smartphone.
- Der Setup-Code steht im Container-Log.
- `pnpm demo` startet die echte App mit Beispieldaten als Vorschau.

## 0.1.0 – 2026-11-03

Erste Produktversion von Puffolio.

- Persönliche Sammlungen, geschütztes Setup, Anmeldung und Nutzerverwaltung.
- Konfigurierbare typisierte Felder einschließlich Datum, Bewertung, Bild,
  Tags und Farbe; bearbeiten, umordnen und archivieren.
- Liquidverwaltung mit gemeinsamer Listen-/Kachelansicht, Favoritenfilter,
  Suche, Sortierung, Bearbeitungskonflikten und Papierkorb.
- Private Bildverarbeitung und manuell nachbearbeitbare 4:3-Ausschnitte.
- Konfigurierbares Dashboard und Nikotinrechner auf der Tools-Seite.
- Gemeinsames responsives Design, Hell/Dunkel/System und aufrufbare Hilfen.
- Sammlungsexport/-import, Instanzsicherung und Offline-Wiederherstellung.
- Einzelcontainer mit SQLite, Multiarch-Images, Compose und Unraid-Template.

Betriebsprüfungen und bekannte Grenzen sind in den Releasehinweisen dokumentiert.
