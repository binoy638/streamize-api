# Architecture

Streamize is moving from a single TypeScript API process to a Go monorepo designed for one self-hosted VPS.

## Runtime Shape

- Go API owns authentication, users, torrent ingestion, catalog metadata, media access, background jobs, and UI serving.
- SQLite stores app state and acts as the durable job queue.
- qBittorrent owns torrent protocol work and downloads into the shared media volume.
- ffmpeg is used by Go workers for inspection, HLS output, subtitles, and previews.
- TMDB and AniList are used by metadata jobs to identify downloaded movies, TV episodes, and anime releases.
- WebSockets will carry live status updates for downloads, jobs, and processing.

## Storage

The app treats `STREAMIZE_MEDIA_ROOT` as the root for local media files:

- `originals`: files downloaded by qBittorrent.
- `hls`: generated playlists and segments.
- `subtitles`: extracted or uploaded subtitles.
- `thumbnails`: preview images and sprite assets.
- `tmp`: temporary processing files.

SQLite lives outside the media root at `STREAMIZE_DATABASE_PATH`.

## Database

The initial schema is intentionally broad enough for the planned backend milestone:

- users and sessions
- torrents and torrent files
- catalog items and catalog episodes
- subtitles and video progress
- expiring shares
- durable jobs
- app settings

The first implementation uses hand-written migrations. Query generation with `sqlc` will be added when the first real repositories are implemented.

## Legacy Code

The previous Node/TypeScript implementation is under `legacy/api`. It is reference material only and should not receive new features.
