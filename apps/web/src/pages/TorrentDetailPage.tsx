import { type CSSProperties, useCallback, useEffect, useMemo, useState } from "react";
import { Trash2 } from "lucide-react";
import { Link, useNavigate, useParams } from "react-router-dom";

import * as api from "../lib/api";
import { DeleteTorrentModal } from "../components/DeleteTorrentModal";
import { Badge, Button, DetailSkeleton, EmptyState, Progress, StatCard } from "../components/ui";
import { formatBytes, formatDate } from "../lib/format";

const tabs = ["files", "jobs", "subtitles", "shares", "overview"];
const pollableStatuses = new Set(["added", "queued", "downloading", "processing"]);
const pollableFileStatuses = new Set(["downloading", "queued", "processing"]);
const pollableJobStatuses = new Set(["queued", "running"]);

type TorrentDetail = {
  id: string;
  name: string;
  status: api.TorrentStatus;
  size: string;
  progress: number;
  peers: string;
  addedAt: string;
  eta: string;
  infoHash: string;
  qbittorrentHash: string;
  retention: string;
  posterHue: number;
};

type FileRow = {
  id: string;
  name: string;
  size: string;
  status: api.TorrentFileStatus;
  progress: number;
  codec: string;
  subtitles: string;
  canOpen: boolean;
};

type JobRow = {
  id: string;
  type: string;
  target: string;
  status: api.JobStatus;
  progress: number;
  worker: string;
  startedAt: string;
  finishedAt: string;
  updatedAt: string;
};

type SubtitleRow = {
  id: string;
  fileName: string;
  title: string;
  language: string;
};

