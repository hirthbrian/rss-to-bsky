# CLAUDE.md

Guidance for Claude Code when working in this repository.

## Project overview

A Go service that polls an RSS feed and posts new items to Bluesky via the AT Protocol HTTP API. Runs as a single long-lived container with an in-process cron scheduler — no external cron, no host dependencies.

## Stack

- **Language:** Go (1.23+)
- **RSS parsing:** `github.com/mmcdole/gofeed`
- **Scheduling:** `github.com/robfig/cron/v3` (in-process, not host cron)
- **Storage:** SQLite via `modernc.org/sqlite` (pure Go, no cgo) — stores posted item GUIDs for dedup
- **Bluesky:** raw HTTP calls to the AT Protocol (`com.atproto.server.createSession`, `com.atproto.repo.createRecord`) — no SDK
- **Deployment:** Docker, multi-stage build, `CGO_ENABLED=0`

## Architecture

Standard Go project layout: a thin `cmd/` entrypoint over `internal/` packages split along the bot's natural seams.

```
cmd/rss-to-bsky/main.go   — main() opens the store, runs once, starts cron, blocks forever; runOnce() drives one poll-and-post cycle
internal/feed/            — Fetch() parses the RSS feed and reduces items to {GUID, Text}
internal/bluesky/         — Login() authenticates; Session.Post() creates a post record — raw HTTP, no SDK
internal/store/           — Open() opens SQLite; AlreadyPosted()/MarkPosted() do GUID dedup
```

## Environment variables

| Var                 | Purpose                                                                             |
| ------------------- | ----------------------------------------------------------------------------------- |
| `RSS_URL`           | Feed to poll                                                                        |
| `BSKY_HANDLE`       | Bot's Bluesky handle                                                                |
| `BSKY_APP_PASSWORD` | App password (never the real account password) — passed via `.env`, never committed |

## Key constraints

- **Post text is capped at 300 chars** — always truncate before sending to `createRecord`. Current code truncates at 280 to leave headroom.
- **Dedup is GUID-based**, stored in SQLite at `/data/posted.db` inside the container. The `./data` volume must persist across deploys or the bot will repost the entire feed on every restart.
- **App password, not account password** — generated in Bluesky settings, scoped for bot use.
- **`CGO_ENABLED=0`** must stay set in the Dockerfile build stage — `modernc.org/sqlite` is pure Go, and disabling cgo keeps the final image small and the build simple. Don't swap in `mattn/go-sqlite3` without also re-adding a C toolchain to the build stage.

## Local development

```bash
docker compose up -d --build
docker compose logs -f
```

`.env` (gitignored) must define `BSKY_APP_PASSWORD`. `RSS_URL` and `BSKY_HANDLE` are set directly in `docker-compose.yml`.

## When making changes

- If changing the poll interval, it's set in `main.go` via `c.AddFunc("@every 15m", runOnce)` — not an env var currently. Consider promoting to an env var if this needs to vary per deployment.
- If adding new AT Protocol calls (likes, replies, images), follow the existing pattern in `postToBluesky()`: raw `net/http` + `encoding/json`, no SDK.
- Any change to the dedup schema requires a migration plan for the existing `/data/posted.db` volume — don't just drop and recreate the table.
- Keep the final Docker image minimal (`alpine` base, static binary). Avoid adding dependencies that require cgo unless there's a strong reason.
