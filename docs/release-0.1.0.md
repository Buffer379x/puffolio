# Puffolio 0.1.0

Erste Version der selbst gehosteten Liquidverwaltung. Eigene Felder,
persönliche Sammlungen für mehrere Nutzer und ein konfigurierbares Dashboard
laufen gemeinsam mit SQLite in einem Container.

- Zehn Feldtypen einschließlich Datum, Sternebewertung, Bild, Tags und Farbe.
- Gemeinsame Listen-/Kachelseite mit Suche, Filtern, Favoritenfilter und
  Bearbeitung bestehender Einträge.
- Private Bilder mit manuellem 4:3-Ausschnitt und späterer Nachbearbeitung;
  Tagfarben aus einer festen Palette.
- Acht konfigurierbare Widgettypen, Hell/Dunkel/System und responsive Ansichten.
- Tools-Seite mit Nikotinrechner für Flaschenvolumen, Aroma, Shots und
  Shotstärke; berechnet Endstärke und benötigte Base.
- Geschütztes Setup, Einladungen, Nutzerverwaltung, Sammlungstransfer,
  Instanzsicherung und Offline-Restore.

## Installation

Das Image `ghcr.io/buffer379x/puffolio:0.1.0` unterstützt `linux/amd64` und
`linux/arm64`. Die Assets `compose.yaml` und `puffolio.xml` gehören exakt zu
dieser Version. `image-digests.txt` enthält die unveränderlichen Image-Digests,
`SHA256SUMS` die Prüfsummen der beigefügten Dateien. Softwareinventare und
vollständige Scanergebnisse liegen ebenfalls bei.

Anleitung: [Installation](https://github.com/Buffer379x/puffolio/blob/v0.1.0/docs/installation.md).
Nach dem Start den einmaligen Setup-Code aus `/data/setup-code` lesen und ein
eigenes Administratorkonto anlegen. Das Datenverzeichnis muss lokal und für
UID99/GID100 beschreibbar sein. Bei externem Zugriff Basis-URL konfigurieren;
bei HTTPS sichere Cookies aktivieren.

## Betriebsgrenzen

SQLite benötigt lokalen Speicher und einen einzigen schreibenden
Puffolio-Prozess. Die App wird an einer eigenen Origin betrieben; Unterpfade,
SMB-/NFS-Datenbanken und öffentliche Selbstregistrierung werden nicht
unterstützt. Bilder dürfen maximal 8 MiB, 24 Megapixel und 65.535 Pixel pro Seite
haben; auch 16-Bit- und interlaced PNG bleiben unterstützt. Die App setzt ohne
explizites `GOMEMLIMIT` ein Go-Heap-Softlimit von 384 MiB und erhält strengere
Laufzeitlimits. Ein expliziter Operatorwert hat Vorrang. Der Webimport
ist auf 100 MiB entpackte Daten, 10.000 Archiveinträge und 5.000 Bilder begrenzt;
größere eigene Exporte können mit ausdrücklich gesetzten CLI-Grenzen
übernommen werden. Pflichtregeln und gemeinsame Sammlungsgrenzen können einen
additiven Import vollständig ablehnen.

Das Unraid-Template wird mitgeliefert. Ein Test auf einem echten Unraid-Host
steht noch aus; Container- und XML-Prüfungen ersetzen diesen nicht. Die erste
Version bietet keine automatische Sicherungsplanung und keinen frei
zuordenbaren CSV-Import. Sicherungen sind unverschlüsselt und müssen extern
geschützt aufbewahrt werden.

Details: [Sicherung und Wiederherstellung](https://github.com/Buffer379x/puffolio/blob/v0.1.0/docs/backup-restore.md).
