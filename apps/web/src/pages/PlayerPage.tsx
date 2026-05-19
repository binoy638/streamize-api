import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { Maximize2, Pause, Play, Volume2 } from "lucide-react";
import type Hls from "hls.js";

import * as api from "../lib/api";
import { Badge, Button, Progress, StatCard } from "../components/ui";
import { formatBytes } from "../lib/format";
import { filesForTorrent, findMedia, torrentFiles } from "../lib/mock-data";

type PlayerFile = {
  id: string;
  source: "api" | "mock";
  torrentId: string;
  name: string;
  size: string;
  status: api.TorrentFileStatus | "ready" | "processing" | "failed";
  codec: string;
  subtitles: number | string;
  playable: boolean;
};

type HlsInstance = InstanceType<typeof Hls>;

export function PlayerPage() {
  const { fileId } = useParams();
  const videoRef = useRef<HTMLVideoElement | null>(null);
  const initialMedia = findMedia(fileId);
  const mockRelated = useMemo(() => {
    const files = filesForTorrent(initialMedia.torrentId);
    return (files.length ? files : torrentFiles.slice(0, 3)).map(mockFileToPlayerFile);
  }, [initialMedia.torrentId]);
  const [related, setRelated] = useState<PlayerFile[]>(mockRelated);
  const [selectedFileId, setSelectedFileId] = useState(fileId || related[0]?.id);
  const selectedFile = related.find((file) => file.id === selectedFileId) || related[0];
  const [playing, setPlaying] = useState(false);
  const [position, setPosition] = useState(0);
  const [durationSeconds, setDurationSeconds] = useState(0);
  const [usingMock, setUsingMock] = useState(true);
  const [error, setError] = useState("");
  const hlsSource = selectedFile?.source === "api" && selectedFile.playable ? hlsPlaylistURL(selectedFile.id) : "";

  const loadAPIFile = useCallback(async () => {
    if (!fileId) {
      return;
    }

    try {
      const torrents = await api.listTorrents();
      for (const torrent of torrents) {
        const files = await api.listTorrentFiles(torrent.id);
        const match = files.find((file) => file.id === fileId);
        if (!match) {
          continue;
        }

        setRelated(files.map((file) => apiFileToPlayerFile(file, torrent.id)));
        setSelectedFileId(match.id);
        setUsingMock(false);
        setError("");
        return;
      }

      throw new Error("File was not found in the API.");
    } catch (err) {
      setRelated(mockRelated);
      setSelectedFileId(fileId || mockRelated[0]?.id);
      setUsingMock(true);
      setError(err instanceof Error ? err.message : "Unable to load file.");
    }
  }, [fileId, mockRelated]);

  useEffect(() => {
    void loadAPIFile();
  }, [loadAPIFile]);

  useEffect(() => {
    const video = videoRef.current;
    if (!video || !hlsSource) {
      return;
    }

    setError("");
    setPlaying(false);
    setPosition(0);
    video.removeAttribute("src");
    video.load();

    if (video.canPlayType("application/vnd.apple.mpegurl")) {
      video.src = hlsSource;
      return () => {
        video.removeAttribute("src");
        video.load();
      };
    }

    let canceled = false;
    let hls: HlsInstance | null = null;

    void import("hls.js")
      .then(({ default: Hls }) => {
        if (canceled) {
          return;
        }
        if (!Hls.isSupported()) {
          setError("This browser cannot play HLS streams.");
          return;
        }

        hls = new Hls({
          xhrSetup: (xhr) => {
            xhr.withCredentials = true;
          },
        });
        hls.loadSource(hlsSource);
        hls.attachMedia(video);
        hls.on(Hls.Events.ERROR, (_, data) => {
          if (data.fatal) {
            setError("Playback failed while loading the HLS stream.");
          }
        });
      })
      .catch(() => {
        if (!canceled) {
          setError("Unable to load the HLS player.");
        }
      });

    return () => {
      canceled = true;
      hls?.destroy();
    };
  }, [hlsSource]);

  async function togglePlayback() {
    const video = videoRef.current;
    if (!video || !hlsSource) {
      return;
    }

    if (video.paused) {
      await video.play();
    } else {
      video.pause();
    }
  }

  function seek(percent: number) {
    const video = videoRef.current;
    setPosition(percent);
    if (!video || !Number.isFinite(video.duration) || video.duration <= 0) {
      return;
    }

    video.currentTime = (percent / 100) * video.duration;
  }

  return (
    <section className="content player-content">
      {usingMock && error ? (
        <div className="alert alert-warn is-visible">Using prototype player data because the API file could not load: {error}</div>
      ) : null}
      {!usingMock && error ? <div className="alert alert-error is-visible">{error}</div> : null}

      <div className="player-layout">
        <section className={`player-stage ${playing ? "is-playing" : ""}`} aria-label="Video player">
          {hlsSource ? (
            <video
              ref={videoRef}
              className="video-player"
              controls
              playsInline
              onPlay={() => setPlaying(true)}
              onPause={() => setPlaying(false)}
              onLoadedMetadata={(event) => setDurationSeconds(event.currentTarget.duration || 0)}
              onTimeUpdate={(event) => {
                const video = event.currentTarget;
                const duration = Number.isFinite(video.duration) && video.duration > 0 ? video.duration : durationSeconds;
                setPosition(duration > 0 ? Math.round((video.currentTime / duration) * 100) : 0);
              }}
            />
          ) : (
            <div className="player-frame">
              <div className="player-gradient">
                <span className="eyebrow">HLS Manifest</span>
                <strong>{selectedFile?.name || initialMedia.title}</strong>
                <p>{selectedFile?.playable ? "Loading stream." : "HLS output is not ready for this file yet."}</p>
              </div>
            </div>
          )}
          <div className="player-controls">
            <Button variant="primary" onClick={() => void togglePlayback()} aria-pressed={playing} disabled={!hlsSource}>
              {playing ? <Pause size={16} /> : <Play size={16} />} {playing ? "Pause" : "Play"}
            </Button>
            <input
              className="range"
              type="range"
              min="0"
              max="100"
              value={position}
              onChange={(event) => seek(Number(event.target.value))}
              aria-label="Playback position"
              disabled={!hlsSource}
            />
            <span className="mono">{position}%</span>
            <Button>
              <Volume2 size={16} /> Audio
            </Button>
            <Button>
              <Maximize2 size={16} /> Fullscreen
            </Button>
          </div>
        </section>

        <aside className="panel player-side">
          <div className="panel-header">
            <div>
              <div className="panel-title">Supported video files</div>
              <p className="muted">Switching files updates metadata and status without leaving playback.</p>
            </div>
          </div>
          <div className="file-list">
            {related.map((file) => (
              <button
                className={`file-row ${file.id === selectedFileId ? "active" : ""}`}
                key={file.id}
                onClick={() => setSelectedFileId(file.id)}
              >
                <span>
                  <strong>{file.name}</strong>
                  <small>
                    {file.size} · {file.codec}
                  </small>
                </span>
                <Badge tone={file.status}>{file.status}</Badge>
              </button>
            ))}
          </div>
          <div className="component-row">
            <Link className="btn btn-ghost" to={`/torrents/${selectedFile?.torrentId || initialMedia.torrentId}`}>
              Torrent detail
            </Link>
            <Button>Share</Button>
          </div>
        </aside>
      </div>

      <div className="stats-row">
        <StatCard label="Pieces" value="98%" detail="availability" />
        <StatCard label="Peers" value="18" detail="healthy swarm" />
        <StatCard label="Duration" value={formatDuration(durationSeconds)} detail="local HLS" />
        <StatCard label="Subtitles" value={String(selectedFile?.subtitles || 0)} detail="tracks detected" />
      </div>

      <div className="panel">
        <div className="timeline-list">
          <div className="timeline-item">
            <Badge tone="online">Direct</Badge>
            <div>
              <strong>{selectedFile?.codec || "H.264 / AAC"}</strong>
              <p className="muted">HLS manifest ready, source retained, previews generated.</p>
            </div>
            <Progress value={100} />
          </div>
          <div className="timeline-item">
            <Badge tone="ready">Subtitles</Badge>
            <div>
              <strong>English, Spanish, Japanese</strong>
              <p className="muted">Default track stays off until selected by the viewer.</p>
            </div>
            <Button>Manage</Button>
          </div>
        </div>
      </div>
    </section>
  );
}

