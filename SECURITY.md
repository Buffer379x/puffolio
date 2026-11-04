# Sicherheit

Puffolio ist für den Betrieb auf dem eigenen Server vorgesehen. Verwende
aktuelle Releases, sichere Passwörter und HTTPS am vorgeschalteten Reverse
Proxy, wenn die App über ein nicht vertrauenswürdiges Netz erreichbar ist.

## Sicherheitsprobleme melden

Bitte keine Passwörter, Cookies, Liquid-Exporte oder Sicherungen in öffentliche
Issues hochladen. Melde eine Sicherheitslücke über die private GitHub-Funktion
„Report a vulnerability“, sofern diese für das Repository verfügbar ist.
Falls sie nicht verfügbar ist, eröffne zunächst ein Issue ohne ausnutzbare
Details und ohne private Daten, um einen privaten Meldeweg abzustimmen.

## Vertrauensgrenzen

Puffolio hat ein Administratorkonto. Die Sammlung (Liquids, Felder, Tags,
zugeschnittene Bilder, Dashboard) ist ohne Anmeldung für alle lesbar, die die
Adresse erreichen. Jede Änderung, der Papierkorb, unbeschnittene Originalbilder,
Export und Sicherung erfordern die Anmeldung; der Server erzwingt das. Wer den Host, die Datenbankdateien oder
Sicherungen kontrolliert, kann gespeicherte Inhalte technisch lesen.
Sicherungen und eigene Exporte sind vertraulich; sie sind nicht verschlüsselt.

Der einmalige Setup-Code steht im Datenverzeichnis und im Container-Log. Es gibt keine
Standardzugangsdaten. Das Datenverzeichnis gehört auf lokalen Speicher; NFS
und SMB sind für den aktiven SQLite-WAL-Betrieb nicht unterstützt.