export function TorrentDetailPage() {
  const { id } = useParams();
  const navigate = useNavigate();
  const [torrent, setTorrent] = useState<TorrentDetail | null>(null);
  const [files, setFiles] = useState<FileRow[]>([]);
  const [detailJobs, setDetailJobs] = useState<JobRow[]>([]);
  const [subtitles, setSubtitles] = useState<SubtitleRow[]>([]);
  const [activeTab, setActiveTab] = useState("files");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [deleteBusy, setDeleteBusy] = useState(false);
  const [deleteError, setDeleteError] = useState("");

  const loadDetail = useCallback(
    async ({ showLoading = true }: { showLoading?: boolean } = {}) => {
      if (showLoading) {
        setLoading(true);
      }

      try {
        const apiTorrents = await api.listTorrents();
        const apiTorrent = apiTorrents.find((item) => item.id === id);
        if (!apiTorrent) {
          throw new Error("Torrent was not found in the API.");
        }

        const [apiFiles, apiJobs] = await Promise.all([
          api.listTorrentFiles(apiTorrent.id),
          api.listJobs(),
        ]);
        const subtitleResults = await Promise.allSettled(
          apiFiles.map(async (file) => {
            const records = await api.listSubtitles(file.id);
            return records.map((subtitle): SubtitleRow => ({
              id: subtitle.id,
              fileName: file.name,
              title: subtitle.title || subtitle.fileName,
              language: subtitle.language,
            }));
          }),
        );
        setTorrent(apiTorrentToDetail(apiTorrent));
        setFiles(apiFiles.map(apiFileToRow));
        setDetailJobs(apiJobs.filter((job) => job.torrentId === apiTorrent.id).map(apiJobToDetailRow));
        setSubtitles(subtitleResults.flatMap((result) => (result.status === "fulfilled" ? result.value : [])));
        setError("");
      } catch (err) {
        if (showLoading) {
          setTorrent(null);
          setFiles([]);
          setDetailJobs([]);
          setSubtitles([]);
        }
        setError(err instanceof Error ? err.message : "Unable to load torrent detail.");
      } finally {
        if (showLoading) {
          setLoading(false);
        }
      }
    },
    [id],
  );

  useEffect(() => {
    void loadDetail();
  }, [loadDetail]);

  useEffect(() => {
    const hasActiveFiles = files.some((file) => pollableFileStatuses.has(file.status));
    const hasActiveJobs = detailJobs.some((job) => pollableJobStatuses.has(job.status));
    if (!torrent || (!pollableStatuses.has(torrent.status) && !hasActiveFiles && !hasActiveJobs)) {
      return;
    }

    const intervalID = window.setInterval(() => {
      void loadDetail({ showLoading: false });
    }, 5000);

    return () => window.clearInterval(intervalID);
  }, [detailJobs, files, loadDetail, torrent]);

  const jobStats = useMemo(() => {
    const failed = detailJobs.filter((job) => job.status === "failed").length;
    const running = detailJobs.filter((job) => job.status === "running").length;
    if (detailJobs.length === 0) {
      return "none queued";
    }
    return `${failed} failed, ${running} running`;
  }, [detailJobs]);

  async function deleteCurrentTorrent(options: Required<api.DeleteTorrentOptions>) {
    if (!torrent) {
      return;
    }
    setDeleteBusy(true);
    setDeleteError("");
    try {
      await api.deleteTorrent(torrent.id, options);
      navigate("/torrents");
    } catch (err) {
      setDeleteOpen(false);
      setDeleteError(err instanceof Error ? err.message : "Unable to delete torrent.");
    } finally {
      setDeleteBusy(false);
    }
  }

  if (loading && !torrent) {
    return <DetailSkeleton />;
  }

  if (!torrent) {
    return (
      <section className="content">
        {error ? <div className="alert alert-error is-visible">{error}</div> : null}
        <EmptyState title="Torrent not found">The torrent detail could not be loaded from the API.</EmptyState>
      </section>
    );
  }

  return (
    <section className="content">
      {error ? <div className="alert alert-error is-visible">Unable to refresh torrent detail: {error}</div> : null}
      {deleteError ? <div className="alert alert-error is-visible">{deleteError}</div> : null}

      <div className="detail-hero">
        <div className="poster detail-poster" style={{ "--poster-hue": torrent.posterHue } as CSSProperties}>
          <span className="poster-label">{torrent.name.slice(0, 18)}</span>
        </div>
        <div className="detail-copy">
          <div className="detail-kicker">
            <span className="eyebrow">Torrent detail</span>
            <Button
              variant="danger"
              disabled={loading || deleteBusy}
              onClick={() => setDeleteOpen(true)}
            >
              <Trash2 size={15} />
              {deleteBusy ? "Deleting..." : "Delete"}
            </Button>
          </div>
          <h1>{torrent.name}</h1>
          <p>
            Files, processing jobs, subtitles, and shares stay attached to the torrent record so
            operations are visible without hopping across screens.
          </p>
          <div className="component-row">
            <Badge tone={torrent.status}>{torrent.status}</Badge>
            <span className="mono">{torrent.size}</span>
            <span>{torrent.peers} peers</span>
          </div>
          <div className="progress-row wide">
            <Progress value={torrent.progress} />
            <span>{torrent.progress}%</span>
          </div>
        </div>
      </div>

      <div className="stats-row">
        <StatCard label="Files" value={String(files.length)} detail={loading ? "loading candidates" : "video candidates"} />
        <StatCard label="Jobs" value={String(detailJobs.length)} detail={jobStats} />
        <StatCard label="Subtitles" value={String(subtitles.length)} detail="detected or extracted" />
        <StatCard label="Retention" value={torrent.retention} detail="originals retained" />
      </div>

      <div className="tabs" role="tablist">
        {tabs.map((tab) => (
          <button
            key={tab}
            className={activeTab === tab ? "active" : ""}
            onClick={() => setActiveTab(tab)}
            role="tab"
            aria-selected={activeTab === tab}
          >
            {tab}
          </button>
        ))}
      </div>

      <div className="panel">
        {activeTab === "overview" ? (
          <div className="overview-grid">
            <Info title="Info hash" value={shortHash(torrent.infoHash)} />
            <Info title="qBittorrent hash" value={shortHash(torrent.qbittorrentHash)} />
            <Info title="Added" value={torrent.addedAt} />
            <Info title="ETA" value={torrent.eta} />
          </div>
        ) : null}

        {activeTab === "files" ? (
          <div className="table-wrap">
            <table className="data-table">
              <thead>
                <tr>
                  <th>File</th>
                  <th>Status</th>
                  <th>Transcode</th>
                  <th>Codec</th>
                  <th>Subtitles</th>
                  <th>Action</th>
                </tr>
              </thead>
              <tbody>
                {files.map((file) => (
                  <tr key={file.id}>
                    <td>
                      <div className="table-title">
                        <strong>{file.name}</strong>
                        <span>{file.size}</span>
                      </div>
                    </td>
                    <td>
                      <Badge tone={file.status}>{file.status}</Badge>
                    </td>
                    <td className="table-progress">
                      <Progress value={file.progress} />
                      <span>{file.progress}%</span>
                    </td>
                    <td>{file.codec}</td>
                    <td>{file.subtitles}</td>
                    <td>
                      {file.canOpen ? (
                        <Link className="btn btn-ghost" to={`/player/${file.id}`}>
                          Open
                        </Link>
                      ) : (
                        <Button disabled>Pending</Button>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
            {files.length === 0 ? <EmptyState title="No files">Metadata sync has not found files yet.</EmptyState> : null}
          </div>
        ) : null}

        {activeTab === "jobs" ? (
          <div className="timeline-list">
            {detailJobs.map((job) => (
              <div className="timeline-item" key={job.id}>
                <Badge tone={job.status}>{job.status}</Badge>
                <div>
                  <strong>{job.type}</strong>
                  <p className="muted">
                    {job.target} · {job.worker} · {job.updatedAt}
                  </p>
                  <p className="muted">
                    Started {job.startedAt} · Finished {job.finishedAt}
                  </p>
                </div>
                <Progress value={job.progress} />
              </div>
            ))}
            {detailJobs.length === 0 ? <EmptyState title="No jobs">No processing jobs are attached to this torrent yet.</EmptyState> : null}
          </div>
        ) : null}

        {activeTab === "subtitles" ? (
          <div className="timeline-list">
            {subtitles.map((subtitle) => (
              <div className="timeline-item" key={subtitle.id}>
                <Badge tone="done">{subtitle.language || "track"}</Badge>
                <div>
                  <strong>{subtitle.title}</strong>
                  <p className="muted">{subtitle.fileName}</p>
                </div>
              </div>
            ))}
            {subtitles.length === 0 ? <EmptyState title="No subtitles">No subtitle tracks are attached to these files yet.</EmptyState> : null}
          </div>
        ) : null}

        {activeTab === "shares" ? (
          <EmptyState title="Shares are managed separately">Open the Shares page to create, copy, or revoke public links.</EmptyState>
        ) : null}
      </div>

      <DeleteTorrentModal
        open={deleteOpen}
        title={torrent.name || "this torrent"}
        busy={deleteBusy}
        onClose={() => {
          if (!deleteBusy) {
            setDeleteOpen(false);
          }
        }}
        onConfirm={deleteCurrentTorrent}
      />
    </section>
  );
}

function apiTorrentToDetail(torrent: api.Torrent): TorrentDetail {
  const name = torrent.name || torrent.infoHash || torrent.id;

  return {
    id: torrent.id,
    name,
    status: torrent.status,
    size: formatOptionalBytes(torrent.sizeBytes),
    progress: progressForTorrent(torrent),
    peers: String(torrent.peers ?? 0),
    addedAt: formatDate(torrent.createdAt),
    eta: etaForTorrent(torrent),
    infoHash: torrent.infoHash || "",
    qbittorrentHash: torrent.qbittorrentHash || "",
    retention: torrent.retentionPolicy === "delete_after_hls" ? "Delete" : "Keep",
    posterHue: hueFromString(name),
  };
}

function apiFileToRow(file: api.TorrentFile): FileRow {
  return {
    id: file.id,
    name: file.name,
    size: formatOptionalBytes(file.sizeBytes),
    status: file.status,
    progress: progressForFile(file),
    codec: codecLabel(file),
    subtitles: file.progressPreview ? "Preview ready" : "Pending",
    canOpen: ((file.status === "done" || file.status === "processing") && Boolean(file.hlsPath)) || file.directPlayable,
  };
}

function apiJobToDetailRow(job: api.Job): JobRow {
  return {
    id: job.id,
    type: jobTypeLabel(job.type),
    target: job.target || job.torrentFileId || "Unattached job",
    status: job.status,
    progress: progressForJob(job),
    worker: job.lockedBy || (job.status === "running" ? "claimed" : "unclaimed"),
    startedAt: formatJobTime(job.startedAt, "not started"),
    finishedAt: formatJobTime(job.finishedAt, job.status === "running" ? "running" : "not finished"),
    updatedAt: formatDate(job.updatedAt),
  };
}

function progressForTorrent(torrent: api.Torrent): number {
  if (Number.isFinite(torrent.progressPercent)) {
    return Math.round(Math.max(0, Math.min(torrent.progressPercent, 100)));
  }
  if (torrent.status === "done") {
    return 100;
  }

  return 0;
}

function progressForFile(file: api.TorrentFile): number {
  if (file.status === "done") {
    return 100;
  }
  if (file.status === "downloading" && Number.isFinite(file.downloadPercent)) {
    return Math.round(Math.max(0, Math.min(file.downloadPercent, 100)));
  }
  if (Number.isFinite(file.transcodingPercent)) {
    return Math.round(Math.max(0, Math.min(file.transcodingPercent, 100)));
  }

  return 0;
}

function progressForJob(job: api.Job): number {
  if (job.status === "succeeded") {
    return 100;
  }
  if (job.status === "queued" || job.status === "canceled") {
    return 0;
  }
  if (Number.isFinite(job.progressPercent)) {
    return Math.round(Math.max(0, Math.min(job.progressPercent, 100)));
  }

  return 0;
}

function etaForTorrent(torrent: api.Torrent) {
  if (torrent.status === "done") {
    return "Complete";
  }
  if (torrent.status === "error") {
    return "Needs retry";
  }
  if (torrent.status === "paused") {
    return "Paused";
  }
  if (Number.isFinite(torrent.etaSeconds) && torrent.etaSeconds >= 0) {
    return formatDuration(torrent.etaSeconds);
  }
  if (torrent.status === "downloading") {
    return "Downloading";
  }
  if (torrent.status === "processing") {
    return "Processing";
  }

  return "Queued";
}

function formatDuration(totalSeconds: number) {
  const seconds = Math.max(0, Math.round(totalSeconds));
  if (seconds < 60) {
    return `${seconds}s`;
  }

  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) {
    return `${minutes}m`;
  }

  const hours = Math.floor(minutes / 60);
  const remainingMinutes = minutes % 60;
  return remainingMinutes > 0 ? `${hours}h ${remainingMinutes}m` : `${hours}h`;
}

function formatOptionalBytes(bytes: number): string {
  return bytes > 0 ? formatBytes(bytes) : "Pending";
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

function jobTypeLabel(type: string): string {
  switch (type) {
    case "hls_transcode":
      return "HLS transcode";
    case "subtitle_extract":
      return "Subtitle extraction";
    case "sprite_generate":
      return "Sprite generation";
    default:
      return type.replace(/[_-]/g, " ");
  }
}

function formatJobTime(value: string | undefined, fallback: string): string {
  if (!value) {
    return fallback;
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function shortHash(value: string): string {
  if (!value) {
    return "Pending";
  }
  if (value.length <= 14) {
    return value;
  }

  return `${value.slice(0, 6)}...${value.slice(-6)}`;
}

function hueFromString(value: string): number {
  let hash = 0;
  for (let index = 0; index < value.length; index += 1) {
    hash = (hash + value.charCodeAt(index) * (index + 1)) % 360;
  }

  return hash;
}

function Info({ title, value }: { title: string; value: string }) {
  return (
    <div className="info-tile">
      <span>{title}</span>
      <strong>{value}</strong>
    </div>
  );
}
