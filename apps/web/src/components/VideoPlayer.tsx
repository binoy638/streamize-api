import {
  type CSSProperties,
  type MutableRefObject,
  type PointerEvent,
  type ReactNode,
  useCallback,
  useEffect,
  useRef,
  useState,
} from "react";
import { Captions, Gauge, Maximize, Minimize, Pause, Play, Volume2, VolumeX } from "lucide-react";
import type Hls from "hls.js";

import type * as api from "../lib/api";

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
const CONTROLS_HIDE_DELAY_MS = 2800;

export type VideoPlayerProps = {
  /** HLS playlist URL or a directly-playable file URL. Empty renders the fallback. */
  source: string;
  /** True when `source` is a progressive file rather than an HLS playlist. */
  isDirect?: boolean;
  /** Send cookies on HLS segment requests (needed for the authenticated player). */
  withCredentials?: boolean;
  subtitles?: api.Subtitle[];
  /** Optional sprite-thumbnail VTT used for scrub previews. */
  previewVTTURL?: string;
  previewWithCredentials?: boolean;
  /** Disables play/pause, seeking and speed — used for watch-party guests. */
  transportLocked?: boolean;
  /** Shown in place of the video when `source` is empty. */
  fallback?: ReactNode;
  /** Optional external ref kept pointed at the underlying <video> element. */
  videoRef?: MutableRefObject<HTMLVideoElement | null>;
  onPlay?: () => void;
  onPause?: () => void;
  onSeeked?: () => void;
  onEnded?: () => void;
  onTimeUpdate?: (currentTime: number, duration: number) => void;
  onLoadedMetadata?: (video: HTMLVideoElement) => void;
  onError?: (message: string) => void;
};