function apiFileToPlayerFile(file: api.TorrentFile, torrentId: string): PlayerFile {
  return {
    id: file.id,
    source: "api",
    torrentId,
    name: file.name,
    size: file.sizeBytes > 0 ? formatBytes(file.sizeBytes) : "Pending",
    status: file.status,
    codec: extensionLabel(file.ext || file.name),
    subtitles: file.progressPreview ? "Preview ready" : "Pending",
    playable: file.status === "done" && Boolean(file.hlsPath),
  };
}

function mockFileToPlayerFile(file: (typeof torrentFiles)[number]): PlayerFile {
  return {
    id: file.id,
    source: "mock",
    torrentId: file.torrentId,
    name: file.name,
    size: file.size,
    status: file.status,
    codec: file.codec,
    subtitles: file.subtitles,
    playable: false,
  };
}

function hlsPlaylistURL(fileID: string): string {
  return `/api/files/${encodeURIComponent(fileID)}/hls/index.m3u8`;
}

function extensionLabel(value: string): string {
  const extension = value.includes(".") ? value.slice(value.lastIndexOf(".") + 1) : value;
  return extension ? extension.toUpperCase() : "Pending";
}

function formatDuration(totalSeconds: number): string {
  if (!Number.isFinite(totalSeconds) || totalSeconds <= 0) {
    return "Pending";
  }

  const seconds = Math.round(totalSeconds);
  const minutes = Math.floor(seconds / 60);
  const remainingSeconds = seconds % 60;
  if (minutes < 60) {
    return `${minutes}:${String(remainingSeconds).padStart(2, "0")}`;
  }

  const hours = Math.floor(minutes / 60);
  const remainingMinutes = minutes % 60;
  return `${hours}:${String(remainingMinutes).padStart(2, "0")}:${String(remainingSeconds).padStart(2, "0")}`;
}
