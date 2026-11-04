# Installation

Puffolio läuft als einzelner Container mit SQLite. Images werden fertig für
`linux/amd64` und `linux/arm64` veröffentlicht; auf dem Zielserver ist kein
Quellcodebuild nötig. Installationsdateien gehören immer zur selben Version.

## Docker Compose

Die Datei `compose.yaml` aus dem GitHub Release in ein eigenes Verzeichnis
herunterladen. Das Datenverzeichnis vor dem Start anlegen:

```sh
mkdir -p data
sudo chown 99:100 data
sudo chmod 700 data
docker compose -f compose.yaml up -d
```

Weboberfläche: `http://localhost:8787` oder die Adresse des Servers, zum Beispiel
`http://192.168.1.10:8787`. Es muss keine Adresse vorab eingestellt werden. Kein
Unterpfadbetrieb. Den einmaligen Setup-Code zeigt das Container-Log:

```sh
docker logs puffolio 2>&1 | grep "setup code"
```

Er steht außerdem in `data/setup-code`.

Den Code im Setup eingeben und das Administratorkonto anlegen. Den Setup-Code
nicht weitergeben. Es gibt kein Standardpasswort und keine Registrierung.

**Sichtbarkeit:** Die Sammlung ist für alle lesbar, die die Adresse erreichen;
bearbeiten kann nur das angemeldete Administratorkonto. Wer das nicht möchte,
stellt Puffolio nicht öffentlich bereit oder setzt einen Zugriffsschutz davor.

## Unraid

Das Releaseasset `puffolio.xml` in die eigenen Docker-Templates übernehmen,
beispielsweise auf dem Bootmedium unter
`/boot/config/plugins/dockerMan/templates-user/puffolio.xml`. Anschließend in
Unraid über „Add Container“ das Template Puffolio wählen. Appdata auf lokalem
Pool-/Array-Speicher einstellen. Der Setup-Code steht im Log des Containers.

Vor dem Start das Appdata-Verzeichnis für UID99/GID100 beschreibbar machen:

```sh
mkdir -p /mnt/user/appdata/puffolio
chown 99:100 /mnt/user/appdata/puffolio
chmod 700 /mnt/user/appdata/puffolio
```

Der Container startet unprivilegiert, ohne Fähigkeiten und ohne Docker-Socket.
Das Template setzt UID99/GID100; es gibt keinen Root-Start für ein rekursives
`chown`. Die Shell-/Konsolenfunktion des Containers steht nicht zur Verfügung.
Das Template wird mitgeliefert; eine Aufnahme in Community Applications ist
ein eigener Vorgang. Ein Test auf echtem Unraid wird im Releaseprüfbericht
separat ausgewiesen.

## Konfiguration

| Variable | Vorgabe / Bedeutung |
| --- | --- |
| `PUFFOLIO_DATA_DIR` | `/data`, persistente Datenbank, Medien und Sicherungen |
| `PUFFOLIO_HTTP_ADDR` | `:8080`, HTTP-Adresse im Container |
| `PUFFOLIO_BASE_URL` | Leer: die aufgerufene Adresse wird akzeptiert. Gesetzt, z. B. `https://puffolio.example`: ausschließlich diese Origin |
| `PUFFOLIO_COOKIE_SECURE` | `false` für direktes lokales HTTP, `true` bei HTTPS |
| `TZ` | `Europe/Berlin`, Prozesszeitzone; die persönliche Zeitzone wird getrennt im Profil gespeichert |
| `GOMEMLIMIT` | Standardvariable der Go-Laufzeit; ohne expliziten Wert setzt die App ein Softlimit von 384 MiB und erhält ein bereits strengeres Laufzeitlimit |

Bei einem HTTPS-Reverse-Proxy sichere Cookies aktivieren und den `Host`-Header
unverändert weitergeben; alternativ Basis-URL auf die externe HTTPS-Origin setzen. Der Proxy muss Anfragen an Port8080
weitergeben. Die App benötigt keine offenen CORS-Freigaben. Sie wertet keine
beliebigen vom Client gelieferten Proxyheader als Vertrauensnachweis aus.

Das Heap-Softlimit gilt für Server und CLI und lässt beim vorgesehenen
512-MiB-Container Platz für Speicher außerhalb des Go-Heaps. Es ist keine harte
RSS-Grenze. Ein explizites, nicht leeres `GOMEMLIMIT` (auch `off` oder ein höherer
Wert) hat Vorrang; für solche Änderungen gilt keine 512-MiB-Speicherzusage.

## Bildgrenzen

Uploads unterstützen JPEG, PNG und WebP einschließlich 16-Bit- und
interlaced PNG. Pro Datei gelten höchstens 8 MiB, 24 Megapixel und 65.535 Pixel
auf jeder Quellachse. Überschreitungen werden mit HTTP413 zurückgewiesen; die
Achsengrenze wird vor der vollständigen Dekodierung geprüft. Sie begrenzt auch
den Speicherbedarf der PNG-Zeilenpuffer bei extrem schmalen oder breiten Bildern.
Große Bilder werden stufenweise proportional verkleinert, abschließend mit
Catmull-Rom auf maximal 1.600 Pixel lange Kante; Thumbnails bleiben bei 480 Pixeln.
Bereits gespeicherte Medienrevisionen werden dabei nicht neu berechnet.

## Datenspeicher und Updates

Die aktive SQLite-Datenbank muss auf lokalem Speicher liegen. Kein SMB-/NFS-
Remote-Mount. Nur eine Puffolio-Instanz darf dasselbe Datenverzeichnis nutzen.
Das gesamte `/data` ist persistent. Eine isolierte Kopie der geöffneten
Datenbankdatei ersetzt keine konsistente Sicherung.

Vor Updates eine Instanzsicherung erstellen und extern sichern. Danach die
gewünschte konkrete Imageversion in Compose bzw. Unraid wählen und den
Container mit demselben Datenverzeichnis neu erstellen. Readiness, Anmeldung,
ein Datum und ein Bild prüfen. Downgrade nur mit passendem Backup der alten
Version. Siehe [Sicherung und Wiederherstellung](backup-restore.md).

## Diagnose

```sh
docker compose -f compose.yaml ps
docker compose -f compose.yaml logs --tail=50
docker exec puffolio /puffolio healthcheck
```

`/healthz` prüft den Prozess, `/readyz` die verfügbare Datenbank. Bei fehlenden
Schreibrechten, einem neueren unbekannten Schema oder einem bereits belegten
Datenverzeichnis bricht der Start mit einer konkreten Meldung ab. Niemals das
bestehende Datenverzeichnis löschen, um einen Startfehler zu umgehen.
