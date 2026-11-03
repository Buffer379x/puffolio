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

Normale Nutzer haben getrennte persönliche Sammlungen. Administratoren
verwalten Konten und Instanzsicherungen. Wer den Host, die Datenbankdateien oder
Sicherungen kontrolliert, kann gespeicherte Inhalte technisch lesen.
Sicherungen und eigene Exporte sind vertraulich; sie sind nicht verschlüsselt.

Der Setup-Code wird nur im Datenverzeichnis erzeugt. Es gibt keine
Standardzugangsdaten. Das Datenverzeichnis gehört auf lokalen Speicher; NFS
und SMB sind für den aktiven SQLite-WAL-Betrieb nicht unterstützt.
