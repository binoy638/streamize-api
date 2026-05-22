import {
  type CSSProperties,
  type FormEvent,
  type PointerEvent,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { Link, useParams } from "react-router-dom";
import { Captions, Gauge, Maximize, Minimize, Pause, Play, Volume2, VolumeX } from "lucide-react";
import type Hls from "hls.js";

import * as api from "../lib/api";
import { Badge, Button, Field, Input, Modal, Select } from "../components/ui";
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

type HlsInstance = InstanceType<typeof Hls>;

type ControlMenu = "speed" | "cc" | null;

type PreviewCue = {
  start: number;
  end: number;
  image: string;
  x: number;
  y: number;
  width: number;
  height: number;
};

const PLAYBACK_RATES = [0.5, 0.75, 1, 1.25, 1.5, 2];

export function PlayerPage() {
  const { fileId } = useParams();
  const videoRef = useRef<HTMLVideoElement | null>(null);
  const stageRef = useRef<HTMLElement | null>(null);
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
  const [currentSeconds, setCurrentSeconds] = useState(0);
  const [durationSeconds, setDurationSeconds] = useState(0);
  const [volume, setVolume] = useState(1);
  const [muted, setMuted] = useState(false);
  const [playbackRate, setPlaybackRate] = useState(1);
  const [activeTrack, setActiveTrack] = useState(-1);
  const [isFullscreen, setIsFullscreen] = useState(false);
  const [menu, setMenu] = useState<ControlMenu>(null);
  const [usingMock, setUsingMock] = useState(true);
  const [error, setError] = useState("");
  const [subtitles, setSubtitles] = useState<api.Subtitle[]>([]);
  const [previewCues, setPreviewCues] = useState<PreviewCue[]>([]);
  const [previewPercent, setPreviewPercent] = useState<number | null>(null);
  const [partyOpen, setPartyOpen] = useState(false);
  const [partyMode, setPartyMode] = useState<api.WatchPartyControlMode>("host_only");
  const [partyDisplayName, setPartyDisplayName] = useState("");
  const [partyBusy, setPartyBusy] = useState(false);
  const [partyError, setPartyError] = useState("");
  const [partyLink, setPartyLink] = useState("");
  const [activeParty, setActiveParty] = useState<api.WatchPartyResponse | null>(null);
  const hlsSource = selectedFile?.source === "api" && selectedFile.playable ? hlsPlaylistURL(selectedFile.id) : "";
  const directSource = selectedFile?.source === "api" && !hlsSource && selectedFile.directPlayable ? originalFileURL(selectedFile.id) : "";
  const playbackSource = hlsSource || directSource;
  const previewStyle = selectedFile?.source === "api" && selectedFile.previewReady && previewPercent !== null && durationSeconds > 0
    ? spritePreviewStyle(previewCues, previewPercent, durationSeconds)
    : undefined;

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
    setDurationSeconds(0);
    if (!video || !playbackSource) {
      return;
    }

    setError("");
    setPlaying(false);
    setPosition(0);
    setCurrentSeconds(0);
    video.removeAttribute("src");
    video.load();

    if (directSource) {
      video.src = directSource;
      return () => {
        video.removeAttribute("src");
        video.load();
      };
    }

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
  }, [directSource, hlsSource, playbackSource]);

  // Keep volume / mute / speed applied to the element, including after a source swap.
  useEffect(() => {
    const video = videoRef.current;
    if (!video) {
      return;
    }
    video.volume = volume;
    video.muted = muted;
    video.playbackRate = playbackRate;
  }, [volume, muted, playbackRate, playbackSource]);

  // Drive subtitle visibility ourselves since the native controls are hidden.
  useEffect(() => {
    const video = videoRef.current;
    if (!video) {
      return;
    }
    const tracks = video.textTracks;
    for (let index = 0; index < tracks.length; index += 1) {
      tracks[index].mode = index === activeTrack ? "showing" : "disabled";
    }
  }, [activeTrack, subtitles, playbackSource]);

  useEffect(() => {
    const onFullscreenChange = () => setIsFullscreen(Boolean(document.fullscreenElement));
    document.addEventListener("fullscreenchange", onFullscreenChange);
    return () => document.removeEventListener("fullscreenchange", onFullscreenChange);
  }, []);

  // Close an open control menu when clicking elsewhere.
  useEffect(() => {
    if (!menu) {
      return;
    }
    const onPointerDown = (event: globalThis.PointerEvent) => {
      if (!(event.target as HTMLElement).closest(".ctrl-menu-host")) {
        setMenu(null);
      }
    };
    document.addEventListener("pointerdown", onPointerDown);
    return () => document.removeEventListener("pointerdown", onPointerDown);
  }, [menu]);

  useEffect(() => {
    setSubtitles([]);
    setActiveTrack(-1);
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

  useEffect(() => {
    setPreviewCues([]);
    if (selectedFile?.source !== "api" || !selectedFile.previewReady) {
      return;
    }

    let canceled = false;
    fetch(previewVTTURL(selectedFile.id), {
      credentials: "include",
      headers: { Accept: "text/vtt" },
    })
      .then((response) => (response.ok ? response.text() : ""))
      .then((body) => {
        if (!canceled) {
          setPreviewCues(parsePreviewVTT(body));
        }
      })
      .catch(() => {
        if (!canceled) {
          setPreviewCues([]);
        }
      });

    return () => {
      canceled = true;
    };
  }, [selectedFile?.id, selectedFile?.previewReady, selectedFile?.source]);

  async function togglePlayback() {
    const video = videoRef.current;
    if (!video || !playbackSource) {
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

  function changeVolume(value: number) {
    setVolume(value);
    setMuted(value === 0);
  }

  function toggleMute() {
    if (muted && volume === 0) {
      setVolume(0.5);
      setMuted(false);
      return;
    }
    setMuted((current) => !current);
  }

  function changeRate(rate: number) {
    setPlaybackRate(rate);
    setMenu(null);
  }

  function chooseSubtitle(index: number) {
    setActiveTrack(index);
    setMenu(null);
  }

  function toggleFullscreen() {
    if (document.fullscreenElement) {
      void document.exitFullscreen();
    } else {
      void stageRef.current?.requestFullscreen();
    }
  }

  function updatePreviewFromPointer(event: PointerEvent<HTMLInputElement>) {
    if (!selectedFile?.previewReady) {
      setPreviewPercent(null);
      return;
    }
    const rect = event.currentTarget.getBoundingClientRect();
    const nextPercent = ((event.clientX - rect.left) / rect.width) * 100;
    setPreviewPercent(Math.max(0, Math.min(nextPercent, 100)));
  }

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

  const volumePercent = muted ? 0 : volume;
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
        <section className="player-stage" ref={stageRef} aria-label="Video player">
          {playbackSource ? (
            <video
              ref={videoRef}
              className="video-player"
              playsInline
              onClick={() => void togglePlayback()}
              onPlay={() => setPlaying(true)}
              onPause={() => setPlaying(false)}
              onLoadedMetadata={(event) => setDurationSeconds(event.currentTarget.duration || 0)}
              onTimeUpdate={(event) => {
                const video = event.currentTarget;
                const duration = Number.isFinite(video.duration) && video.duration > 0 ? video.duration : durationSeconds;
                setCurrentSeconds(video.currentTime || 0);
                setPosition(duration > 0 ? (video.currentTime / duration) * 100 : 0);
              }}
            >
              {subtitles.map((subtitle) => (
                <track
                  key={subtitle.id}
                  kind="subtitles"
                  src={subtitle.url}
                  srcLang={subtitle.language}
                  label={subtitle.title || subtitle.language.toUpperCase()}
                />
              ))}
            </video>
          ) : (
            <div className="player-frame">
              <div className="player-gradient">
                <span className="eyebrow">Playback</span>
                <strong>{selectedFile?.name || initialMedia.title}</strong>
                <p>HLS output is not ready and the original file is not directly playable yet.</p>
              </div>
            </div>
          )}

          {playbackSource ? (
            <div className="player-controls">
              <span className="scrub-host" onPointerLeave={() => setPreviewPercent(null)}>
                {previewStyle ? <span className="scrub-preview" style={previewStyle} /> : null}
                <input
                  className="range"
                  type="range"
                  min="0"
                  max="100"
                  step="0.1"
                  value={position}
                  style={{ "--range-fill": `${position}%` } as CSSProperties}
                  onChange={(event) => seek(Number(event.target.value))}
                  onPointerMove={updatePreviewFromPointer}
                  aria-label="Seek"
                />
              </span>

              <div className="control-row">
                <div className="control-cluster">
                  <button
                    className="ctrl-btn primary"
                    type="button"
                    onClick={() => void togglePlayback()}
                    aria-label={playing ? "Pause" : "Play"}
                  >
                    {playing ? <Pause size={19} /> : <Play size={19} />}
                  </button>
                  <div className="volume">
                    <button
                      className="ctrl-btn"
                      type="button"
                      onClick={toggleMute}
                      aria-label={volumePercent === 0 ? "Unmute" : "Mute"}
                    >
                      {volumePercent === 0 ? <VolumeX size={18} /> : <Volume2 size={18} />}
                    </button>
                    <input
                      className="range volume-range"
                      type="range"
                      min="0"
                      max="1"
                      step="0.05"
                      value={volumePercent}
                      style={{ "--range-fill": `${volumePercent * 100}%` } as CSSProperties}
                      onChange={(event) => changeVolume(Number(event.target.value))}
                      aria-label="Volume"
                    />
                  </div>
                  <span className="timecode">
                    {formatClock(currentSeconds)} <i>/</i> {formatClock(durationSeconds)}
                  </span>
                </div>

                <div className="control-cluster">
                  <span className="ctrl-menu-host">
                    <button
                      className="ctrl-btn"
                      type="button"
                      onClick={() => setMenu(menu === "speed" ? null : "speed")}
                      aria-label="Playback speed"
                    >
                      <Gauge size={17} />
                      <span className="ctrl-label">{playbackRate}×</span>
                    </button>
                    {menu === "speed" ? (
                      <div className="ctrl-menu">
                        {PLAYBACK_RATES.map((rate) => (
                          <button
                            key={rate}
                            type="button"
                            className={rate === playbackRate ? "active" : ""}
                            onClick={() => changeRate(rate)}
                          >
                            {rate}× {rate === 1 ? "(Normal)" : ""}
                          </button>
                        ))}
                      </div>
                    ) : null}
                  </span>

                  <span className="ctrl-menu-host">
                    <button
                      className="ctrl-btn"
                      type="button"
                      onClick={() => setMenu(menu === "cc" ? null : "cc")}
                      aria-label="Subtitles"
                      disabled={subtitles.length === 0}
                    >
                      <Captions size={18} />
                    </button>
                    {menu === "cc" ? (
                      <div className="ctrl-menu">
                        <button
                          type="button"
                          className={activeTrack === -1 ? "active" : ""}
                          onClick={() => chooseSubtitle(-1)}
                        >
                          Off
                        </button>
                        {subtitles.map((subtitle, index) => (
                          <button
                            key={subtitle.id}
                            type="button"
                            className={activeTrack === index ? "active" : ""}
                            onClick={() => chooseSubtitle(index)}
                          >
                            {subtitle.title || subtitle.language.toUpperCase()}
                          </button>
                        ))}
                      </div>
                    ) : null}
                  </span>

                  <button
                    className="ctrl-btn"
                    type="button"
                    onClick={toggleFullscreen}
                    aria-label={isFullscreen ? "Exit fullscreen" : "Fullscreen"}
                  >
                    {isFullscreen ? <Minimize size={18} /> : <Maximize size={18} />}
                  </button>
                </div>
              </div>
            </div>
          ) : null}
        </section>

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
          <strong>{previewCues.length > 0 ? "Sprites ready" : "Pending"}</strong>
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

function spritePreviewStyle(cues: PreviewCue[], percent: number, durationSeconds: number): CSSProperties | undefined {
  if (cues.length === 0) {
    return undefined;
  }

  const seconds = (percent / 100) * durationSeconds;
  const cue = cues.find((candidate) => seconds >= candidate.start && seconds < candidate.end) || cues[cues.length - 1];
  return {
    backgroundImage: `url(${cue.image})`,
    backgroundPosition: `-${cue.x}px -${cue.y}px`,
    height: `${cue.height}px`,
    left: `${Math.max(0, Math.min(percent, 100))}%`,
    width: `${cue.width}px`,
  };
}

function parsePreviewVTT(body: string): PreviewCue[] {
  const lines = body.split(/\r?\n/);
  const cues: PreviewCue[] = [];

  for (let index = 0; index < lines.length; index += 1) {
    const line = lines[index].trim();
    if (!line.includes("-->")) {
      continue;
    }

    const [startText, endText] = line.split("-->").map((part) => part.trim());
    const start = parseVTTTime(startText);
    const end = parseVTTTime(endText);
    if (start === null || end === null || end <= start) {
      continue;
    }

    let assetLine = "";
    for (let assetIndex = index + 1; assetIndex < lines.length; assetIndex += 1) {
      const candidate = lines[assetIndex].trim();
      if (!candidate) {
        break;
      }
      assetLine = candidate;
      break;
    }

    const [image, fragment = ""] = assetLine.split("#xywh=");
    const coordinates = fragment.split(",").map((value) => Number(value));
    if (!image || coordinates.length !== 4 || coordinates.some((value) => !Number.isFinite(value))) {
      continue;
    }

    cues.push({
      start,
      end,
      image,
      x: coordinates[0],
      y: coordinates[1],
      width: coordinates[2],
      height: coordinates[3],
    });
  }

  return cues;
}

function parseVTTTime(value: string): number | null {
  const parts = value.split(":");
  if (parts.length !== 3) {
    return null;
  }

  const hours = Number(parts[0]);
  const minutes = Number(parts[1]);
  const seconds = Number(parts[2]);
  if (![hours, minutes, seconds].every(Number.isFinite)) {
    return null;
  }

  return hours * 3600 + minutes * 60 + seconds;
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

function formatClock(totalSeconds: number): string {
  if (!Number.isFinite(totalSeconds) || totalSeconds < 0) {
    return "0:00";
  }

  const seconds = Math.floor(totalSeconds);
  const secs = seconds % 60;
  const mins = Math.floor(seconds / 60) % 60;
  const hours = Math.floor(seconds / 3600);
  const pad = (value: number) => String(value).padStart(2, "0");
  return hours > 0 ? `${hours}:${pad(mins)}:${pad(secs)}` : `${mins}:${pad(secs)}`;
}
