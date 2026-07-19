# rss-to-bsky

A small Go service that polls an RSS feed and posts new items to [Bluesky](https://bsky.app) via the AT Protocol HTTP API. It runs as a single long-lived container with an in-process cron scheduler — no host cron, no external dependencies beyond the container itself.

## Features

- Polls an RSS feed on a schedule (every 15 minutes by default) and posts new items to Bluesky
- Tracks posted items in a local SQLite database so restarts don't repost the entire feed
- Renders links in posts as proper clickable rich-text links, not plain text
- Attaches a link preview card (title, description, thumbnail) pulled from the article's Open Graph tags
- Truncates post text to fit Bluesky's character limit without ever cutting off a link mid-URL
- Pure Go, no cgo — small Alpine-based Docker image

## Requirements

- A Bluesky account and an [app password](https://bsky.app/settings/app-passwords) (never use your real account password)
- Docker + Docker Compose (recommended), or a local Go 1.23+ toolchain

## Setup

1. Clone the repo and copy the example env file:

   ```bash
   cp .env.example .env
   ```

2. Fill in `.env`:

   ```
   RSS_URL=https://example.com/feed.xml
   BSKY_HANDLE=yourhandle.bsky.social
   BSKY_APP_PASSWORD=xxxx-xxxx-xxxx-xxxx
   ```

3. Start it:

   ```bash
   docker compose up -d --build
   docker compose logs -f
   ```

Posted item state is persisted in `./data/posted.db` (mounted into the container at `/data/posted.db`), so it survives restarts and redeploys as long as that volume isn't deleted.

## Configuration

| Var                 | Purpose                                                                         |
| -------------------- | -------------------------------------------------------------------------------- |
| `RSS_URL`           | Feed to poll                                                                     |
| `BSKY_HANDLE`       | Bot's Bluesky handle                                                             |
| `BSKY_APP_PASSWORD` | App password — generate one in Bluesky settings, never your account password    |
| `DB_PATH`           | Optional. SQLite file path, defaults to `/data/posted.db`. Useful for pointing at a project-local path when running without Docker. |

## Running without Docker

Requires a local Go 1.23+ toolchain.

```bash
make check   # build + vet + test
make run     # sources .env, then runs the service
```

Or run the underlying commands directly:

```bash
go build ./...   # compile check
go vet ./...      # static checks
go test ./...     # run the test suite

set -a; source .env; set +a  # load .env into the shell
DB_PATH=./data/posted.db go run ./cmd/rss-to-bsky
```

`main()` doesn't read `.env` itself — only Docker Compose's `env_file` does — so it needs to be sourced into the shell first (`make run` does this for you). The default `DB_PATH` (`/data/posted.db`) targets the Docker volume mount and generally isn't writable outside a container, so override it for local runs.

## Project layout

```
cmd/rss-to-bsky/main.go   — entrypoint: opens the store, runs once, then starts the cron scheduler
internal/feed/            — fetches and parses the RSS feed into postable items
internal/bluesky/         — minimal AT Protocol HTTP client (login, create post record, upload preview thumbnail)
internal/opengraph/       — fetches Open Graph metadata from a linked article for the preview card
internal/store/           — SQLite-backed dedup store, including schema migrations
```

## License

No license specified.
