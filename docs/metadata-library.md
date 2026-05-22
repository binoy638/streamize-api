# Metadata Library

Streamize keeps torrent state and playable files separate from library metadata. Torrent ingestion records what was downloaded; metadata jobs decide what that file represents and attach it to catalog items.

## Goals

- Turn downloaded torrent files into a browsable library.
- Group TV episodes, movies, and anime releases under stable catalog records.
- Keep playback resilient when metadata cannot be identified.
- Allow manual correction and retry without re-downloading or re-transcoding files.

## Runtime Flow

1. qBittorrent reports files for an active or completed torrent.
2. Supported video files are inserted into `torrent_files`.
3. A `metadata_identify` job is queued for each new file.
4. On API startup, pending existing files without metadata jobs are backfilled.
5. The worker parses the file name and queries metadata providers.
6. Successful matches create or update `catalog_items` and, for episodic content, `catalog_episodes`.
7. The torrent file is linked to the matched catalog item or episode.
8. Unmatched files stay visible in the library as raw file entries.

The job queue uses dedupe keys, so repeated ingestion and restarts do not create duplicate metadata jobs.

## Database Shape

The metadata migration adds:

- `catalog_items`: one row per matched movie, TV series, anime series, or manually-created library item.
- `catalog_episodes`: optional season/episode or absolute-number records under a catalog item.
- metadata columns on `torrent_files`:
  - `catalog_item_id`
  - `catalog_episode_id`
  - `metadata_status`
  - `metadata_confidence`
  - `metadata_provider`
  - `metadata_error`

`metadata_status` can be:

- `pending`: file has not been identified yet.
- `matched`: file was matched automatically.
- `manual`: file was linked by a manual API correction.
- `unmatched`: provider lookup finished but did not reach the confidence threshold.
- `failed`: lookup could not complete because of configuration or provider errors.

## Providers

TMDB is the primary provider for movies and TV:

```env
STREAMIZE_TMDB_API_KEY=...
STREAMIZE_METADATA_LANGUAGE=en-US
```

AniList is used as a fallback for anime-like releases when TMDB cannot produce a confident match. AniList does not require an API key for the current lookup path.

If `STREAMIZE_TMDB_API_KEY` is not configured, non-anime metadata jobs are marked failed. This does not block HLS transcoding or playback. After configuring the key, retry metadata from the file row in the Library UI or call the refresh endpoint.

## API

All routes below require an authenticated user session.

```text
GET /api/library
```

Returns catalog-grouped library items. Matched files are grouped by catalog item; pending, failed, or unmatched files are returned as raw file-backed library entries.

```text
GET /api/metadata/search?query=...
```

Runs a conservative metadata lookup for a query and returns zero or one match. This is currently intended for manual correction workflows, not broad search browsing.

```text
POST /api/files/{id}/metadata-refresh
```

Queues or retries the `metadata_identify` job for a file owned by the current user.

```text
POST /api/files/{id}/metadata-match
```

Manually links a file to metadata. The request body can describe a movie/series item and, when applicable, episode fields:

```json
{
  "mediaType": "tv",
  "provider": "tmdb",
  "providerId": "123",
  "title": "Example Show",
  "releaseYear": 2026,
  "posterUrl": "https://image.tmdb.org/t/p/w500/example.jpg",
  "seasonNumber": 1,
  "episodeNumber": 2,
  "episodeTitle": "Episode Title"
}
```

## UI

The Library page now reads from `/api/library` instead of only torrent rows. It shows poster artwork when available, groups matched files under the same library item, and links multi-file items to a detail page.

The Library detail page lists the files attached to one catalog item and exposes per-file metadata refresh. Watch and details actions still use the existing playback routes.

The Jobs page includes a distinct visual treatment for `metadata_identify` jobs so metadata work is distinguishable from HLS, subtitle, and sprite jobs.

## Existing Downloads

Existing files are handled automatically after the metadata migration is applied:

- API startup queues metadata jobs for files still in `pending` status.
- Already matched or manually matched files are left alone.
- Failed or unmatched files are not reprocessed automatically; use refresh when you want to retry them.

This makes the backfill safe to run on every restart while still preserving manual corrections.

## Operational Notes

- Metadata lookup is best-effort. Playback and transcoding do not depend on a successful match.
- Automatic matching is intentionally conservative. Ambiguous files are left unmatched rather than attached to the wrong catalog item.
- The initial manual-match route is API-first. The UI currently exposes refresh, while richer correction/search screens can be layered on later.
- TMDB image URLs are stored as provider URLs and rendered directly by the web app.
