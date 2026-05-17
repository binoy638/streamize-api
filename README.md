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
GET /api/admin/users
POST /api/admin/users
```

In development, the bootstrap admin defaults to `admin` / `adminadmin`. Override it with `STREAMIZE_ADMIN_USERNAME` and `STREAMIZE_ADMIN_PASSWORD`.

## Docker Compose

Copy the example environment file and adjust values if needed:

```bash
cp deploy/.env.example deploy/.env
docker compose -f deploy/docker-compose.yml --env-file deploy/.env up --build
```

The initial Compose stack runs the Go API and qBittorrent. SQLite and media files are mounted as Docker volumes.
