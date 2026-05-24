import { useCallback, useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";

import * as api from "../lib/api";
import { Badge, Button, DetailSkeleton, EmptyState, Progress } from "../components/ui";
import { formatBytes } from "../lib/format";

export function LibraryDetailPage() {
  const { id = "" } = useParams();
  const itemId = decodeURIComponent(id);
  const [item, setItem] = useState<api.LibraryCatalogItem | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [busyFileId, setBusyFileId] = useState("");

  const loadItem = useCallback(async () => {
    setLoading(true);
    try {
      const items = await api.listLibrary();
      setItem(items.find((candidate) => candidate.id === itemId) || null);
      setError("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to load library title.");
    } finally {
      setLoading(false);
    }
  }, [itemId]);

  useEffect(() => {
    void loadItem();
  }, [loadItem]);

  async function refreshMetadata(fileId: string) {
    setBusyFileId(fileId);
    try {
      await api.refreshFileMetadata(fileId);
      await loadItem();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to refresh metadata.");
    } finally {
      setBusyFileId("");
    }
  }

  if (loading && !item) {
    return <DetailSkeleton />;
  }

  if (error) {
    return (
      <section className="content">
        <div className="alert alert-warn is-visible">{error}</div>
      </section>
    );
  }

  if (!item) {
    return (
      <section className="content">
        <EmptyState title="Title not found">{loading ? "Loading library title." : "This library title is no longer available."}</EmptyState>
      </section>
    );
  }

  return (
    <section className="content">
      <div className="screen-header">
        <div>
          <span className="eyebrow">{item.mediaType}</span>
          <h1>{item.title}</h1>
          <p>{item.overview || metadataCopy(item.metadataStatus)}</p>
          <div className="component-row mt-3">
            <Badge tone={item.metadataStatus === "matched" || item.metadataStatus === "manual" ? "done" : "warn"}>
              {item.metadataStatus}
            </Badge>
            {item.releaseYear ? <span className="mono">{item.releaseYear}</span> : null}
            <span>{item.files.length} {item.files.length === 1 ? "file" : "files"}</span>
          </div>
        </div>
      </div>

      <div className="panel">
        <div className="table-wrap">
          <table className="data-table">
            <thead>
              <tr>
                <th>File</th>
                <th>Episode</th>
                <th>Status</th>
                <th>Progress</th>
                <th>Size</th>
                <th>Codec</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {item.files.map((file) => {
                const playable = Boolean(file.hlsPath) || file.directPlayable;
                return (
                  <tr key={file.id}>
                    <td>
                      <div className="table-title">
                        <strong>{file.name}</strong>
                        <span>{file.torrentId}</span>
                      </div>
                    </td>
                    <td>{episodeLabel(file.episode)}</td>
                    <td>
                      <Badge tone={file.status}>{file.status}</Badge>
                    </td>
                    <td className="table-progress">
                      <Progress value={fileProgress(file)} />
                      <span>{fileProgress(file)}%</span>
                    </td>
                    <td>{file.sizeBytes > 0 ? formatBytes(file.sizeBytes) : "Pending"}</td>
                    <td>{codecLabel(file)}</td>
                    <td>
                      <div className="row-actions">
                        {playable ? (
                          <Link className="btn btn-primary" to={`/player/${file.id}`}>
                            Watch
                          </Link>
                        ) : (
                          <Link className="btn" to={`/torrents/${file.torrentId}`}>
                            Details
                          </Link>
                        )}
                        <Button disabled={busyFileId === file.id} onClick={() => void refreshMetadata(file.id)}>
                          {busyFileId === file.id ? "Queued..." : "Refresh metadata"}
                        </Button>
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </div>
    </section>
  );
}

function metadataCopy(status: api.MetadataStatus): string {
  switch (status) {
    case "matched":
    case "manual":
      return "Metadata is linked to this title.";
    case "unmatched":
      return "Streamize could not confidently match this title yet.";
    case "failed":
      return "Metadata lookup failed. Refresh metadata after provider configuration is fixed.";
    default:
      return "Metadata lookup is pending.";
  }
}

function episodeLabel(episode: api.CatalogEpisode | undefined): string {
  if (!episode) {
    return "—";
  }
  if (episode.seasonNumber && episode.episodeNumber) {
    return `S${String(episode.seasonNumber).padStart(2, "0")}E${String(episode.episodeNumber).padStart(2, "0")} · ${episode.title}`;
  }
  if (episode.absoluteNumber) {
    return `Episode ${episode.absoluteNumber} · ${episode.title}`;
  }
  return episode.title;
}

function fileProgress(file: api.LibraryFile): number {
  if (file.status === "done") {
    return 100;
  }
  return clampPercent(file.transcodingPercent || file.downloadPercent || 0);
}

function codecLabel(file: api.LibraryFile): string {
  const codecs = [file.videoCodec, file.audioCodec].filter(Boolean);
  if (codecs.length > 0) {
    return codecs.map((codec) => String(codec).toUpperCase()).join(" / ");
  }
  return file.ext ? file.ext.replace(".", "").toUpperCase() : "Pending";
}

function clampPercent(value: number): number {
  if (!Number.isFinite(value)) {
    return 0;
  }
  return Math.round(Math.max(0, Math.min(100, value)));
}
