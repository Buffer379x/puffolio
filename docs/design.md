# Gestaltungsregeln

Die Referenz für das Aussehen ist die laufende App, nicht ein separater Entwurf:
`pnpm --dir web demo` startet sie mit Beispieldaten, `pnpm --dir web screenshots`
erzeugt daraus `docs/screenshots`. Oberflächen entstehen aus den
gemeinsamen Bausteinen in `web/src/components` und dem einen Stylesheet
`web/src/styles/app.css`. Neue Funktionen werden auf Smartphone und Desktop, mit
Tastatur und in beiden Farbschemata geprüft.

## Grundwerte

- Systemschriften; UI-Größen 12, 14, 16, 24, 28 und 32 px. Labels und Buttons 14 px,
  Eingaben 16 px.
- Abstände 4/8/12/16/20/24/32/40/48/64 px. Radien: Controls 8 px, Panels und Dialoge
  16 px, Tags 4 px. Leichte Rahmen statt starker Schatten.
- Interaktive Ziele mindestens 44 px hoch, Iconaktionen 44 × 44 px.
- Inhalt maximal 1360 px, Seitenränder 32 px, mobil 16 px. Breakpoints 1199, 900, 600.
- Farben nur über die Tokens in `tokens.css`. Normaler Text mindestens 4,5:1.

## Bausteine

| Baustein | Regel |
| --- | --- |
| `Button`, `IconButton`, `SegmentedControl` | Einzige Bedienelemente; Primär nur für die Hauptaktion je Bereich |
| `Icon` | Einziges Iconset, 24-px-Raster; keine Emoji |
| `Panel`, `section-card` | Einzige Flächen für Inhalte |
| `ItemCard` / `LiquidCard` | Kartenraster, das auch Tools nutzt; die ganze Karte öffnet das Detail, Leerwerte belegen keinen Platz |
| `LiquidImage` | Einziger Bildplatz: quadratisch, gefüllt, Thumbnail außer im Detail; ohne Foto Flasche in der Liquidfarbe |
| `ShelfBar` | Haltbarkeit: Länge und Farbe aus dem Restanteil, Grün bis Rot, immer mit Text („80 %“, „Abgelaufen“) |
| `Stars` | Lesende Bewertung; keine Bewertung ist „—“, nie null Sterne |
| `DonutChart`, `ColumnChart`, `ShelfStatusChart` | Diagramme ohne Bibliothek: Legende mit Zahlen, Tabelle für Screenreader, Serienfarben aus `--series-*` |
| `Dialog`, `Confirm` | Einzige Überlagerungen; mobil bildschirmfüllend für Liquidformulare |
| `useToast` | Bestätigung ohne sichtbare Änderung sowie „Rückgängig“ |

## Texte

Kurze Funktionsnamen, keine Marketingtexte. Erklärungen nur im aufrufbaren
Hilfe-Pop-up, Fehler direkt am Feld. Leerzustand: ein Satz und höchstens eine Aktion.

## Besucher

Besucher sehen dieselbe Oberfläche ohne ändernde Bedienelemente. Bedienelemente
werden entfernt, nicht deaktiviert. Sicherheit entsteht auf dem Server, nicht durch
das Ausblenden.
