# Wohnungssuche

Telegram-gesteuerte Go-Anwendung zum Beobachten von Wohnungsseiten. Die App lädt Zielseiten mit `net/http`, prüft auf den Text `Mietwohnungen verfügbar`, extrahiert relevante Wohnungsdaten per LLM und speichert neue Treffer in PostgreSQL über GORM.

## Stack

- Go
- `net/http` als Fetcher
- `github.com/PuerkitoBio/goquery` als HTML-Parser
- PostgreSQL als Datenbank
- GORM als ORM
- Telegram Bot als Bedienoberfläche

## Funktionen

- URLs per Telegram hinzufügen, auflisten und entfernen
- Hintergrund-Scraping in einem konfigurierbaren Intervall
- Prüfung auf den Marker `Mietwohnungen verfügbar`
- LLM-gestützte Extraktion relevanter Daten als JSON
- Speicherung neuer Treffer mit Deduplizierung per Fingerprint
- Telegram-Benachrichtigung bei neuen Treffern

## Einrichtung

1. PostgreSQL starten, zum Beispiel mit Docker Compose:

```bash
docker compose up -d
```

2. Umgebungsvariablen setzen. Eine Vorlage liegt in `.env.example`.

3. Abhängigkeiten installieren:

```bash
go mod tidy
```

4. Bot starten:

```bash
go run ./cmd/bot
```

## Telegram-Befehle

- `/addurl <url> [name]`
- `/listurls`
- `/removeurl <id>`
- `/scrape`

## Wichtige Umgebungsvariablen

- `DATABASE_URL`: PostgreSQL-DSN
- `TELEGRAM_BOT_TOKEN`: Telegram Bot Token
- `TELEGRAM_ALLOWED_CHAT_ID`: optional, schränkt den Zugriff auf einen Chat ein
- `LLM_API_URL`: OpenAI-kompatibler Chat-Completions-Endpunkt
- `LLM_API_KEY`: API-Schlüssel für das LLM
- `LLM_MODEL`: Modellname für die Extraktion
- `SCRAPE_INTERVAL`: zum Beispiel `15m`

## Hinweise

- Das Projekt erwartet ein OpenAI-kompatibles LLM-API-Format.
- Die LLM-Ausgabe wird als JSON gespeichert, damit die Felder später erweitert oder anders ausgewertet werden können.
- Die Deduplizierung basiert auf URL, Titel, Zusammenfassung und dem extrahierten JSON.

## Deployment Mit GitHub Secrets

- Lege in GitHub Actions folgende Repository-Secrets an: `DEPLOY_HOST`, `DEPLOY_USER`, `SSH_PRIVATE_KEY`, optional `SSH_PASSPHRASE`, sowie alle App-Variablen aus `.env.example`.
- Der Workflow in `.github/workflows/deploy.yml` baut `./cmd/bot` als Linux-Binary, kopiert die Binary und die Systemd-Unit auf den Server und schreibt `/etc/scraper/scraper.env` aus den GitHub-Secrets.
- Die bereitgestellte Unit-Datei in `deploy/scraper.service` lädt die Laufzeitvariablen über `EnvironmentFile=-/etc/scraper/scraper.env`.
