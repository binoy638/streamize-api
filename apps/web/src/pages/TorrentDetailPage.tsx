import { type CSSProperties, useCallback, useEffect, useMemo, useState } from "react";
import { Link, useParams } from "react-router-dom";

import * as api from "../lib/api";
import { Badge, Button, EmptyState, Progress, StatCard } from "../components/ui";
import { formatBytes, formatDate } from "../lib/format";
import {
  filesForTorrent,
  findTorrent,
  jobs as mockJobs,
  shares,
  type Job as MockJob,
  type Torrent as MockTorrent,
  type TorrentFile as MockTorrentFile,
} from "../lib/mock-data";

const tabs = ["overview", "files", "jobs", "subtitles", "shares"];
const pollableStatuses = new Set(["added", "queued", "downloading", "processing"]);
const pollableFileStatuses = new Set(["downloading", "queued", "processing"]);
const pollableJobStatuses = new Set(["queued", "running"]);

type TorrentDetail = {
  id: string;
  name: string;
  status: api.TorrentStatus | MockTorrent["status"];
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
  status: api.TorrentFileStatus | MockTorrentFile["status"];
  progress: number;
  codec: string;
  subtitles: string;
  canOpen: boolean;
};

type JobRow = {
  id: string;
  type: string;
  target: string;
  status: api.JobStatus | MockJob["status"];
  progress: number;
  worker: string;
  updatedAt: string;
};

export function TorrentDetailPage() {
  const { id } = useParams();
  const mockTorrent = useMemo(() => findTorrent(id), [id]);
  const mockFiles = useMemo(() => filesForTorrent(mockTorrent.id).map(mockFileToRow), [mockTorrent.id]);
  const mockDetailJobs = useMemo(() => mockJobs.slice(0, 4).map(mockJobToDetailRow), []);
  const [torrent, setTorrent] = useState<TorrentDetail>(() => mockTorrentToDetail(mockTorrent));
  const [files, setFiles] = useState<FileRow[]>(() => mockFiles);
  const [detailJobs, setDetailJobs] = useState<JobRow[]>(() => mockDetailJobs);
  const [activeTab, setActiveTab] = useState("overview");
  const [loading, setLoading] = useState(true);
  const [usingMock, setUsingMock] = useState(true);
  const [error, setError] = useState("");

  const loadDetail = useCallback(
    async ({ showLoading = true, fallbackToMock = true }: { showLoading?: boolean; fallbackToMock?: boolean } = {}) => {
      if (showLoading) {
        setLoading(true);
      }

      try {
        const apiTorrents = await api.listTorrents();
        const apiTorrent = apiTorrents.find((item) => item.id === id);
        if (!apiTorrent) {
          throw new Error("Torrent was not found in the API.");
        }

        const apiFiles = await api.listTorrentFiles(apiTorrent.id);
        const apiJobs = await api.listJobs();
        setTorrent(apiTorrentToDetail(apiTorrent));
        setFiles(apiFiles.map(apiFileToRow));
        setDetailJobs(apiJobs.filter((job) => job.torrentId === apiTorrent.id).map(apiJobToDetailRow));
        setUsingMock(false);
        setError("");
      } catch (err) {
        if (fallbackToMock) {
          setTorrent(mockTorrentToDetail(mockTorrent));
          setFiles(mockFiles);
          setDetailJobs(mockDetailJobs);
          setUsingMock(true);
          setError(err instanceof Error ? err.message : "Unable to load torrent detail.");
        }
      } finally {
        if (showLoading) {
          setLoading(false);
        }
      }
    },
    [id, mockDetailJobs, mockFiles, mockTorrent],
  );

  useEffect(() => {
    void loadDetail();
  }, [loadDetail]);

  useEffect(() => {
    const hasActiveFiles = files.some((file) => pollableFileStatuses.has(file.status));
    const hasActiveJobs = detailJobs.some((job) => pollableJobStatuses.has(job.status));
    if (usingMock || (!pollableStatuses.has(torrent.status) && !hasActiveFiles && !hasActiveJobs)) {
      return;
    }

    const intervalID = window.setInterval(() => {
      void loadDetail({ showLoading: false, fallbackToMock: false });
    }, 5000);

    return () => window.clearInterval(intervalID);
  }, [detailJobs, files, loadDetail, torrent.status, usingMock]);

  const jobStats = useMemo(() => {
    const failed = detailJobs.filter((job) => job.status === "failed").length;
    const running = detailJobs.filter((job) => job.status === "running").length;
    if (detailJobs.length === 0) {
      return "none queued";
    }
    return `${failed} failed, ${running} running`;
  }, [detailJobs]);

  return (
    <section className="content">
      {usingMock && error ? (
        <div className="alert alert-warn is-visible">Using prototype torrent detail because the API detail could not load: {error}</div>
      ) : null}

      <div className="detail-hero">
        <div className="poster detail-poster" style={{ "--poster-hue": torrent.posterHue } as CSSProperties}>
          <span className="poster-label">{torrent.name.slice(0, 18)}</span>
        </div>
        <div className="detail-copy">
          <span className="eyebrow">Torrent detail</span>
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
        <StatCard label="Subtitles" value="6" detail="detected or extracted" />
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
                </div>
                <Progress value={job.progress} />
              </div>
            ))}
            {detailJobs.length === 0 ? <EmptyState title="No jobs">No processing jobs are attached to this torrent yet.</EmptyState> : null}
          </div>
        ) : null}

        {activeTab === "subtitles" ? (
          <div className="overview-grid">
            <Info title="English" value="Embedded SRT extracted" />
            <Info title="Spanish" value="Uploaded VTT" />
            <Info title="Japanese" value="Queued for extraction" />
            <Info title="Default" value="English forced off" />
          </div>
        ) : null}

        {activeTab === "shares" ? (
          <div className="timeline-list">
            {shares.slice(0, 2).map((share) => (
              <div className="timeline-item" key={share.id}>
                <Badge tone={share.status}>{share.status}</Badge>
                <div>
                  <strong>{share.title}</strong>
                  <p className="muted">
                    {share.scope} · expires {share.expiresAt}
                  </p>
                </div>
                <Button>Copy</Button>
              </div>
            ))}
          </div>
        ) : null}
      </div>
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

function mockTorrentToDetail(torrent: MockTorrent): TorrentDetail {
  return {
    id: torrent.id,
    name: torrent.name,
    status: torrent.status,
    size: torrent.size,
    progress: torrent.progress,
    peers: String(torrent.peers),
    addedAt: torrent.addedAt,
    eta: torrent.eta,
    infoHash: "a91d72cf",
    qbittorrentHash: "QBT-7A91D2",
    retention: "Keep",
    posterHue: hueFromString(torrent.id),
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

function mockFileToRow(file: MockTorrentFile): FileRow {
  return {
    id: file.id,
    name: file.name,
    size: file.size,
    status: file.status,
    progress: file.progress,
    codec: file.codec,
    subtitles: `${file.subtitles} tracks`,
    canOpen: true,
  };
}

function apiJobToDetailRow(job: api.Job): JobRow {
  return {
    id: job.id,
    type: job.type,
    target: job.target || job.torrentFileId || "Unattached job",
    status: job.status,
    progress: progressForJob(job),
    worker: job.lockedBy || (job.status === "running" ? "claimed" : "unclaimed"),
    updatedAt: formatDate(job.updatedAt),
  };
}

function mockJobToDetailRow(job: MockJob): JobRow {
  return {
    id: job.id,
    type: job.type,
    target: job.target,
    status: job.status,
    progress: job.progress,
    worker: job.worker,
    updatedAt: job.updatedAt,
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
