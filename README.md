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
GET /api/admin/users
POST /api/admin/users
```

In development, the bootstrap admin defaults to `admin` / `adminadmin`. Override it with `STREAMIZE_ADMIN_USERNAME` and `STREAMIZE_ADMIN_PASSWORD`.

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
docker compose -f deploy/docker-compose.yml --env-file deploy/.env up --build
```

The initial Compose stack runs the Go API and qBittorrent. SQLite and media files are mounted as Docker volumes.
