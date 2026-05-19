# Streamize

Streamize is being rewritten as a Go-first monorepo for self-hosted torrent-backed video streaming.

The current TypeScript API has been preserved under `legacy/api` while the new implementation is built in `apps/api`.

## Repository Layout

```text
apps/
  api/   Go API, workers, SQLite schema, and future static UI serving
  web/   Future React + Vite UI
deploy/ Docker Compose and environment examples
docs/   Architecture and implementation notes
legacy/ Previous TypeScript API kept for reference during the rewrite
```

## Backend Development

```bash
make api-test
make api-dev
```

The API defaults to `http://localhost:8080` and exposes:

```text
GET /api/health
POST /api/auth/sign-in
POST /api/auth/sign-out
GET /api/auth/me
GET /api/torrents
POST /api/torrents
GET /api/torrents/{id}/files
DELETE /api/torrents/{id}
GET /api/jobs
POST /api/jobs/{id}/retry
POST /api/jobs/{id}/cancel
GET /api/files/{id}/original
GET /api/files/{id}/hls/index.m3u8
GET /api/files/{id}/hls/{segment}
GET /api/files/{id}/subtitles
GET /api/files/{id}/preview/thumbnails.vtt
GET /api/files/{id}/preview/{asset}
GET /api/subtitles/{id}/track.vtt
GET /api/admin/users
POST /api/admin/users
```

In development, the bootstrap admin defaults to `admin` / `adminadmin`. Override it with `STREAMIZE_ADMIN_USERNAME` and `STREAMIZE_ADMIN_PASSWORD`.

Media workers use `ffmpeg` and `ffprobe` by default. Override them with `STREAMIZE_FFMPEG_PATH` and `STREAMIZE_FFPROBE_PATH` if the binaries live outside `PATH`.

## Web Development

The React + Vite UI lives in `apps/web` and implements the design-spec prototype as production React screens.

```bash
cd apps/web
npm install
npm run dev
```

The web dev server defaults to `http://localhost:5173` and proxies `/api` to the Go API at `http://localhost:8080`. If the API is not running, use the sign-in screen's prototype entry to browse the mock media flows.

## Docker Compose

Copy the example environment file and adjust values if needed:

```bash
cp deploy/.env.example deploy/.env
mkdir -p data/qbittorrent-config media/originals media/hls media/subtitles media/thumbnails media/tmp
chmod -R u+rwX,g+rwX data media
docker compose -f deploy/docker-compose.yml --env-file deploy/.env up --build
```

The Compose stack runs the Go API and qBittorrent. SQLite, qBittorrent config, and media files are bind-mounted into local `data/` and `media/` folders so downloads are visible on the host.
