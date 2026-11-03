# Gestaltungsregeln

Puffolio verwendet gemeinsame Bausteine statt eigener Varianten je Seite.
Neue Funktionen übernehmen diese Regeln und werden auf Smartphone und Desktop,
mit Tastatur sowie in beiden Farbschemata geprüft.

- Systemschriften; UI-Größen 12, 14, 16, 24, 28 und 32 CSS-Pixel.
- 14-px-Labels und Buttontexte, 16-px-Eingaben; Zusatztexte mindestens 12 px.
- Interaktive Ziele mindestens 44 px hoch, Iconaktionen mindestens 44 × 44 px.
- Abstandsskala 4/8/12/16/20/24/32/40/48/64 px.
- Radien: Controls 8 px, Panels und Dialoge 16 px, nicht interaktive Tags 4 px.
- Desktopnavigation 232 px; bis 900 px Menüschublade. Navigationsinhalt muss
  bei geringer Höhe scrollbar bleiben. Geschlossenes Menü ist nicht bedienbar.
- Inhalt maximal 1360 px, Seitenränder 32 px bzw. mobil 16 px.
- Kartenraster anhand tatsächlicher Inhaltsbreite, Mindestkartenbreite 240 px.
  Karten einer Rasterzeile sind gleich hoch; Aktionen stehen an derselben Kante.
- Alle Liquidbildplätze haben 4:3 und zeigen denselben bestätigten Ausschnitt.
- Einfache Formularfelder paarweise; Name, Tags, Rating, Upload und Notizen
  breit. Einzelne Restfelder breit; niemals die Nutzerreihenfolge verändern.
  Bis 600 px einspaltig.
- Dialoge teilen Kopf, Körper und Aktionsleiste. Normalerweise scrollt nur der
  Körper. Bei geringer Höhe ist ein Gesamtscroll-Fallback erforderlich. Mobile
  Liquidformulare sind bildschirmfüllend. Fokus und ESC sind verbindlich.
- Gleiche Kartenbasis für Tools, dort ausschließlich Titel und Bild.
- Theme ausschließlich im Menü, Widgets ausschließlich im Dashboard,
  Ansichtsoptionen ausschließlich auf der Liquids-Seite.
- Kurze sachliche UI-Texte; zusätzliche Erklärungen in aufrufbaren Hilfen.
  Notwendige Einheiten und Fehlermeldungen stehen direkt am Feld.
- Eine Primär-, Sekundär- und Gefahrvariante, keine neue kleine Größenfamilie.
- Sichtbarer Fokus, Kontrastziel 4,5:1 für Text und 3:1 für relevante
  Controlgrenzen. Auswahl nicht ausschließlich durch Farbe kennzeichnen.
- Reduzierte Bewegung beachten, keine externen Fonts oder Marketing-Tracker.

Bei Änderungen: vorhandenen Baustein wählen, lange/fehlende/fehlerhafte Werte
prüfen, gemeinsame Tokens statt Seitensondermaßen ändern und die Prüfung
dokumentieren. Die Referenzmatrix umfasst 320/360/390/768/900/1280/1440 px,
200 % Zoom sowie niedrige Fenster und mobile Bildschirmtastaturen.
