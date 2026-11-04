# Release erstellen

Eine Version entsteht aus einem geprüften Commit auf `main`. Image und Tag sind danach
unveränderlich; eine fehlerhafte Version wird durch eine neue ersetzt.

## Vorbereiten

1. Version eintragen: `VERSION`, `web/package.json`, `info.version` in `docs/api/openapi.yaml`,
   Image in `deploy/compose.yaml`, Image, Icon und Template-URL in `deploy/unraid/puffolio.xml`,
   Pins in README und Betriebsdoku.
2. `CHANGELOG.md` ergänzen und `docs/release-X.Y.Z.md` schreiben.
3. Screenshots neu erzeugen, wenn sich die Oberfläche geändert hat:
   `pnpm --dir web build`, `go build -o work/bin/puffolio ./cmd/puffolio`,
   `pnpm --dir web screenshots`.
4. `python3 docs/scripts/check-version.py` meldet jeden vergessenen Eintrag. Die CI und der
   Release-Workflow führen diese Prüfung ebenfalls aus.
5. Pull Request öffnen, CI abwarten, mergen.

## Veröffentlichen

```sh
gh workflow run release.yml --repo Buffer379x/puffolio --ref main \
  -f version=X.Y.Z -f source_sha="$(gh api repos/Buffer379x/puffolio/commits/main --jq .sha)"
gh run watch --repo Buffer379x/puffolio
```

`source_sha` muss der aktuelle Stand von `main` sein. Der Workflow führt die vollständige CI aus,
baut das Multiarch-Image, signiert die Herkunft, startet das Image anonym auf beiden
Architekturen samt Upgrade- und Restore-Prüfung, scannt es und erstellt erst danach Tag und
Release. Ein erneuter Lauf baut ein vorhandenes Image nicht neu, sondern prüft es.

## Wenn der letzte Schritt scheitert

Die geprüften Dateien liegen 30 Tage als Artefakt `release-assets` am Lauf. Scheitert nur
`Create release`, zum Beispiel weil GitHub dem Workflow-Token das Schreiben verweigert, wird der
Release mit den eigenen Zugangsdaten aus genau diesen Dateien abgeschlossen:

```sh
gh run download RUN_ID --repo Buffer379x/puffolio --name release-assets --dir assets
(cd assets && sha256sum -c SHA256SUMS)
gh release create vX.Y.Z assets/* --repo Buffer379x/puffolio --target SOURCE_SHA \
  --title 'Puffolio X.Y.Z' --notes-file docs/release-X.Y.Z.md --latest
```

Nichts daran neu bauen: Das Image ist bereits veröffentlicht und signiert.
