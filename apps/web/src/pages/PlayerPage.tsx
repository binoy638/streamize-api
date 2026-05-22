import { type FormEvent, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Link, useParams } from "react-router-dom";

import * as api from "../lib/api";
import { Badge, Button, Field, Input, Modal, Select } from "../components/ui";
import { VideoPlayer } from "../components/VideoPlayer";
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
  directPlayable: boolean;
  previewReady: boolean;
};

const PROGRESS_SAVE_INTERVAL_MS = 10_000;
const PROGRESS_SAVE_DELTA_SECONDS = 5;
const RESUME_SKIP_AT_END_SECONDS = 8;

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
  const [durationSeconds, setDurationSeconds] = useState(0);
  const [usingMock, setUsingMock] = useState(true);
  const [error, setError] = useState("");
  const [subtitles, setSubtitles] = useState<api.Subtitle[]>([]);
  const [savedProgress, setSavedProgress] = useState<api.VideoProgress | null>(null);
  const [partyOpen, setPartyOpen] = useState(false);
  const [partyMode, setPartyMode] = useState<api.WatchPartyControlMode>("host_only");
  const [partyDisplayName, setPartyDisplayName] = useState("");
  const [partyBusy, setPartyBusy] = useState(false);
  const [partyError, setPartyError] = useState("");
  const [partyLink, setPartyLink] = useState("");
  const [activeParty, setActiveParty] = useState<api.WatchPartyResponse | null>(null);
  const resumeAppliedRef = useRef("");
  const lastProgressSaveRef = useRef({ fileId: "", positionSeconds: 0, savedAt: 0 });
  const hlsSource = selectedFile?.source === "api" && selectedFile.playable ? hlsPlaylistURL(selectedFile.id) : "";
  const directSource = selectedFile?.source === "api" && !hlsSource && selectedFile.directPlayable ? originalFileURL(selectedFile.id) : "";
  const playbackSource = hlsSource || directSource;
  const previewVTT = selectedFile?.source === "api" && selectedFile.previewReady ? previewVTTURL(selectedFile.id) : undefined;

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
    setSavedProgress(null);
    resumeAppliedRef.current = "";
    lastProgressSaveRef.current = { fileId: selectedFile?.id || "", positionSeconds: 0, savedAt: 0 };

    if (selectedFile?.source !== "api" || !playbackSource) {
      return;
    }

    let canceled = false;
    api
      .getVideoProgress(selectedFile.id)
      .then((progress) => {
        if (!canceled) {
          setSavedProgress(progress);
        }
      })
      .catch(() => {
        if (!canceled) {
          setSavedProgress(null);
        }
      });

    return () => {
      canceled = true;
    };
  }, [selectedFile?.id, selectedFile?.source, playbackSource]);

  useEffect(() => {
    setSubtitles([]);
    if (selectedFile?.source !== "api") {
      return;
    }

    let canceled = false;
    api
      .listSubtitles(selectedFile.id)
      .then((records) => {
        if (!canceled) {
          setSubtitles(records);
        }
      })
      .catch(() => {
        if (!canceled) {
          setSubtitles([]);
        }
      });

    return () => {
      canceled = true;
    };
  }, [selectedFile?.id, selectedFile?.source]);

  const persistProgress = useCallback(
    (positionValue: number, durationValue: number, force = false) => {
      if (!selectedFile || selectedFile.source !== "api" || !playbackSource) {
        return;
      }

      const fileId = selectedFile.id;
      const nextPosition = normalizeProgressSeconds(positionValue);
      const nextDuration = normalizeProgressSeconds(durationValue);
      const now = Date.now();
      const last = lastProgressSaveRef.current;
      if (
        !force &&
        last.fileId === fileId &&
        now - last.savedAt < PROGRESS_SAVE_INTERVAL_MS &&
        Math.abs(nextPosition - last.positionSeconds) < PROGRESS_SAVE_DELTA_SECONDS
      ) {
        return;
      }

      lastProgressSaveRef.current = { fileId, positionSeconds: nextPosition, savedAt: now };
      void api.saveVideoProgress(fileId, {
        positionSeconds: nextPosition,
        durationSeconds: nextDuration,
      }).catch(() => undefined);
    },
    [playbackSource, selectedFile?.id, selectedFile?.source],
  );

  const applySavedProgress = useCallback(
    (video: HTMLVideoElement) => {
      if (
        !selectedFile ||
        selectedFile.source !== "api" ||
        !savedProgress ||
        savedProgress.torrentFileId !== selectedFile.id ||
        resumeAppliedRef.current === selectedFile.id
      ) {
        return;
      }

      const savedSeconds = normalizeProgressSeconds(savedProgress.positionSeconds);
      if (savedSeconds < 1) {
        resumeAppliedRef.current = selectedFile.id;
        return;
      }
      if (video.readyState < 1) {
        return;
      }
      if (normalizeProgressSeconds(video.currentTime) > 1) {
        resumeAppliedRef.current = selectedFile.id;
        return;
      }

      const duration = normalizeProgressSeconds(video.duration || savedProgress.durationSeconds);
      if (duration > 0 && savedSeconds >= Math.max(duration - RESUME_SKIP_AT_END_SECONDS, 0)) {
        resumeAppliedRef.current = selectedFile.id;
        return;
      }

      video.currentTime = duration > 0 ? Math.min(savedSeconds, Math.max(duration - 1, 0)) : savedSeconds;
      resumeAppliedRef.current = selectedFile.id;
    },
    [savedProgress, selectedFile?.id, selectedFile?.source],
  );

  useEffect(() => {
    const video = videoRef.current;
    if (!video || !playbackSource) {
      return;
    }
    applySavedProgress(video);
  }, [applySavedProgress, playbackSource]);

  useEffect(() => {
    const onPageHide = () => {
      const video = videoRef.current;
      if (video) {
        persistProgress(video.currentTime, video.duration, true);
      }
    };

    window.addEventListener("pagehide", onPageHide);
    return () => window.removeEventListener("pagehide", onPageHide);
  }, [persistProgress]);

  async function submitWatchParty(event: FormEvent) {
    event.preventDefault();
    setPartyError("");
    setPartyLink("");
    if (!selectedFile || selectedFile.source !== "api") {
      setPartyError("Watch parties can be created after this file loads from the API.");
      return;
    }
    if (!selectedFile.playable && !selectedFile.directPlayable) {
      setPartyError("This file is not ready for watch party playback yet.");
      return;
    }

    setPartyBusy(true);
    try {
      const response = await api.createWatchParty({
        torrentFileId: selectedFile.id,
        controlMode: partyMode,
        displayName: partyDisplayName.trim() || undefined,
      });
      if (response.session) {
        sessionStorage.setItem(watchPartySessionStorageKey(response.party.slug), JSON.stringify(response.session));
      }
      const localPartyLink = watchPartyURL(response.party.slug);
      setActiveParty({ ...response, joinUrl: localPartyLink });
      setPartyLink(localPartyLink);
      await navigator.clipboard?.writeText(localPartyLink);
    } catch (err) {
      setPartyError(err instanceof Error ? err.message : "Unable to create watch party.");
    } finally {
      setPartyBusy(false);
    }
  }

  const sourceLabel = hlsSource ? "HLS stream" : directSource ? "Direct file" : "Not ready";

  return (
    <section className="content player-content">
      {usingMock && error ? (
        <div className="alert alert-warn is-visible">Using prototype player data because the API file could not load: {error}</div>
      ) : null}
      {!usingMock && error ? <div className="alert alert-error is-visible">{error}</div> : null}
      {activeParty ? (
        <div className="watch-party-banner">
          <div className="watch-party-banner-copy">
            <Badge tone="online">Watch party active</Badge>
            <strong>{activeParty.file.name}</strong>
            <p className="muted">
              {activeParty.party.controlMode === "everyone" ? "Everyone can control playback." : "Only the host can control playback."}
            </p>
          </div>
          <div className="component-row">
            <Input readOnly value={activeParty.joinUrl} aria-label="Watch party link" />
            <Button onClick={() => void navigator.clipboard?.writeText(activeParty.joinUrl)}>Copy</Button>
            <Link className="btn btn-primary" to={watchPartyPath(activeParty.party.slug)}>
              Open host room
            </Link>
          </div>
        </div>
      ) : null}

      <div className="player-layout">
        <VideoPlayer
          source={playbackSource}
          isDirect={Boolean(directSource)}
          withCredentials
          subtitles={subtitles}
          previewVTTURL={previewVTT}
          previewWithCredentials
          videoRef={videoRef}
          fallback={
            <div className="player-frame">
              <div className="player-gradient">
                <span className="eyebrow">Playback</span>
                <strong>{selectedFile?.name || initialMedia.title}</strong>
                <p>HLS output is not ready and the original file is not directly playable yet.</p>
              </div>
            </div>
          }
          onPause={() => {
            const video = videoRef.current;
            if (video) {
              persistProgress(video.currentTime, video.duration, true);
            }
          }}
          onEnded={() => {
            const video = videoRef.current;
            if (video) {
              persistProgress(0, video.duration, true);
            }
          }}
          onLoadedMetadata={(video) => {
            setDurationSeconds(normalizeProgressSeconds(video.duration));
            applySavedProgress(video);
          }}
          onTimeUpdate={(currentTime, duration) => persistProgress(currentTime, duration)}
          onError={(message) => setError(message)}
        />

        <aside className="panel player-side">
          <div className="panel-header">
            <div>
              <div className="panel-title">Files</div>
              <p className="muted">
                {related.length} file{related.length === 1 ? "" : "s"} in this torrent
              </p>
            </div>
          </div>
          <div className="file-list">
            {related.map((file) => (
              <button
                className={`file-row ${file.id === selectedFileId ? "active" : ""}`}
                key={file.id}
                onClick={() => setSelectedFileId(file.id)}
              >
                <span className="file-row-copy">
                  <strong>{file.name}</strong>
                  <small>
                    {file.size} · {file.codec}
                  </small>
                </span>
                <Badge tone={file.status}>{file.status}</Badge>
              </button>
            ))}
          </div>
          <div className="player-side-actions">
            <Link className="btn flex-1" to={`/torrents/${selectedFile?.torrentId || initialMedia.torrentId}`}>
              Torrent detail
            </Link>
            <Button variant="primary" className="flex-1" onClick={() => setPartyOpen(true)}>
              Watch party
            </Button>
          </div>
        </aside>
      </div>

      <div className="player-meta">
        <div className="player-meta-item">
          <span>Source</span>
          <strong>{sourceLabel}</strong>
        </div>
        <div className="player-meta-item">
          <span>Duration</span>
          <strong>{formatDuration(durationSeconds)}</strong>
        </div>
        <div className="player-meta-item">
          <span>Codec</span>
          <strong>{selectedFile?.codec || "—"}</strong>
        </div>
        <div className="player-meta-item">
          <span>Subtitles</span>
          <strong>
            {subtitles.length > 0 ? `${subtitles.length} track${subtitles.length === 1 ? "" : "s"}` : "None"}
          </strong>
        </div>
        <div className="player-meta-item">
          <span>Size</span>
          <strong>{selectedFile?.size || "—"}</strong>
        </div>
        <div className="player-meta-item">
          <span>Preview</span>
          <strong>{selectedFile?.previewReady ? "Sprites ready" : "Pending"}</strong>
        </div>
      </div>

      <Modal
        title="Create watch party"
        open={partyOpen}
        onClose={() => {
          if (!partyBusy) {
            setPartyOpen(false);
          }
        }}
        footer={
          <>
            <Button type="button" onClick={() => setPartyOpen(false)} disabled={partyBusy}>
              Close
            </Button>
            <Button variant="primary" type="submit" form="create-watch-party-form" disabled={partyBusy || Boolean(partyLink)}>
              {partyBusy ? "Creating..." : partyLink ? "Created" : "Create"}
            </Button>
          </>
        }
      >
        <form id="create-watch-party-form" className="contents" onSubmit={submitWatchParty}>
          {partyError ? <div className="alert alert-error is-visible">{partyError}</div> : null}
          <Field label="Display name">
            <Input
              value={partyDisplayName}
              onChange={(event) => setPartyDisplayName(event.target.value)}
              placeholder="Shown as host"
            />
          </Field>
          <Field label="Playback control">
            <Select value={partyMode} onChange={(event) => setPartyMode(event.target.value as api.WatchPartyControlMode)}>
              <option value="host_only">Host only</option>
              <option value="everyone">Everyone can control</option>
            </Select>
          </Field>
          {partyLink ? (
            <div className="form-grid">
              <div className="alert alert-success is-visible">Watch party created and link copied.</div>
              <div className="share-link">
                <Input readOnly value={partyLink} />
                <Link className="btn btn-primary" to={activeParty ? watchPartyPath(activeParty.party.slug) : "/library"}>
                  Open
                </Link>
              </div>
            </div>
          ) : null}
        </form>
      </Modal>
    </section>
  );
}

