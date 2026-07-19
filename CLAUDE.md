# CLAUDE.md

Guidance for Claude Code when working in this repository.

## Project overview

A Go service that polls an RSS feed and posts new items to Bluesky via the AT Protocol HTTP API. Runs as a single long-lived container with an in-process cron scheduler — no external cron, no host dependencies.

## Stack

- **Language:** Go (1.26+)
- **RSS parsing:** `github.com/mmcdole/gofeed`
- **Scheduling:** `github.com/robfig/cron/v3` (in-process, not host cron)
- **Storage:** SQLite via `modernc.org/sqlite` (pure Go, no cgo) — stores posted items for dedup, keyed by a unique `origin` column with a generated UUID (`github.com/google/uuid`) `id` primary key
- **Bluesky:** raw HTTP calls to the AT Protocol (`com.atproto.server.createSession`, `com.atproto.repo.createRecord`, `com.atproto.repo.uploadBlob`) — no SDK
- **Link previews:** `github.com/PuerkitoBio/goquery` to parse Open Graph tags off the linked article for the `app.bsky.embed.external` card
- **Deployment:** Docker, multi-stage build, `CGO_ENABLED=0`

## Architecture

Standard Go project layout: a thin `cmd/` entrypoint over `internal/` packages split along the bot's natural seams.

```
cmd/rss-to-bsky/main.go   — main() opens the store, runs once, starts cron, blocks forever; runOnce() drives one poll-and-post cycle, buildEmbed()/fetchAndUploadThumb() assemble the link preview card
internal/feed/            — Fetch() parses the RSS feed and reduces items to {GUID, Title, Text, Link}
internal/bluesky/         — Login() authenticates; Session.Post() creates a post record (with rich-text link facet and optional ExternalEmbed); Session.UploadBlob() uploads the preview thumbnail — raw HTTP, no SDK
internal/opengraph/       — Fetch() pulls og:title/og:description/og:image off a linked page for the preview card, best-effort (errors don't fail the post)
internal/store/           — Open() opens SQLite and runs migrate(); AlreadyPosted()/MarkPosted() do origin dedup, MarkPosted() generates the row's UUID id
```

## Environment variables

| Var                 | Purpose                                                                             |
| ------------------- | ----------------------------------------------------------------------------------- |
| `RSS_URL`           | Feed to poll                                                                        |
| `BSKY_HANDLE`       | Bot's Bluesky handle                                                                |
| `BSKY_APP_PASSWORD` | App password (never the real account password) — passed via `.env`, never committed |
| `DB_PATH`           | Optional. SQLite file path, defaults to `/data/posted.db`. Override for local `go run` testing outside Docker (e.g. `./data/posted.db`). |

## Key constraints

- **Post text is capped at 300 chars** — always truncate before sending to `createRecord`. Current code truncates at 280 to leave headroom.
- **Dedup is origin-based** (the feed item's GUID, or its link as a fallback), stored in SQLite at `/data/posted.db` inside the container. Each row has a generated UUID `id` (primary key) and a unique `origin` column. The `./data` volume must persist across deploys or the bot will repost the entire feed on every restart.
- **Links must survive truncation intact.** `formatPost()` in `internal/feed/feed.go` truncates only the title, never the link, so the URL always matches byte-for-byte against the rich-text facet and stays a valid, clickable link.
- **Link previews are best-effort.** If `opengraph.Fetch()` or the thumbnail upload fails, `buildEmbed()` in `cmd/rss-to-bsky/main.go` logs and posts without an embed rather than failing the run — a missing preview card shouldn't block a post.
- **App password, not account password** — generated in Bluesky settings, scoped for bot use.
- **`CGO_ENABLED=0`** must stay set in the Dockerfile build stage — `modernc.org/sqlite` is pure Go, and disabling cgo keeps the final image small and the build simple. Don't swap in `mattn/go-sqlite3` without also re-adding a C toolchain to the build stage.

## Local development

### With Docker (matches production)

```bash
docker compose up -d --build
docker compose logs -f
```

`.env` (gitignored) must define `BSKY_APP_PASSWORD`. `RSS_URL` and `BSKY_HANDLE` are set directly in `docker-compose.yml`.

### Without Docker

Requires a local Go 1.26+ toolchain (`brew install go`).

```bash
make check   # build + vet + test
make test    # go test ./... only
make run     # sources .env, then go run ./cmd/rss-to-bsky
```

Or run the underlying commands directly:

```bash
go build ./...   # compile check
go vet ./...      # static checks
go test ./...     # test suite
set -a; source .env; set +a  # load env vars from .env into the shell
go run ./cmd/rss-to-bsky
```

`main()` doesn't read `.env` itself — only `docker-compose.yml`'s `env_file` does — so `.env` must be sourced into the shell before `go run` (the `make run` target does this for you). For local runs, `.env` needs `RSS_URL`, `BSKY_HANDLE`, and `BSKY_APP_PASSWORD` set directly (they're not hardcoded into `docker-compose.yml` the way the Docker path has them). Also set `DB_PATH` (e.g. `./data/posted.db`) — the default `/data/posted.db` targets the Docker volume mount and isn't writable outside a container (on macOS, `/` is a read-only system volume, so this fails even with `sudo`).

## When making changes

- If changing the poll interval, it's set in `cmd/rss-to-bsky/main.go` via `c.AddFunc("@every 15m", run)` — not an env var currently. Consider promoting to an env var if this needs to vary per deployment.
- If adding new AT Protocol calls (likes, replies, images), follow the existing pattern in `internal/bluesky/bluesky.go`'s `Session.Post()`: raw `net/http` + `encoding/json`, no SDK.
- Any change to the dedup schema requires a migration plan for the existing `/data/posted.db` volume — don't just drop and recreate the table. See `migrate()` in `internal/store/store.go` for the pattern: detect the old schema via `PRAGMA table_info`, rename the old table aside, create the new one, copy rows across inside a transaction, then drop the old table. The whole migration runs in one transaction so a crash mid-migration rolls back cleanly and retries on next startup.
- Keep the final Docker image minimal (`alpine` base, static binary). Avoid adding dependencies that require cgo unless there's a strong reason.
