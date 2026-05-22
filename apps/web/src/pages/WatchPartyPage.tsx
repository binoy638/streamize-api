import { type FormEvent, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Link, useParams } from "react-router-dom";

import * as api from "../lib/api";
import { Badge, Button, Field, Input, LoadingScreen, StatCard } from "../components/ui";
import { VideoPlayer } from "../components/VideoPlayer";
import { formatBytes } from "../lib/format";

type PartySocketMessage = {
  type: "snapshot" | "play" | "pause" | "seek" | "sync" | "participant_joined" | "participant_left" | "ended" | "error";
  party?: api.WatchParty;
  participant?: api.WatchPartyParticipant;
  participants?: api.WatchPartyParticipant[];
  error?: string;
};

export function WatchPartyPage() {
  const { slug = "" } = useParams();
  const videoRef = useRef<HTMLVideoElement | null>(null);
  const socketRef = useRef<WebSocket | null>(null);
  const applyingRemoteRef = useRef(false);
  const [party, setParty] = useState<api.WatchParty | null>(null);
  const [file, setFile] = useState<api.WatchPartyFile | null>(null);
  const [session, setSession] = useState<api.WatchPartySession | null>(() => loadStoredSession(slug));
  const [participants, setParticipants] = useState<api.WatchPartyParticipant[]>([]);
  const [subtitles, setSubtitles] = useState<api.Subtitle[]>([]);
  const [displayName, setDisplayName] = useState("");
  const [loading, setLoading] = useState(true);
  const [joining, setJoining] = useState(false);
  const [connected, setConnected] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [remotePlayBlocked, setRemotePlayBlocked] = useState(false);

  const canControl = party?.controlMode === "everyone" || session?.participant.role === "host";
  const playbackSource = useMemo(() => {
    if (!party || !file || !session) {
      return "";
    }
    if (file.hlsReady) {
      return api.watchPartyHLSPlaylistURL(party.slug, file.id, session);
    }
    if (file.directPlayable) {
      return api.watchPartyOriginalURL(party.slug, file.id, session);
    }
    return "";
  }, [file, party, session]);

  const loadParty = useCallback(async () => {
    if (!slug) {
      setError("Watch party link is missing.");
      setLoading(false);
      return;
    }
    try {
      const response = await api.getWatchParty(slug);
      setParty(response.party);
      setFile(response.file);
      setError("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to load watch party.");
    } finally {
      setLoading(false);
    }
  }, [slug]);

  useEffect(() => {
    void loadParty();
  }, [loadParty]);

  useEffect(() => {
    if (!party || !file || !session) {
      return;
    }
    let canceled = false;
    api
      .listWatchPartySubtitles(party.slug, file.id, session)
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
  }, [file, party, session]);

  useEffect(() => {
    if (!party || !session) {
      return;
    }

    let canceled = false;
    let attempt = 0;
    const urls = api.watchPartyWebSocketURLs(party.slug, session);

    function connect() {
      const url = urls[attempt];
      if (!url || canceled) {
        setNotice("Watch party connection failed. Check that the API server is running and reachable from this browser.");
        return;
      }

      const socket = new WebSocket(url);
      socketRef.current = socket;
      let opened = false;
      socket.onopen = () => {
        opened = true;
        setConnected(true);
        setNotice("");
      };
      socket.onclose = () => {
        if (canceled) {
          return;
        }
        setConnected(false);
        if (!opened) {
          attempt += 1;
          connect();
        }
      };
      socket.onerror = () => {
        socket.close();
      };
      socket.onmessage = (event) => {
        const message = JSON.parse(event.data) as PartySocketMessage;
        handleSocketMessage(message);
      };
    }

    connect();

    return () => {
      canceled = true;
      socketRef.current?.close();
      socketRef.current = null;
      setConnected(false);
    };
  }, [party?.slug, session?.participant.id, session?.token]);

  useEffect(() => {
    if (!canControl) {
      return;
    }
    const intervalID = window.setInterval(() => {
      const video = videoRef.current;
      if (!video || video.paused) {
        return;
      }
      sendPlayback("sync");
    }, 5000);
    return () => window.clearInterval(intervalID);
  }, [canControl, party?.id]);

  async function joinParty(event: FormEvent) {
    event.preventDefault();
    setJoining(true);
    setError("");
    try {
      const response = await api.joinWatchParty(slug, displayName.trim());
      if (!response.session) {
        throw new Error("Join response did not include a participant session.");
      }
      sessionStorage.setItem(watchPartySessionStorageKey(slug), JSON.stringify(response.session));
      setParty(response.party);
      setFile(response.file);
      setSession(response.session);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to join watch party.");
    } finally {
      setJoining(false);
    }
  }

  function handleSocketMessage(message: PartySocketMessage) {
    if (message.error) {
      setNotice(message.error);
    }
    if (message.participants) {
      setParticipants(message.participants);
    }
    if (message.party?.slug) {
      setParty(message.party);
    }
    if (message.type === "ended") {
      setNotice("This watch party has ended.");
      return;
    }
    if (!message.party?.slug || message.type === "snapshot" || message.participant?.id === session?.participant.id) {
      if (message.type === "snapshot" && message.party) {
        applyRemotePlayback(message.party, true);
      }
      return;
    }
    if (["play", "pause", "seek", "sync"].includes(message.type)) {
      applyRemotePlayback(message.party, message.type !== "sync");
    }
  }

  function applyRemotePlayback(nextParty: api.WatchParty, forcePlayState: boolean) {
    const video = videoRef.current;
    if (!video) {
      return;
    }
    applyingRemoteRef.current = true;
    const drift = Math.abs(video.currentTime - nextParty.currentPositionSeconds);
    if (drift > 1.5) {
      video.currentTime = nextParty.currentPositionSeconds;
    }
    if (forcePlayState) {
      if (nextParty.isPlaying) {
        video.play().then(() => setRemotePlayBlocked(false)).catch(() => setRemotePlayBlocked(true));
      } else {
        video.pause();
        setRemotePlayBlocked(false);
      }
    }
    window.setTimeout(() => {
      applyingRemoteRef.current = false;
    }, 150);
  }

  function sendPlayback(type: "play" | "pause" | "seek" | "sync") {
    if (!canControl || applyingRemoteRef.current || !party) {
      return;
    }
    const socket = socketRef.current;
    const video = videoRef.current;
    if (!socket || socket.readyState !== WebSocket.OPEN || !video) {
      return;
    }
    socket.send(JSON.stringify({
      type,
      positionSeconds: Number.isFinite(video.currentTime) ? video.currentTime : 0,
      durationSeconds: Number.isFinite(video.duration) ? video.duration : party.durationSeconds,
      isPlaying: !video.paused,
    }));
  }

  async function copyLink() {
    await navigator.clipboard?.writeText(window.location.href);
    setNotice("Party link copied.");
  }

  async function endParty() {
    if (!party || session?.participant.role !== "host") {
      return;
    }
    try {
      const ended = await api.endWatchParty(party.id);
      setParty(ended);
      setNotice("Watch party ended.");
    } catch (err) {
      setNotice(err instanceof Error ? err.message : "Unable to end watch party.");
    }
  }

  if (loading) {
    return <LoadingScreen label="Loading watch party" />;
  }

  if (error && !party) {
    return (
      <main className="auth-page">
        <section className="auth-card">
          <div className="panel-title">Watch party unavailable</div>
          <p className="muted">{error}</p>
          <Link className="btn btn-primary" to="/sign-in">Sign in</Link>
        </section>
      </main>
    );
  }

  if (!session) {
    return (
      <main className="auth-page watch-join-page">
        <section className="auth-card">
          <div>
            <span className="eyebrow">Watch party</span>
            <h1>{file?.name || "Join room"}</h1>
          </div>
          {error ? <div className="alert alert-error is-visible">{error}</div> : null}
          <form className="form-grid" onSubmit={joinParty}>
            <Field label="Display name">
              <Input
                value={displayName}
                onChange={(event) => setDisplayName(event.target.value)}
                placeholder="How others will see you"
                required
              />
            </Field>
            <Button variant="primary" type="submit" disabled={joining}>
              {joining ? "Joining..." : "Join watch party"}
            </Button>
          </form>
        </section>
      </main>
    );
  }

  return (
    <section className="content player-content watch-party-page">
      <div className="screen-header">
        <div>
          <span className="eyebrow">Watch party</span>
          <h1>{file?.name || "Shared playback"}</h1>
          <p>{party?.controlMode === "everyone" ? "Everyone can control playback." : "Only the host can control playback."}</p>
        </div>
        <div className="header-actions">
          <Badge tone={connected ? "online" : "offline"}>{connected ? "Connected" : "Offline"}</Badge>
          <Button onClick={() => void copyLink()}>Copy link</Button>
          {session.participant.role === "host" ? <Button variant="danger" onClick={() => void endParty()}>End</Button> : null}
        </div>
      </div>

      {notice ? <div className="alert alert-warn is-visible">{notice}</div> : null}
      {remotePlayBlocked ? (
        <div className="alert alert-warn is-visible">
          Playback is live. <Button variant="primary" onClick={() => videoRef.current?.play()}>Resume</Button>
        </div>
      ) : null}

      <div className="player-layout">
        <VideoPlayer
          source={playbackSource}
          isDirect={Boolean(file && !file.hlsReady && file.directPlayable)}
          withCredentials
          subtitles={subtitles}
          previewVTTURL={
            file?.progressPreview && party && session
              ? api.watchPartyPreviewVTTURL(party.slug, file.id, session)
              : undefined
          }
          previewWithCredentials
          transportLocked={!canControl}
          videoRef={videoRef}
          onPlay={() => sendPlayback("play")}
          onPause={() => sendPlayback("pause")}
          onSeeked={() => sendPlayback("seek")}
          onError={(message) => setNotice(message)}
          fallback={
            <div className="player-frame">
              <div className="player-gradient">
                <span className="eyebrow">Playback</span>
                <strong>Stream unavailable</strong>
                <p>This watch party file is not ready for public playback.</p>
              </div>
            </div>
          }
        />

        <aside className="panel player-side">
          <div className="panel-header">
            <div>
              <div className="panel-title">Participants</div>
              <p className="muted">{participants.length} connected</p>
            </div>
          </div>
          <div className="file-list">
            {participants.map((participant) => (
              <div className="file-row" key={participant.id}>
                <span className="file-row-copy">
                  <strong>{participant.displayName}</strong>
                  <small>{participant.role}</small>
                </span>
                <Badge tone={participant.connected ? "online" : "offline"}>{participant.connected ? "Here" : "Away"}</Badge>
              </div>
            ))}
          </div>
        </aside>
      </div>

      <div className="stats-row">
        <StatCard label="File" value={file?.hlsReady ? "HLS" : file?.directPlayable ? "Direct" : "Pending"} detail="party source" />
        <StatCard label="Size" value={file ? formatBytes(file.sizeBytes) : "0 B"} detail="selected file" />
        <StatCard label="Controls" value={canControl ? "Enabled" : "Host"} detail="playback control" />
        <StatCard label="Subtitles" value={String(subtitles.length)} detail="available tracks" />
      </div>
    </section>
  );
}

function watchPartySessionStorageKey(slug: string): string {
  return `watchPartySession:${slug}`;
}

function loadStoredSession(slug: string): api.WatchPartySession | null {
  if (!slug) {
    return null;
  }
  try {
    const raw = sessionStorage.getItem(watchPartySessionStorageKey(slug));
    return raw ? (JSON.parse(raw) as api.WatchPartySession) : null;
  } catch {
    return null;
  }
}