function watchPartySessionStorageKey(slug: string): string {
  return `watchPartySession:${slug}`;
}

function watchPartyPath(slug: string): string {
  return `/watch/${encodeURIComponent(slug)}`;
}

function watchPartyURL(slug: string): string {
  return `${window.location.origin}${watchPartyPath(slug)}`;
}

function apiFileToPlayerFile(file: api.TorrentFile, torrentId: string): PlayerFile {
  return {
    id: file.id,
    source: "api",
    torrentId,
    name: file.name,
    size: file.sizeBytes > 0 ? formatBytes(file.sizeBytes) : "Pending",
    status: file.status,
    codec: codecLabel(file),
    subtitles: 0,
    playable: (file.status === "done" || file.status === "processing") && Boolean(file.hlsPath),
    directPlayable: file.directPlayable,
    previewReady: file.progressPreview,
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
    directPlayable: false,
    previewReady: false,
  };
}

function hlsPlaylistURL(fileID: string): string {
  return `/api/files/${encodeURIComponent(fileID)}/hls/index.m3u8`;
}

function originalFileURL(fileID: string): string {
  return `/api/files/${encodeURIComponent(fileID)}/original`;
}

function previewVTTURL(fileID: string): string {
  return `/api/files/${encodeURIComponent(fileID)}/preview/thumbnails.vtt`;
}

function codecLabel(file: api.TorrentFile): string {
  const codecs = [file.videoCodec, file.audioCodec].filter(Boolean);
  if (codecs.length > 0) {
    return codecs.map((codec) => String(codec).toUpperCase()).join(" / ");
  }
  return extensionLabel(file.ext || file.name);
}

function extensionLabel(value: string): string {
  const extension = value.includes(".") ? value.slice(value.lastIndexOf(".") + 1) : value;
  return extension ? extension.toUpperCase() : "Pending";
}

function normalizeProgressSeconds(value: number): number {
  return Number.isFinite(value) && value > 0 ? value : 0;
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
