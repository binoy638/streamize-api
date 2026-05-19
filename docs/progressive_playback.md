# Progressive Playback Plan

## Summary

Implement three playback paths:

- Live HLS while transcoding for files whose HLS job is running.
- Direct original playback while queued for browser-compatible files that
  are downloaded but waiting for HLS.
- Direct original playback while downloading for browser-compatible files
  when metadata can be probed and enough bytes exist.

Default behavior:

- HLS remains the final canonical playback format.
- Direct original playback is only a temporary/fallback path.
- Downloading preview is direct-only, not live-HLS.

## Backend Changes

- Add early torrent-file ingestion for active torrents, not only completed
  torrents:
  - Poll qBittorrent file list for downloading, queued, and done
    torrents.
  - Create/update torrent_files rows before torrent completion.
  - Store per-file download progress, e.g. download_percent, so UI can
    show whether a file is partially available.
- Add a lightweight media probe step before HLS transcode:
  - Introduce a probe job or probe phase that runs before hls_transcode.
  - Probe completed or partially available files with ffprobe.
  - Persist container, codec, duration, processing mode, and
    directPlayable.
  - If probe fails because the file is incomplete, keep the file in a
    non-error “waiting for playable bytes” state and retry later.
- Change HLS worker output for running transcodes:
  - Write hls_path as soon as the worker starts and the first playlist
    can be served.
  - Use a live/event-style growing playlist during processing instead of
    requiring final VOD output first.
  - When FFmpeg completes, finalize/normalize the playlist as VOD and
    mark the file done.
- Relax HLS playback route readiness:
  - Serve HLS playlist/segments when file status is processing and
    hls_path exists.
  - Keep current done behavior unchanged.
  - Return 409 only when neither live HLS nor direct fallback is
    available.
- Relax direct original route readiness:
  - Allow /api/files/{id}/original for downloading, queued, processing,
    and done when directPlayable=true.
  - Keep path safety checks and range-capable http.ServeFile.
  - For downloading files, serve only if the original file exists on disk
    and metadata has confirmed browser support.

## Frontend Changes

- Update file/player types to include downloadPercent and richer playback
  readiness.
- Player source selection priority:
  1. HLS if hlsPath exists and status is processing or done.
  2. Direct original if directPlayable=true.
  3. Pending message otherwise.
- Player copy/status:
  - Show “Live HLS” while transcoding.
  - Show “Direct preview” when playing the original file before HLS is
    ready.
  - Show “Waiting for playable bytes” for incomplete downloads that
    cannot yet be probed or served.
- Library and torrent detail:
  - Show file-level download progress.
  - Enable Watch for direct-playable queued/downloading files and live-
    HLS processing files.
  - Continue polling while torrents/files/jobs are active.

## Test Plan

- Backend:
  - Active torrent ingestion creates/updates supported file rows before
    torrent completion.
  - Probe success marks MP4/H.264/AAC files direct-playable before HLS
    job runs.
  - Probe failure on incomplete files does not mark file error and can
    retry.
  - Direct original route serves queued/downloading direct-playable files
    and rejects unsupported codecs.
  - Completed transcode still marks file done and keeps final HLS
    playback working.
- Frontend:
  - Player chooses live HLS during processing.
  - Player chooses direct original for queued browser-compatible files.
  - Library/Torrent detail enable Watch for those states.

## Assumptions

- “Supported by browser” means MP4/M4V with H.264 video and AAC audio for
  v1.
- Downloading playback is best-effort and direct-only; files may not play
  until metadata and needed byte ranges are available.
- Live HLS is required for transcoding-in-progress, even though it is more
  complex than direct fallback.
- Final HLS output remains VOD-style after transcoding completes.