// VideoPlayer renders a <video> element with the app's custom control bar:
// scrubber + sprite preview, play/pause, volume, speed, subtitles, fullscreen,
// and idle auto-hide. It is shared by the library player, share links, and
// watch parties so playback looks identical everywhere.
export function VideoPlayer({
  source,
  isDirect = false,
  withCredentials = false,
  subtitles = [],
  previewVTTURL,
  previewWithCredentials = false,
  transportLocked = false,
  fallback,
  videoRef,
  onPlay,
  onPause,
  onSeeked,
  onEnded,
  onTimeUpdate,
  onLoadedMetadata,
  onError,
}: VideoPlayerProps) {
  const internalVideoRef = useRef<HTMLVideoElement | null>(null);
  const stageRef = useRef<HTMLElement | null>(null);
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
  const [controlsVisible, setControlsVisible] = useState(true);
  const [previewCues, setPreviewCues] = useState<PreviewCue[]>([]);
  const [previewPercent, setPreviewPercent] = useState<number | null>(null);
  const hideControlsTimerRef = useRef<number | null>(null);
  const pointerOverControlsRef = useRef(false);

  // Keep both the internal ref and any caller-supplied ref pointed at the node.
  const setVideoNode = useCallback(
    (node: HTMLVideoElement | null) => {
      internalVideoRef.current = node;
      if (videoRef) {
        videoRef.current = node;
      }
    },
    [videoRef],
  );

  // Attach the playback source, loading hls.js when the browser cannot play
  // HLS natively.
  useEffect(() => {
    const video = internalVideoRef.current;
    setDurationSeconds(0);
    setPlaying(false);
    setPosition(0);
    setCurrentSeconds(0);
    if (!video || !source) {
      return;
    }

    video.removeAttribute("src");
    video.load();

    if (isDirect || video.canPlayType("application/vnd.apple.mpegurl")) {
      video.src = source;
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
          onError?.("This browser cannot play HLS streams.");
          return;
        }

        hls = new Hls({
          xhrSetup: (xhr) => {
            xhr.withCredentials = withCredentials;
          },
        });
        hls.loadSource(source);
        hls.attachMedia(video);
        hls.on(Hls.Events.ERROR, (_, data) => {
          if (data.fatal) {
            onError?.("Playback failed while loading the HLS stream.");
          }
        });
      })
      .catch(() => {
        if (!canceled) {
          onError?.("Unable to load the HLS player.");
        }
      });

    return () => {
      canceled = true;
      hls?.destroy();
    };
    // onError is intentionally excluded; callers pass a fresh closure each render.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [source, isDirect, withCredentials]);

  // Keep volume / mute / speed applied to the element, including after a swap.
  useEffect(() => {
    const video = internalVideoRef.current;
    if (!video) {
      return;
    }
    video.volume = volume;
    video.muted = muted;
    video.playbackRate = playbackRate;
  }, [volume, muted, playbackRate, source]);

  // Drive subtitle visibility ourselves since native controls are hidden.
  useEffect(() => {
    const video = internalVideoRef.current;
    if (!video) {
      return;
    }
    const tracks = video.textTracks;
    for (let index = 0; index < tracks.length; index += 1) {
      tracks[index].mode = index === activeTrack ? "showing" : "disabled";
    }
  }, [activeTrack, subtitles, source]);

  // Reset the selected subtitle whenever the track list changes.
  useEffect(() => {
    setActiveTrack(-1);
  }, [subtitles]);

  useEffect(() => {
    const onFullscreenChange = () => setIsFullscreen(Boolean(document.fullscreenElement));
    document.addEventListener("fullscreenchange", onFullscreenChange);
    return () => document.removeEventListener("fullscreenchange", onFullscreenChange);
  }, []);

  // Load sprite-thumbnail cues for scrub previews.
  useEffect(() => {
    setPreviewCues([]);
    if (!previewVTTURL) {
      return;
    }
    let canceled = false;
    fetch(previewVTTURL, {
      credentials: previewWithCredentials ? "include" : "same-origin",
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
  }, [previewVTTURL, previewWithCredentials]);

  // Reveal the controls and, while playback runs, schedule them to fade out.
  const revealControls = useCallback(() => {
    setControlsVisible(true);
    if (hideControlsTimerRef.current !== null) {
      window.clearTimeout(hideControlsTimerRef.current);
      hideControlsTimerRef.current = null;
    }
    if (playing && !menu && !pointerOverControlsRef.current) {
      hideControlsTimerRef.current = window.setTimeout(() => {
        setControlsVisible(false);
      }, CONTROLS_HIDE_DELAY_MS);
    }
  }, [playing, menu]);

  useEffect(() => {
    revealControls();
    return () => {
      if (hideControlsTimerRef.current !== null) {
        window.clearTimeout(hideControlsTimerRef.current);
        hideControlsTimerRef.current = null;
      }
    };
  }, [revealControls]);

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

  async function togglePlayback() {
    const video = internalVideoRef.current;
    if (!video || !source || transportLocked) {
      return;
    }
    if (video.paused) {
      await video.play().catch(() => undefined);
    } else {
      video.pause();
    }
  }

  function seek(percent: number) {
    const video = internalVideoRef.current;
    if (transportLocked) {
      return;
    }
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
    if (previewCues.length === 0) {
      setPreviewPercent(null);
      return;
    }
    const rect = event.currentTarget.getBoundingClientRect();
    const nextPercent = ((event.clientX - rect.left) / rect.width) * 100;
    setPreviewPercent(Math.max(0, Math.min(nextPercent, 100)));
  }

  const volumePercent = muted ? 0 : volume;
  const previewStyle =
    previewPercent !== null && durationSeconds > 0
      ? spritePreviewStyle(previewCues, previewPercent, durationSeconds)
      : undefined;

  return (
    <section
      className={`player-stage${controlsVisible ? "" : " controls-hidden"}`}
      ref={stageRef}
      aria-label="Video player"
      onPointerMove={revealControls}
    >
      {source ? (
        <video
          ref={setVideoNode}
          className="video-player"
          playsInline
          onClick={() => void togglePlayback()}
          onPlay={() => {
            setPlaying(true);
            onPlay?.();
          }}
          onPause={() => {
            setPlaying(false);
            onPause?.();
          }}
          onEnded={() => {
            setPlaying(false);
            onEnded?.();
          }}
          onSeeked={() => onSeeked?.()}
          onLoadedMetadata={(event) => {
            const video = event.currentTarget;
            setDurationSeconds(Number.isFinite(video.duration) ? video.duration : 0);
            onLoadedMetadata?.(video);
          }}
          onTimeUpdate={(event) => {
            const video = event.currentTarget;
            const duration = Number.isFinite(video.duration) && video.duration > 0 ? video.duration : durationSeconds;
            setCurrentSeconds(video.currentTime || 0);
            setPosition(duration > 0 ? (video.currentTime / duration) * 100 : 0);
            onTimeUpdate?.(video.currentTime || 0, duration);
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
        fallback ?? (
          <div className="player-frame">
            <div className="player-gradient">
              <span className="eyebrow">Playback</span>
              <strong>Stream unavailable</strong>
              <p>This file is not ready for playback yet.</p>
            </div>
          </div>
        )
      )}

      {source ? (
        <div
          className={`player-controls${controlsVisible ? "" : " is-hidden"}`}
          onPointerEnter={() => {
            pointerOverControlsRef.current = true;
            revealControls();
          }}
          onPointerLeave={() => {
            pointerOverControlsRef.current = false;
            revealControls();
          }}
        >
          <span className="scrub-host" onPointerLeave={() => setPreviewPercent(null)}>
            {previewStyle ? <span className="scrub-preview" style={previewStyle} /> : null}
            <input
              className="range"
              type="range"
              min="0"
              max="100"
              step="0.1"
              value={position}
              disabled={transportLocked}
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
                disabled={transportLocked}
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
                  disabled={transportLocked}
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
  );
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
