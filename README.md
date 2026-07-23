# Music library syncer

> Go daemon that syncs a music library from a Google Sheet to a remote storage host via `rsync`.
> Evolution of the older [`tracker-downloader`](https://github.com/antoni-ostrowski/tracker-downloader).

## What it does

1. Downloads a source CSV from a Google Sheet.
2. Parses track metadata and `pillows.su` download links.
3. Downloads tracks from the API.
4. Writes tags (album = era, title, artist, custom notes) and embeds the era cover image.
5. Keeps track metadata in an SQLite database (deletes removed rows, upserts changed ones).
6. Syncs the downloaded song directory to a remote host via `rsync` over SSH.
7. Repeats every 2 hours.


## Run locally

```bash
./scripts/dev.sh
```

- Loads `.env.local`.
- Runs `go run ./cmd/main.go -d` (dev mode: downloads only one sample and exits the loop after one pass).

## Build

```bash
./scripts/build.sh
```

Builds a stripped binary `./library-syncer` from `cmd/main.go`.

## Docker

Build:

```bash
./scripts/docker-build.sh
```

```bash
./scripts/docker-run-local.sh
```

The image is published as `antost360/library-syncer:latest`.

### CI/CD

`.woodpecker.yml` builds the image on every PR/push to `main`, pushes to Docker Hub, and updates the `library-syncer` service in a my homelab Docker Compose setup.

## Required environment variables

| Variable | Purpose | Example (local) | Example (Docker) |
|---|---|---|---|
| `RSYNC_USER` | SSH user for the sync target | `antost` | `antost` |
| `RSYNC_HOSTNAME` | SSH host for the sync target | `linux` | `linux` |
| `RSYNC_DEST` | Remote path to sync songs into | `/home/antost/mac-test` | `/home/antost/mac-test` |
| `SSH_KEY` | SSH private key path | `~/.ssh/id_ed25519` | `/root/.ssh/id_ed25519` |
| `DB_PATH` | Directory for the SQLite file | `.` | `/app/data/db` |
| `SONGS_PATH` | Directory for downloaded tracks | `dev/songs` | `/app/data/songs` |
| `SECRETS_PATH` | Directory for Google credentials/tokens | `dev/data/secrets` | `/app/data/secrets` |
| `WORKER_COUNT` | Concurrent download workers | `6` | `6` |
| `ASSETS_PATH` | Directory containing cover images (`<era>.jpg`, `default.jpg`) | `assets/covers` | `/app/data/covers` |

The app loads `.env.local` automatically for local runs; Docker uses `--env-file ./scripts/.docker-env`.

## Required secrets

Place inside `SECRETS_PATH`:

- `credentials.json` — Google OAuth 2.0 credentials from Google Cloud Console.
- On first run, the app prints an auth URL and waits for the OAuth code at `SECRETS_PATH/code.txt`. After exchange, it stores the token at `SECRETS_PATH/token.json`.

## Project structure

```
.
├── cmd/main.go              # Entry point + env loading + loop
├── internal
│   ├── db                   # SQLite schema, migrations, sync logic
│   ├── downloader           # pillows.su API downloader + tagging
│   ├── gsh                  # Google Sheet CSV download
│   ├── parser               # CSV → track model
│   └── syncer               # rsync over SSH
├── scripts                  # build, dev, docker build/run/deploy
├── assets/covers            # Cover images
├── Dockerfile
└── .woodpecker.yml          # CI/CD pipeline
```

## Stack

- Go 1.25
- SQLite (modernc.org/sqlite)
- Google Sheets API v4 + OAuth2
- taglib for metadata / cover embedding
- ffmpeg for mp4 → mp3 conversion
- rsync over SSH for remote sync
