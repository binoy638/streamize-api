# Backend Roadmap

## Milestone 1: Foundation

- Go API process with config, logging, health route, graceful shutdown.
- SQLite connection and migrations.
- Docker Compose with API and qBittorrent.
- Basic tests and local developer commands.

## Milestone 2: Auth and Admin

- Admin bootstrap from environment.
- Password hashing and session cookies.
- Admin-managed users and storage quotas.
- Auth middleware and authorization helpers.

## Milestone 3: Torrent Orchestration

- qBittorrent Web API client.
- Add magnet endpoint.
- Torrent/file metadata sync into SQLite.
- Delete, retry, pause/resume, and retention settings.

## Milestone 4: Jobs and Media Processing

- SQLite-backed job claims, leases, retries, and failures.
- ffprobe inspection.
- Source-quality HLS generation.
- Subtitle extraction.
- Thumbnail and preview generation.
- Cleanup jobs for original-file retention policy.

## Milestone 5: Playback and Sharing

- Authenticated HLS serving.
- Progress save/read endpoints.
- Expiring public share links for torrents and single videos.
- WebSocket status events.

## Milestone 6: Web UI

- React + Vite app in `apps/web`.
- Tailwind + shadcn component system.
- Media library, torrent detail, player, share creation, and basic admin screens.
