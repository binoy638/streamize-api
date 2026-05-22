import { useCallback, useEffect, useMemo, useState } from "react";
import { Link, useParams } from "react-router-dom";

import * as api from "../lib/api";
import { Badge, Button, LoadingScreen } from "../components/ui";
import { VideoPlayer } from "../components/VideoPlayer";
import { formatBytes } from "../lib/format";

export function SharePage() {
  const { slug = "" } = useParams();
  const [item, setItem] = useState<api.SharedItem | null>(null);
  const [selectedFileId, setSelectedFileId] = useState("");
  const [subtitles, setSubtitles] = useState<api.Subtitle[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [expired, setExpired] = useState(false);

  const selectedFile = useMemo(
    () => item?.files.find((file) => file.id === selectedFileId) || item?.files[0],
    [item, selectedFileId],
  );

  const playbackSource = useMemo(() => {
    if (!selectedFile) {
      return "";
    }
    if (selectedFile.hlsReady) {
      return api.shareHLSPlaylistURL(slug, selectedFile.id);
    }
    if (selectedFile.directPlayable) {
      return api.shareOriginalURL(slug, selectedFile.id);
    }
    return "";
  }, [selectedFile, slug]);

  const isDirect = Boolean(selectedFile && !selectedFile.hlsReady && selectedFile.directPlayable);

  useEffect(() => {
    let canceled = false;
    setLoading(true);
    api
      .getSharedItem(slug)
      .then((shared) => {
        if (canceled) {
          return;
        }
        setItem(shared);
        setSelectedFileId(shared.files[0]?.id || "");
        setError("");
        setExpired(false);
      })
      .catch((err) => {
        if (canceled) {
          return;
        }
        if (err instanceof api.ApiError && err.status === 410) {
          setExpired(true);
        }
        setError(err instanceof Error ? err.message : "Unable to load this share.");
        setItem(null);
      })
      .finally(() => {
        if (!canceled) {
          setLoading(false);
        }
      });

    return () => {
      canceled = true;
    };
  }, [slug]);

  // Load subtitle tracks for the selected file.
  useEffect(() => {
    setSubtitles([]);
    if (!selectedFile) {
      return;
    }
    let canceled = false;
    api
      .listShareSubtitles(slug, selectedFile.id)
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
  }, [selectedFile, slug]);

  const copyLink = useCallback(() => {
    void navigator.clipboard?.writeText(window.location.href);
  }, []);

  if (loading) {
    return <LoadingScreen label="Loading shared video" />;
  }

  if (!item) {
    return (
      <main className="auth-page">
        <section className="auth-card">
          <span className="eyebrow">Streamize</span>
          <div className="panel-title">{expired ? "This share link has expired" : "Share link unavailable"}</div>
          <p className="muted">{error || "The link may have been revoked or never existed."}</p>
          <Link className="btn btn-primary" to="/library">
            Go to Streamize
          </Link>
        </section>
      </main>
    );
  }

  return (
    <section className="content player-content">
      <div className="screen-header">
        <div>
          <span className="eyebrow">Shared {item.scope === "torrent" ? "collection" : "video"}</span>
          <h1>{item.title}</h1>
          <p>Expires {formatExpiry(item.expiresAt)}</p>
        </div>
        <div className="header-actions">
          <Badge tone="online">Public link</Badge>
          <Button onClick={copyLink}>Copy link</Button>
        </div>
      </div>

      {error ? <div className="alert alert-warn is-visible">{error}</div> : null}

      <div className="player-layout">
        <VideoPlayer
          source={playbackSource}
          isDirect={isDirect}
          subtitles={subtitles}
          previewVTTURL={
            selectedFile?.progressPreview ? api.sharePreviewVTTURL(slug, selectedFile.id) : undefined
          }
          fallback={
            <div className="player-frame">
              <div className="player-gradient">
                <span className="eyebrow">Playback</span>
                <strong>{selectedFile?.name || item.title}</strong>
                <p>This file is not ready for playback yet.</p>
              </div>
            </div>
          }
          onError={(message) => setError(message)}
        />

        {item.files.length > 1 ? (
          <aside className="panel player-side">
            <div className="panel-header">
              <div>
                <div className="panel-title">Files</div>
                <p className="muted">
                  {item.files.length} file{item.files.length === 1 ? "" : "s"} shared
                </p>
              </div>
            </div>
            <div className="file-list">
              {item.files.map((file) => (
                <button
                  className={`file-row ${file.id === selectedFile?.id ? "active" : ""}`}
                  key={file.id}
                  onClick={() => setSelectedFileId(file.id)}
                >
                  <span className="file-row-copy">
                    <strong>{file.name}</strong>
                    <small>
                      {formatBytes(file.sizeBytes)}
                      {file.videoCodec ? ` · ${file.videoCodec.toUpperCase()}` : ""}
                    </small>
                  </span>
                  <Badge tone={file.hlsReady || file.directPlayable ? "ready" : "processing"}>
                    {file.hlsReady ? "HLS" : file.directPlayable ? "Direct" : "Pending"}
                  </Badge>
                </button>
              ))}
            </div>
          </aside>
        ) : null}
      </div>

      <p className="muted share-footnote">
        Shared via Streamize. Anyone with this link can watch until it expires.
      </p>
    </section>
  );
}

function formatExpiry(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "soon";
  }
  return date.toLocaleString(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  });
}
