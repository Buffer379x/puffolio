# Sicherung und Wiederherstellung

## Eigene Sammlung

Unter Einstellungen → Daten lässt sich die persönliche Sammlung als
Puffolio-ZIP exportieren. Sie enthält Felddefinitionen, Werte, Tags und deren
Palettenfarben, bereinigte Bildmaster und Ausschnitte sowie gespeicherte
Ansichten, Dashboard und eigene Darstellungspräferenzen. Zugangsdaten und
fremde Sammlungen gehören nicht zum Sammlungsexport.

In der Listenansicht der Liquids lassen sich ausgewählte Liquids über „Auswahl
exportieren“ übertragen (bis zu 500 pro Export). Alle Felddefinitionen, Tags,
Vorgaben und Ansichtseinstellungen bleiben enthalten; Bilder stammen nur aus der
Auswahl oder den Feldvorgaben. Andere Liquids und deren zusätzliche Bilder fehlen.
Einstellungen → Daten exportiert weiterhin die gesamte Sammlung.

Der Import hat zwei Schritte: Datei prüfen und die angezeigte Vorschau
bestätigen. Er importiert Puffolio-Exporte, keine beliebigen CSV-Dateien.
IDs werden auf die Zielsammlung abgebildet; ein Import überschreibt nicht
stillschweigend bestehende Liquids. Die Webgrenzen betragen 100 MiB entpackte
Daten, 10.000 Archiveinträge und 5.000 Bilder. Ungültige Archivpfade und unbekannte
Schemaversionen werden abgelehnt. Exporte und Instanzsicherungen älterer Versionen
lassen sich weiter einspielen, umgekehrt nicht: Eine ältere Version lehnt das Archiv
einer neueren ab.

Der Import fügt Daten hinzu und behält Pflichtregeln bei. Würden vorhandene
oder importierte Liquids dadurch Pflichtwerte vermissen, wird der Import
vollständig abgelehnt. Das gilt auch beim Überschreiten gemeinsamer Grenzen,
etwa 100 gespeicherter Ansichten. Eine leere persönliche Sammlung eignet sich
für die unveränderte Übernahme einer anderen Sammlung.

Für größere eigene Exporte gibt es einen Offline-Import mit ausdrücklich
anzugebenden, begrenzten Ressourcenlimits. Den Container vorher stoppen und
das Datenverzeichnis sichern:

```sh
puffolio import --from collection.zip --collection COLLECTION_UUID \
  --max-expanded-bytes 209715200 --max-entries 20000 --max-images 10000
```

Dieser Aufruf erfolgt mit Zugriff auf das lokale Datenverzeichnis, etwa in
einem einmaligen Container mit demselben `/data`-Mount. Es gibt keinen
unbeschränkten „Limits aus“-Schalter. Freier Speicher wird benötigt. Ein
Sammlungsexport ist keine vollständige Instanzsicherung.

## Instanzsicherung

Der Administrator lädt unter Einstellungen → Daten nach erneuter
Passwortbestätigung eine vollständige Instanzsicherung herunter. Sie enthält
Datenbank, Konto und Medien. Das Manifest enthält Versionen und
Prüfsummen. Die Sicherung ist vertraulich und nicht verschlüsselt.

Die App erstellt eine konsistente SQLite-Sicherung und nimmt die zugehörigen
unveränderlichen Medien unter Schreibsperre auf. Nicht nur die gerade
geöffnete `.db`-Datei kopieren: SQLite-WAL gehört zum aktiven Zustand.
Sicherungen zusätzlich auf ein anderes physisches Ziel kopieren; eine
Sicherung auf demselben Volume schützt nicht vor dessen Ausfall.

## Offline-Restore

1. Passende Puffolio-Imageversion bereitstellen und den laufenden Container
   stoppen. Das bisherige Datenverzeichnis unverändert als Rückweg behalten.
2. Backup in ein eigenes Wiederherstellungsverzeichnis kopieren. Ein leeres
   Zielverzeichnis für die wiederhergestellten Daten anlegen und UID/GID99:100
   die benötigten Lese-/Schreibrechte geben.
3. Wiederherstellung mit dem Binary desselben Releases starten:

```sh
docker stop puffolio
mkdir -p recovery/restored
sudo chown -R 99:100 recovery
sudo chmod 700 recovery recovery/restored
docker run --rm --user 99:100 --read-only --cap-drop ALL \
  --security-opt no-new-privileges \
  --tmpfs /tmp:rw,noexec,nosuid,nodev,size=32m \
  -v "$PWD/recovery:/restore" ghcr.io/buffer379x/puffolio:0.4.1 \
  restore --from /restore/puffolio-backup.zip --data-dir /restore/restored
```

4. Der Restore prüft Manifest, sichere Pfade, Prüfsummen, Schema und
   Datenbankintegrität. Bestehende Ziele werden nicht überschrieben. Alte
   Sitzungen werden widerrufen; der Administrator meldet sich erneut an.
   Die Sicherung einer älteren Version wird beim ersten Start automatisch
   auf das aktuelle Schema gebracht.
5. Die Bindung `/data` in Compose bzw. Unraid auf das wiederhergestellte
   Verzeichnis umstellen und mit dem passenden Image starten.
6. Readiness, Anmeldung, Datum, Tags, Bilder einschließlich nachträglichem
   Ausschnitt und Dashboard prüfen. Erst danach über das alte Verzeichnis
   entscheiden.

## Passwort vergessen

Ein Administrator kann lokal das Passwort eines Kontos zurücksetzen. Den
laufenden Dienst vorher stoppen, damit nur ein Prozess das Datenverzeichnis
schreibend nutzt. Der Unterbefehl `reset-password --username NAME` liest das
neue Passwort über die lokale Eingabe; Geheimnisse nicht als Prozessargument
in Shell-History oder Logs schreiben. Keine öffentlich abrufbaren
Standard-Resetcodes und keine SMTP-Abhängigkeit.

Alte Sitzungen werden dabei ungültig. Danach ist die Anmeldung mit dem
Benutzernamen und dem neuen Passwort möglich.

## Update und Rollback

Vor einem Update eine geprüfte Instanzsicherung anlegen. Eine konkrete
Imageversion wählen und dieselbe Datenbindung verwenden. Ein unbekanntes
neueres Schema wird beim Start abgelehnt. Rollback benötigt sowohl das alte
Image als auch ein dazu passendes Backup; ein altes Binary darf nicht einfach
auf eine neu migrierte Datenbank angewendet werden.

Ein Schema-Update läuft beim Start in einer einzigen Transaktion: Es wird ganz
oder gar nicht angewendet. Schlägt es fehl, startet Puffolio nicht, und die
Datenbank bleibt unverändert auf dem alten Stand.

Die Version ist in `compose.yaml` und im Unraid-Template fest eingetragen und wird
nie stillschweigend überschrieben. Zusätzlich zeigt `latest` auf den neuesten Release. Für ein Update die neue Version aus dem Release
übernehmen: `compose.yaml` ersetzen beziehungsweise in Unraid den Image-Tag ändern.
Die Release-Hinweise nennen, was sich für bestehende Installationen ändert.
