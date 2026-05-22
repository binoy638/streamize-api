# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Go API
make api-dev          # run the API (http://localhost:8080)
make api-test         # run all Go tests
make api-tidy         # go mod tidy

# Run a single package's tests
cd apps/api && go test ./internal/transcoding/...

# React + Vite frontend
make web-dev          # dev server (http://localhost:5173, proxies /api → :8080)
make web-build

# Docker Compose (full stack)
docker compose -f deploy/docker-compose.yml --env-file deploy/.env up --build
```

Default dev credentials: `admin` / `adminadmin`. Override with `STREAMIZE_ADMIN_USERNAME` / `STREAMIZE_ADMIN_PASSWORD`.

## Architecture

Streamize is a self-hosted torrent-backed video streaming app undergoing a rewrite from TypeScript to Go. `legacy/api` is reference-only; all active work is in `apps/api` (Go) and `apps/web` (React+Vite).

### Backend (`apps/api`)

Entry point: `cmd/api/main.go`. Wires config → database → auth bootstrap → media worker goroutine → HTTP server.

**Internal packages:**

| Package | Role |
|---|---|
| `config` | Env-var config (all vars prefixed `STREAMIZE_`). Validates on load; fails fast on missing required values. |
| `database` | Opens SQLite via `go-sqlite3`, runs hand-written migrations from `internal/database/migrations/`. |
| `auth` | Session-cookie auth; `BootstrapAdmin` creates the admin user on first start. |
| `torrents` | `Store` wraps all SQLite reads/writes for torrents, torrent files, subtitles, preview assets, video progress, and share links. Torrent status constants live here (`added`, `downloading`, `done`, `error`, etc.). |
| `jobs` | Durable job queue backed by SQLite. `ClaimNext` leases a job with an expiring lock so the worker can crash-recover. |
| `qbittorrent` | HTTP client wrapping the qBittorrent WebUI API. Implements the handler interfaces (`TorrentAdder`, `TorrentLister`, etc.). |
| `httpserver` | chi router. Handler structs receive interface dependencies (not concrete types) so tests can inject fakes. `NewRouter` wires the real qBittorrent client unless overridden via `RouterOption` functions. |
| `transcoding` | `Worker` polls the job queue and runs the HLS transcode pipeline: `Prober` → `PlanHLS` → `Transcoder.TranscodeHLS` → `AssetProcessor`. All three are interfaces; `FFprobeProber`, `FFmpegTranscoder`, and `FFmpegAssetProcessor` are the production implementations. |
| `media` | Disk-level helpers (free space, path safety). |

### Media pipeline

When a torrent finishes downloading, a `hls_transcode` job is enqueued. The worker:
1. Probes the file with `ffprobe` to get codec/container/duration.
2. Calls `PlanHLS` to decide processing mode (copy vs. transcode).
3. Runs `ffmpeg` to produce an HLS playlist + fMP4 segments under `<HLSDir>/<torrent_file_id>/`.
4. Extracts subtitles and generates a thumbnail sprite sheet as post-processing.
5. Marks the file `done` and completes the job.

Progress is reported back to the DB as `transcoding_percent` throughout.

### Sharing & public playback

A **share** is an expiring, login-free link to either a single torrent file or a
whole torrent. Owners create/list/revoke shares with `POST` / `GET` / `DELETE
/api/shares` (`ShareHandler`); rows live in the `shares` table, owned by the
`torrents` `Store`. Anyone holding a slug streams without authentication through
the public, slug-scoped routes under `/api/shares/view/{slug}/...` — these mirror
the authenticated playback endpoints (HLS playlist/segments, original file,
subtitles, preview thumbnails) and rewrite asset URLs so they stay within the
slug scope. The browser-facing link is `/s/{slug}`, served by the SPA's public
`SharePage`. Watch-party links follow the same pattern under
`/api/watch-parties/join/{slug}`.

### Storage layout (under `STREAMIZE_MEDIA_ROOT`)

```
originals/   raw downloads (managed by qBittorrent)
hls/         playlists and fMP4 segments per file id
subtitles/   extracted subtitle tracks
thumbnails/  sprite sheets and VTT preview files
tmp/         temporary processing scratch space
```

SQLite lives at `STREAMIZE_DATABASE_PATH` (default `./data/streamize.db`), outside the media root.

### Frontend (`apps/web`)

React + Vite app. Dev server proxies `/api` to `:8080`. The sign-in screen has a prototype bypass for browsing mock media flows when the API isn't running. Shell pages (require auth): `LibraryPage`, `LibraryDetailPage`, `TorrentsPage`, `TorrentDetailPage`, `PlayerPage`, `JobsPage`, `SharesPage`, `SettingsPage`, `AdminUsersPage`. Public pages (no shell, no auth): `SharePage` (`/s/:slug`) and `WatchPartyPage` (`/watch/:slug`).

## Key conventions

- All config is via environment variables; no config files at runtime.
- Handler interfaces in `httpserver` (`TorrentAdder`, `TorrentLister`, etc.) exist solely for test injection — production uses the qBittorrent client.
- The job queue uses SQLite row-level leasing: a worker ID and lease expiry are stamped on the claimed row; expired leases are reclaimed automatically.
- HLS output uses fMP4 segments (`segment_%05d.m4s`) with an `index.m3u8` playlist, stored per torrent-file ID.
- Public playback (shares, watch parties) is unauthenticated but slug-scoped: a request can only reach files covered by its slug, and served playlists/VTTs are rewritten so every asset URL stays inside that scope.
- `ffmpeg`/`ffprobe` binaries default to `PATH` lookup; override with `STREAMIZE_FFMPEG_PATH` / `STREAMIZE_FFPROBE_PATH`.
